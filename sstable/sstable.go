package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	dataFilesDefaultDirectory   = "data_files_sstable"
	firstLevelFilesSubdirectory = "l0"
	defaultBlockLength          = 100

	errWhileReadingIndexBlock    = "error while reading index block"
	potentialIndexBlockCorrupted = "index block seems incomplete or corrupted"
)

type indexBlockEntry struct {
	key    string
	offset int
}

type SsTable struct {
	dataFilesDirectory string
	firstLevelFiles    []*os.File
	blockLength        int
	indexBlocks        [][]indexBlockEntry
}

func NewSsTable(dataFilesDirectory string, blockLength int) (*SsTable, error) {
	if dataFilesDirectory == "" {
		dataFilesDirectory = dataFilesDefaultDirectory
	}
	if blockLength == 0 {
		blockLength = defaultBlockLength
	}
	st := SsTable{
		dataFilesDirectory: dataFilesDirectory,
		blockLength:        blockLength,
		firstLevelFiles:    make([]*os.File, 0),
		indexBlocks:        make([][]indexBlockEntry, 0),
	}

	firstLevelFiles, err := st.getAllFirstLevelFilesFromDirectory()
	if err != nil {
		return nil, err
	}
	st.firstLevelFiles = firstLevelFiles

	indexBlocks, err := st.buildIndexes(st.firstLevelFiles)
	st.indexBlocks = indexBlocks
	return &st, err
}

// create a new first level file
func (st *SsTable) NewFile() (*os.File, error) {
	numFiles := len(st.firstLevelFiles)
	if err := os.MkdirAll(st.dataFilesDirectory, 0755); err != nil {
		return nil, err
	}
	ssTableFilePath := fmt.Sprintf("%s/%d.log", st.dataFilesDirectory, numFiles)
	file, err := os.OpenFile(ssTableFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Write writes a stream of key, value pairs to the required file as per the format
// of SSTable file which is [data-block(s)][index-block][footer].
// It calls the iteratorFunc function to get a stream of key, value pairs from a source.
// example: 1. MemTable OR 2. firstLevelFiles which need to be merged and compacted.
// It also updates the internal structs for firstLevelFiles and indexBlocks
func (st *SsTable) Write(file *os.File, iteratorFunc func(fn func(key, value string))) error {
	indexOffset, indexBlock, err := st.writeDataBlocks(file, iteratorFunc)
	if err != nil {
		return err
	}
	if err = st.writeIndexBlock(file, indexBlock); err != nil {
		return err
	}
	if err = st.writeFooter(file, indexOffset); err != nil {
		return err
	}

	st.firstLevelFiles = append(st.firstLevelFiles, file)
	st.indexBlocks = append(st.indexBlocks, indexBlock)
	return nil
}

func (st *SsTable) writeFooter(file *os.File, indexBlockStartOffset int) error {
	footerBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(footerBuf[0:4], uint32(indexBlockStartOffset))
	_, err := file.Write(footerBuf)
	return err
}

// writeDataBlocks breaks down the blocks into blocks of fixed size as defined in ssTable.blockLength.
// the last block might have lesser blockLength.
// It also returns:
// 1. Offset from which the index block should be written. This is also important to be tracked in the file footer.
// 2. A struct slice for the index block entries which is next written to the ssTable file.
func (st *SsTable) writeDataBlocks(file *os.File, iteratorFunc func(fn func(key, value string))) (int,
	[]indexBlockEntry, error) {
	blockLength := 0
	blockStartOffset := 0
	blockFirstKey := ""
	ssTableBlock := ""
	offset := 0
	indexBlock := []indexBlockEntry{}

	var err error

	iteratorFunc(func(key, value string) {
		if blockFirstKey == "" {
			blockFirstKey = key
		}
		ssTableEntry := fmt.Sprintf("PUT %s %s\n", key, value)
		offset += len(ssTableEntry)
		blockLength += len(ssTableEntry)
		ssTableBlock = ssTableBlock + ssTableEntry
		if blockLength > st.blockLength {
			// one data block completed

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
			if _, err = file.Write([]byte(ssTableBlock)); err != nil {
				// todo: add some break statement
				// break
			}

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
	_, err = file.Write([]byte(ssTableBlock))
	return offset, indexBlock, err
}

func (st *SsTable) writeIndexBlock(file *os.File, indexBlock []indexBlockEntry) error {
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
	return nil
}

func (st *SsTable) getAllFirstLevelFilesFromDirectory() ([]*os.File, error) {
	if err := os.MkdirAll(st.dataFilesDirectory, 0755); err != nil {
		return nil, err
	}
	ssTableFiles := []*os.File{}
	// todo: os.Entries might be a better approach if there are frequent deletes due to compaction
	for i := 0; ; i++ {
		filePath := fmt.Sprintf("%s/%d.log", st.dataFilesDirectory, i)
		file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
		// as per current design, if a file is not found for 0th file, it won't also be present
		// 1st file.
		if os.IsNotExist(err) {
			return ssTableFiles, nil
		}
		if err != nil {
			return nil, err
		}
		ssTableFiles = append(ssTableFiles, file)
	}
}

func (st *SsTable) buildIndexes(files []*os.File) ([][]indexBlockEntry, error) {
	ssTableIndexes := [][]indexBlockEntry{}
	for _, file := range files {
		ssTableIndex, err := buildIndexFromFile(file)
		if err != nil {
			return nil, err
		}
		ssTableIndexes = append(ssTableIndexes, ssTableIndex)
	}
	return ssTableIndexes, nil
}

func buildIndexFromFile(file *os.File) ([]indexBlockEntry, error) {
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

func (st *SsTable) Get(key string) (string, error) {
	// newest file to oldest file
	for i := len(st.firstLevelFiles) - 1; i >= 0; i-- {
		file := st.firstLevelFiles[i]
		ssTableIndex := st.indexBlocks[i]
		lowerBoundSliceIndex := getLowerBound(key, ssTableIndex)
		if lowerBoundSliceIndex == -1 {
			continue
		}
		endOffset := ssTableIndex[lowerBoundSliceIndex].offset + st.blockLength
		if lowerBoundSliceIndex < len(ssTableIndex)-1 {
			endOffset = ssTableIndex[lowerBoundSliceIndex+1].offset
		}
		value, err := st.getValueFromSsTableDataBlock(file, key,
			ssTableIndex[lowerBoundSliceIndex].offset, endOffset)
		if value == "" && err == nil {
			continue
		}
		return value, err
	}
	return "", nil
}

func (st *SsTable) getValueFromSsTableDataBlock(ssTableFile *os.File, key string, dataBlockStartOffset, dataBlockEndOffset int) (string, error) {
	ssTableDataBlockBuf := make([]byte, dataBlockEndOffset-dataBlockStartOffset+1)
	_, err := ssTableFile.ReadAt(ssTableDataBlockBuf, int64(dataBlockStartOffset))
	if err != nil && err != io.EOF {
		return "", err
	}
	ssTableDataBlockEntries := strings.Split(string(ssTableDataBlockBuf), "\n")
	for _, payload := range ssTableDataBlockEntries {
		cmds := strings.Split(payload, " ")
		if len(cmds) < 2 {
			continue
		}
		if cmds[1] == key {
			return cmds[2], nil
		}
	}
	return "", nil
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
