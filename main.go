package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/golang-db/memtable"
	"github.com/golang-db/wal"
)

const (
	ssTableMaxBlockLengthDefaultValue = 4000 // 4 KB

	errWhileReadingIndexBlock    = "error while reading index block"
	potentialIndexBlockCorrupted = "index block seems incomplete or corrupted"
)

type indexBlockEntry struct {
	key    string
	offset int
}

type DB struct {
	wal                   *wal.Wal
	memTable              *memtable.Memtable
	ssTableFiles          []*os.File
	ssTableIndexes        [][]indexBlockEntry
	ssTableMaxBlockLength int
}

func (db *DB) flushMemtableToSsTable() error {
	// 1. Iterate through the memtable and insert all content in a new ss table file
	numFiles := len(db.ssTableFiles)
	if err := os.MkdirAll("ss_table/l0", 0755); err != nil {
		return err
	}
	ssTableFilePath := fmt.Sprintf("ss_table/l0/l0_%d.log", numFiles)
	// 1.1 Create and open file in append-only mode
	file, err := os.OpenFile(ssTableFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	// 1.2 Identify 4kB blocks, write them and build up the struct for index block
	blockLength := 0
	blockStartOffset := 0
	blockFirstKey := ""
	ssTableBlock := ""
	offset := 0
	indexBlock := []indexBlockEntry{}
	db.memTable.Iterate(func(key, value string) {
		if blockFirstKey == "" {
			blockFirstKey = key
		}
		ssTableEntry := fmt.Sprintf("PUT %s %s\n", key, value)
		offset += len(ssTableEntry)
		blockLength += len(ssTableEntry)
		ssTableBlock = ssTableBlock + ssTableEntry
		if blockLength > db.ssTableMaxBlockLength {
			// one data block completed

			// add relevant details to indexBlock
			// [key_length -> 4 bytes][key][offset -> 4 bytes]
			indexBlock = append(indexBlock, indexBlockEntry{
				key:    blockFirstKey,
				offset: blockStartOffset,
			})

			// write this data block to the file
			// todo: change data block entry to [length][payload][checksum]xx
			// instead of "PUT key value\n"
			// if _, err := file.Write([]byte(ssTableBlock)); err != nil {
			// 	return err
			// }
			file.Write([]byte(ssTableBlock))

			// start new block
			blockStartOffset = offset
			blockFirstKey = ""
			blockLength = 0
			ssTableBlock = ""
		}
	})

	// add last data block
	indexBlock = append(indexBlock, indexBlockEntry{
		key:    blockFirstKey,
		offset: blockStartOffset,
	})
	file.Write([]byte(ssTableBlock))

	indexBlockStartOffset := offset

	// 1.3 Write index blocks
	for _, ib := range indexBlock {
		keyLength := len(ib.key)
		indexBuf := make([]byte, 4+keyLength+4)
		binary.BigEndian.PutUint32(indexBuf[0:4], uint32(keyLength))
		copy(indexBuf[4:4+keyLength], []byte(ib.key))
		binary.BigEndian.PutUint32(indexBuf[4+keyLength:], uint32(ib.offset))
		if _, err := file.Write(indexBuf); err != nil {
			return err
		}
	}

	// 1.4 Write footer
	footerBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(footerBuf[0:4], uint32(indexBlockStartOffset))
	file.Write(footerBuf)

	// 2. Clear the memtable and WAL
	db.memTable.Clear()
	db.wal.Clear()

	// 3. Update ssTables
	db.ssTableFiles = append(db.ssTableFiles, file)

	// 4. Update ssTableIndexes
	ssTableIndex, err := buildSsTableIndexFromFile(file)
	if err != nil {
		return err
	}
	db.ssTableIndexes = append(db.ssTableIndexes, ssTableIndex)

	return nil
}

func (db *DB) writeToWal(key, value string) error {
	payload := fmt.Sprintf("PUT %s %s\n", key, value)
	return db.wal.WriteEntry(payload)
}

func (db *DB) buildMemtableFromWal() (*memtable.Memtable, error) {
	memTable := memtable.NewMemtable()
	for {
		payload, err := db.wal.ReadEntry()
		if err == io.EOF {
			return &memTable, nil
		}
		// for now, I will abort even in case of partial write
		// todo: in case of partial write we should just truncate that log.
		// we can also do that as part of listening to signal SIGTERM and SIGKILL?
		if err != nil {
			return nil, err
		}
		line := string(payload)
		args := strings.Split(line, " ")
		if len(args) != 3 {
			return nil, errors.New("Expected exactly 2 arguments for PUT command\n")
		}
		key := args[1]
		value := args[2]
		memTable.Put(key, value)
	}
}

func buildSsTableIndexFromFile(file *os.File) ([]indexBlockEntry, error) {
	// 1. read footer and get the index offset
	info, err := os.Stat(file.Name())
	fileSize := info.Size()
	footerOffset := fileSize - 4
	indexOffsetBuf := make([]byte, 4)
	if _, err = file.ReadAt(indexOffsetBuf, footerOffset); err != nil {
		return nil, err
	}
	indexOffset := binary.BigEndian.Uint32(indexOffsetBuf)

	// 2. load index in-memory
	// 2.1 read index byte array
	indexBlockLength := (fileSize - 4) - int64(indexOffset)
	indexBlockBuf := make([]byte, indexBlockLength)
	if _, err = file.ReadAt(indexBlockBuf, int64(indexOffset)); err != nil {
		return nil, err
	}

	// 2.2 read keys and offsets from the index block and create in-memory index
	ssTableIndex := []indexBlockEntry{}
	for i := 0; i < int(indexBlockLength); {
		// read first 4 bytes to get length
		keyLengthBuf := indexBlockBuf[i : i+4]
		keyLength := binary.BigEndian.Uint32(keyLengthBuf)

		// read next keyLength bytes
		i += 4
		if i >= int(indexBlockLength) {
			return nil, errors.New(errWhileReadingIndexBlock + ": " + potentialIndexBlockCorrupted)
		}
		key := string(indexBlockBuf[i : i+int(keyLength)])

		// read offset
		i += int(keyLength)
		if i >= int(indexBlockLength) {
			return nil, errors.New(errWhileReadingIndexBlock + ": " + potentialIndexBlockCorrupted)
		}
		offsetBuf := indexBlockBuf[i : i+4]
		offset := binary.BigEndian.Uint32(offsetBuf)

		ssTableIndex = append(ssTableIndex, indexBlockEntry{key: key, offset: int(offset)})
		i += 4
	}
	return ssTableIndex, nil
}

func buildSsTableIndexes(files []*os.File) ([][]indexBlockEntry, error) {
	ssTableIndexes := [][]indexBlockEntry{}
	for _, file := range files {
		ssTableIndex, err := buildSsTableIndexFromFile(file)
		if err != nil {
			return nil, err
		}
		ssTableIndexes = append(ssTableIndexes, ssTableIndex)
	}
	return ssTableIndexes, nil
}

func getSsTableFiles() ([]*os.File, error) {
	if err := os.MkdirAll("ss_table/l0", 0755); err != nil {
		return nil, err
	}
	ssTableFiles := []*os.File{}
	// todo: os.Entries might be a better approach if there are frequent deletes due to compaction
	for i := 0; ; i++ {
		filePath := fmt.Sprintf("ss_table/l0/l0_%d.log", i)
		file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
		// as per current design, if a file is not found for l0_0, it won't also be present
		// for l0_1.
		if os.IsNotExist(err) {
			return ssTableFiles, nil
		}
		if err != nil {
			return nil, err
		}
		ssTableFiles = append(ssTableFiles, file)
	}
}

func newDB(walFilePath string) (*DB, error) {
	db := DB{}
	wal, err := wal.NewWal("")
	if err != nil {
		return nil, err
	}
	db.wal = wal

	memTable, err := db.buildMemtableFromWal()
	if err != nil {
		return nil, err
	}
	db.memTable = memTable

	db.ssTableMaxBlockLength = ssTableMaxBlockLengthDefaultValue

	ssTableFiles, err := getSsTableFiles()
	db.ssTableFiles = ssTableFiles

	ssTableIndexes, err := buildSsTableIndexes(ssTableFiles)
	if err != nil {
		return nil, err
	}
	db.ssTableIndexes = ssTableIndexes
	return &db, nil
}

func getLowerBound(key string, index []indexBlockEntry) int {
	low := 0
	high := len(index) - 1
	lowerBoundSliceIndex := -1
	for low <= high {
		mid := low + (high-low)/2
		if index[mid].key <= key {
			lowerBoundSliceIndex = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return lowerBoundSliceIndex
}

func getValueFromSsTableDataBlock(ssTableFile *os.File, key string, dataBlockOffset, dataBlockMaxLength int) (string, error) {
	ssTableDataBlockBuf := make([]byte, dataBlockMaxLength)
	_, err := ssTableFile.ReadAt(ssTableDataBlockBuf, int64(dataBlockOffset))
	if err != nil && err != io.EOF {
		return "", err
	}
	ssTableDataBlockEntries := strings.Split(string(ssTableDataBlockBuf), "\n")
	for _, payload := range ssTableDataBlockEntries {
		cmds := strings.Split(payload, " ")
		if cmds[1] == key {
			return cmds[2], nil
		}
	}
	return "", nil
}

// todo: this needs to be moved to ssTable package
func (db *DB) getValueFromSsTable(key string) (string, error) {
	// newest File to oldest File
	for i := len(db.ssTableFiles) - 1; i >= 0; i-- {
		file := db.ssTableFiles[i]
		ssTableIndex := db.ssTableIndexes[i]
		lowerBoundSliceIndex := getLowerBound(key, ssTableIndex)
		if lowerBoundSliceIndex == -1 {
			continue
		}
		value, err := getValueFromSsTableDataBlock(file, key, ssTableIndex[lowerBoundSliceIndex].offset, db.ssTableMaxBlockLength)
		if value == "" && err == nil {
			continue
		}
		return value, err
		// need to read from the file
	}
	return "", nil
}

func main() {
	db, err := newDB("")
	defer db.wal.Close()
	if err != nil {
		log.Fatal("Error while setting up DB: ", err.Error())
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		breakLoop := false
		switch cmd {
		case "GET":
			value, err := db.cmdGet(args)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("GET %s returned: %s\n", args[1], value)
			}

		case "PUT":
			err := db.cmdPut(args)
			if err != nil {
				fmt.Printf("Error while performing PUT operation: '%s'\n", err.Error())
			} else {
				fmt.Println("PUT operation performed successfully")
			}
		case "EXIT":
			breakLoop = true
		default:
			fmt.Println("Command not supported")
		}
		if breakLoop {
			break
		}
	}
}
