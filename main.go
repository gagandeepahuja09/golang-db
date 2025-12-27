package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/golang-db/memtable"
)

const (
	ssTableMaxBlockLength = 4000 // 4 KB
)

type indexBlockEntry struct {
	key    string
	offset int
}

type DB struct {
	walFile      *os.File
	memTable     memtable.Memtable
	ssTableFiles []*os.File
}

func (db *DB) cmdGet(args []string) {
	if len(args) != 2 {
		fmt.Fprint(os.Stderr, "Expected exactly 1 argument for GET command\n")
		return
	}
	key := args[1]
	value, ok := db.memTable.Get(key)
	if !ok {
		fmt.Printf("No value found for GET %s\n", key)
	} else {
		fmt.Printf("GET %s returned: %s\n", key, value)
	}
}

func (db *DB) cmdPut(args []string) error {
	if len(args) != 3 {
		return errors.New("Expected exactly 2 arguments for PUT command\n")
	}
	key := args[1]
	value := args[2]
	if err := db.writeToWal(key, value); err != nil {
		return errors.New("Something went wrong")
	}
	db.memTable.Put(key, value)

	if db.memTable.ShouldFlush() {
		// todo: log for error
		db.flushMemtableToSsTable()
	}
	return nil
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
	indexBlockStartOffset := 0
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
		if blockLength > ssTableMaxBlockLength {
			// one data block completed

			// add relevant details to indexBlock
			// [key_length -> 4 bytes][key][offset -> 4 bytes]
			indexBlock = append(indexBlock, indexBlockEntry{
				key:    blockFirstKey,
				offset: blockStartOffset,
			})

			indexBlockStartOffset += blockStartOffset

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

	// 1.3 Write index blocks
	for _, ib := range indexBlock {
		indexBuf := make([]byte, 4+len(ib.key)+4)
		binary.BigEndian.PutUint32(indexBuf[0:4], uint32(len(ib.key)))
		copy(indexBuf[4:4+len(ib.key)], []byte(ib.key))
		binary.BigEndian.PutUint32(indexBuf[4+len(ib.key):], uint32(ib.offset))
		if _, err := file.Write(indexBuf); err != nil {
			return err
		}
	}

	// 1.4 Write footer
	footerBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(footerBuf[0:4], uint32(indexBlockStartOffset))

	// 2. Clear the memtable and WAL
	db.memTable.Clear()
	db.walFile.Truncate(0)
	db.walFile.Seek(0, 0)

	return nil
}

func (db *DB) writeToWal(key, value string) error {
	cmd := fmt.Sprintf("PUT %s %s\n", key, value)
	buf := make([]byte, 4+len(cmd)+4)
	checksum := crc32.ChecksumIEEE([]byte(cmd))
	// 1. add length
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(cmd)))
	// 2. add cmd / payload
	copy(buf[4:4+len(cmd)], []byte(cmd))
	// 3. add checksum
	binary.BigEndian.PutUint32(buf[4+len(cmd):], checksum)
	if _, err := db.walFile.Write(buf); err != nil {
		slog.Error("WAL_WRITE_FAILED", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	return db.walFile.Sync()
}

func readEntry(file *os.File) (payload []byte, err error) {
	// 1. Read Length (4 bytes)
	lengthBuf := make([]byte, 4)
	_, err = io.ReadFull(file, lengthBuf)
	if err == io.EOF {
		return nil, io.EOF
	}
	if err == io.ErrUnexpectedEOF {
		return nil, errors.New("partial write: incomplete length")
	}
	if err != nil {
		return nil, err
	}
	// 2. Parse Length
	payloadLength := binary.BigEndian.Uint32(lengthBuf)

	// 3. Sanity Check
	if payloadLength > 1_000_000 { // 1 MB max
		return nil, errors.New("corrupt: length too large")
	}

	// 4. Read payload
	payload = make([]byte, payloadLength)
	_, err = io.ReadFull(file, payload)
	if err == io.ErrUnexpectedEOF {
		return nil, errors.New("partial write: incomplete payload")
	}
	if err != nil {
		return nil, err
	}

	// 5. Read checksum
	checksumBuf := make([]byte, 4)
	_, err = io.ReadFull(file, checksumBuf)
	if err == io.ErrUnexpectedEOF {
		return nil, errors.New("partial write: incomplete checksum")
	}
	if err != nil {
		return nil, err
	}

	// 6. Verify checksum
	storedChecksum := binary.BigEndian.Uint32(checksumBuf)
	computedChecksum := crc32.ChecksumIEEE(payload)
	if storedChecksum != computedChecksum {
		return nil, errors.New("corrupt: checksum mismatch")
	}

	return payload, err
}

func buildDatabaseFromWal() (*DB, error) {
	file, err := os.OpenFile("wal.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		slog.Error("WAL_FILE_OPEN_FAILED", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}
	db := &DB{
		walFile:  file,
		memTable: memtable.NewMemtable(),
	}

	for {
		payload, err := readEntry(file)
		if err == io.EOF {
			return db, nil
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
		db.memTable.Put(key, value)
	}
}

func main() {
	db, err := buildDatabaseFromWal()
	if err != nil {
		log.Fatal("Error while setting up DB")
	}
	defer db.walFile.Close()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		breakLoop := false
		switch cmd {
		case "GET":
			db.cmdGet(args)
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
