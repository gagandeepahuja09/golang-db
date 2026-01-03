package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	dataFilesDefaultDirectory   = "data_files_sstable"
	firstLevelFilesSubdirectory = "l0"
	defaultBlockLength          = 100

	errWhileReadingIndexBlock    = "error while reading index block"
	potentialIndexBlockCorrupted = "index block seems incomplete or corrupted"
	manifestJsonFileName         = "manifest.json"
)

type indexBlockEntry struct {
	key    string
	offset int
}

type SsTable struct {
	mutex              sync.RWMutex
	dataFilesDirectory string
	firstLevelFiles    []*os.File
	blockLength        int
	indexBlocks        [][]indexBlockEntry
	manifest           manifest
	skipIndex          bool // added only for benchmarking. Default is that index will always be used
	compacting         bool
}

type Config struct {
	DataFilesDirectory string
	BlockLength        int
	SkipIndex          bool
}

func NewSsTable(config Config) (*SsTable, error) {
	if config.DataFilesDirectory == "" {
		config.DataFilesDirectory = dataFilesDefaultDirectory
	}
	if config.BlockLength == 0 {
		config.BlockLength = defaultBlockLength
	}
	st := SsTable{
		dataFilesDirectory: config.DataFilesDirectory,
		blockLength:        config.BlockLength,
		skipIndex:          config.SkipIndex,
		firstLevelFiles:    make([]*os.File, 0),
		indexBlocks:        make([][]indexBlockEntry, 0),
		mutex:              sync.RWMutex{},
	}

	directoryMetadata, err := st.getDirectoryMetadata()
	if err != nil {
		return nil, err
	}
	st.firstLevelFiles = directoryMetadata.firstLevelFiles
	st.manifest = directoryMetadata.manifest

	if st.skipIndex {
		return &st, err
	}
	indexBlocks, err := st.buildIndexes(st.firstLevelFiles)
	st.indexBlocks = indexBlocks
	return &st, err
}

// create a new first level file
func (st *SsTable) NewFile() (*os.File, error) {
	if err := os.MkdirAll(st.dataFilesDirectory, 0755); err != nil {
		return nil, err
	}
	st.mutex.Lock()
	id := st.manifest.NextFileId
	st.manifest.NextFileId++
	st.mutex.Unlock()
	ssTableFilePath := fmt.Sprintf("%s/%d.log", st.dataFilesDirectory, id)
	return os.OpenFile(ssTableFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
}

// Write writes a stream of key, value pairs to the required file as per the format
// of SSTable file which is [data-block(s)][index-block][footer].
// It calls the iteratorFunc function to get a stream of key, value pairs from a source.
// example: 1. MemTable OR 2. firstLevelFiles which need to be merged and compacted.
// It also updates the internal structs for firstLevelFiles and indexBlocks
func (st *SsTable) Write(file *os.File, iteratorFunc func(fn func(key, value string))) error {
	indexBlock, err := st.writeToFile(file, iteratorFunc)
	if err != nil {
		return err
	}

	st.mutex.Lock()
	st.firstLevelFiles = append(st.firstLevelFiles, file)
	if !st.skipIndex {
		st.indexBlocks = append(st.indexBlocks, indexBlock)
	}
	st.manifest.FileNames = append(st.manifest.FileNames, file.Name())

	st.saveManifest()
	st.mutex.Unlock()
	return nil
}

// Similar to Write function, but it doesn't update internal structs
// Write writes a stream of key, value pairs to the required file as per the format
// of SSTable file which is [data-block(s)][index-block][footer].
// It calls the iteratorFunc function to get a stream of key, value pairs from a source.
// example: 1. MemTable OR 2. firstLevelFiles which need to be merged and compacted.
func (st *SsTable) writeToFile(file *os.File, iteratorFunc func(fn func(key, value string))) ([]indexBlockEntry, error) {
	indexOffset, indexBlock, err := st.writeDataBlocks(file, iteratorFunc)
	if err != nil {
		return nil, err
	}
	if !st.skipIndex {
		if err = st.writeIndexBlock(file, indexBlock); err != nil {
			return nil, err
		}
		if err = st.writeFooter(file, indexOffset); err != nil {
			return nil, err
		}
	}
	return nil, err
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
	if blockFirstKey != "" {
		indexBlock = append(indexBlock, indexBlockEntry{
			key:    blockFirstKey,
			offset: blockStartOffset,
		})
		_, err = file.Write([]byte(ssTableBlock))
	}
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

// Gets the following metadata:
// 1. Reads manifest JSON to get the nextFileId and expected order of files. Populate in
// directoryMetadata.manifest.
// 2. Opens all sstable log files and populates in directoryMetadata.firstLevelFiles
func (st *SsTable) getDirectoryMetadata() (directoryMetadata *SsTable, err error) {
	directoryMetadata = &SsTable{}
	if err := os.MkdirAll(st.dataFilesDirectory, 0755); err != nil {
		return nil, err
	}
	manifest, err := st.getManifest()
	if err != nil {
		return nil, err
	}
	directoryMetadata.manifest = *manifest

	ssTableFiles, err := st.getAllLogFiles()
	if err != nil {
		return nil, err
	}
	directoryMetadata.firstLevelFiles = ssTableFiles
	return directoryMetadata, nil
}

func (st *SsTable) buildIndexes(files []*os.File) ([][]indexBlockEntry, error) {
	ssTableIndexes := [][]indexBlockEntry{}
	for _, file := range files {
		ssTableIndex, err := st.buildIndexFromFile(file)
		if err != nil {
			return nil, err
		}
		ssTableIndexes = append(ssTableIndexes, ssTableIndex)
	}
	return ssTableIndexes, nil
}

func (st *SsTable) getIndexOffset(file *os.File) (uint32, error) {
	info, err := os.Stat(file.Name())
	fileSize := info.Size()
	footerOffset := fileSize - 4
	indexOffsetBuf := make([]byte, 4)
	if _, err = file.ReadAt(indexOffsetBuf, footerOffset); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(indexOffsetBuf), nil
}

func (st *SsTable) buildIndexFromFile(file *os.File) ([]indexBlockEntry, error) {
	// 1. get the index offset
	info, err := os.Stat(file.Name())
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()
	indexOffset, err := st.getIndexOffset(file)
	if err != nil {
		return nil, err
	}

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
	st.mutex.RLock()
	defer st.mutex.RUnlock()
	if st.skipIndex {
		return st.linearSearch(key)
	}
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
			// todo: it is safer to have endOffset as start of index offset.
			// this can potentially lead to issue as more than
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

func (st *SsTable) linearSearch(key string) (string, error) {
	for i := len(st.firstLevelFiles) - 1; i >= 0; i-- {
		file := st.firstLevelFiles[i]
		value, err := st.linearSearchFile(file, key)
		if err != nil || value != "" {
			return value, err
		}
	}
	return "", nil
}

func (st *SsTable) linearSearchFile(file *os.File, key string) (string, error) {
	stat, _ := file.Stat()
	fileSize := stat.Size()
	buf := make([]byte, fileSize)
	_, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	entries := strings.Split(string(buf), "\n")
	for _, payload := range entries {
		cmds := strings.Split(payload, " ")
		if cmds[1] == key {
			return cmds[2], nil
		}
	}
	return "", nil
}
