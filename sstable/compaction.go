package sstable

// todo: handle concurrent reads and writes between Compaction and regular Get, Write
// functions
import (
	"log/slog"
	"os"
	"strings"
)

func (st *SsTable) ShouldRunCompaction() bool {
	return !st.compacting && len(st.firstLevelFiles) >= 4
}

// builds a compactedMap formed from all the key value pairs present in the files.
// we go from the oldest file to the newest one to ensure that the key has the most up-to-date value.
func (st *SsTable) getCompactedMap(files []*os.File) (map[string]string, error) {
	var compactedMap map[string]string
	for _, file := range files {
		indexOffset, err := st.getIndexOffset(file)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, indexOffset)
		_, err = file.ReadAt(buf, 0)
		if err != nil {
			return nil, err
		}
		entries := strings.Split(string(buf), "\n")
		for _, payload := range entries {
			cmds := strings.Split(payload, " ")
			key := cmds[1]
			value := cmds[2]
			compactedMap[key] = value
		}
	}
	return compactedMap, nil
}

func (st *SsTable) RunCompaction() {
	// 1. build compacted map
	var filesToCompact []*os.File
	st.mutex.RLock()
	copy(filesToCompact, st.firstLevelFiles)
	st.mutex.RUnlock()
	compactedMap, err := st.getCompactedMap(filesToCompact)
	if err != nil {
		slog.Error("COMPACTED_MAP_BUILD_FAILED", "error", err.Error())
	}
	// 2. get sorted keys
	sortedKeys := sortedKeys(compactedMap)

	// 3. create iterator function which calls the callback for each key-value pair in sorted
	// and compacted map
	iterator := func(fn func(key, value string)) {
		for _, key := range sortedKeys {
			value := compactedMap[key]
			fn(key, value)
		}
	}

	// 4. write to the compacted file
	compactedFile, err := st.NewFile()
	if err != nil {
		slog.Error("COMPACTED_FILE_CREATE_FAILED", "error", err.Error())
	}
	compactedIndexBlock, err := st.writeToFile(compactedFile, iterator)
	if err != nil {
		slog.Error("COMPACTED_FILE_WRITE_FAILED", "error", err.Error())
	}

	// 5. atomic swap of files array and indexes array
	st.atomicSwap(compactedFile, filesToCompact, compactedIndexBlock)

	// 6. delete old files
}

// takes the compacted file, old files array and current state of files array to construct the new
// ssTables array and sets it.
// similar behaviour done for indexes array.
func (st *SsTable) atomicSwap(compactedFile *os.File, oldFiles []*os.File, compactedIndexBlock []indexBlockEntry) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	var oldFilesMap map[string]bool

	for _, file := range oldFiles {
		oldFilesMap[file.Name()] = true
	}

	currentFiles := st.firstLevelFiles

	swappedFiles := []*os.File{compactedFile}
	fileNames := []string{compactedFile.Name()}
	swappedIndexBlocks := [][]indexBlockEntry{compactedIndexBlock}

	for i, file := range currentFiles {
		if !oldFilesMap[file.Name()] {
			swappedFiles = append(swappedFiles, file)
			swappedIndexBlocks = append(swappedIndexBlocks, st.indexBlocks[i])
			fileNames = append(fileNames, file.Name())
		}
	}

	st.firstLevelFiles = swappedFiles
	st.indexBlocks = swappedIndexBlocks

	st.manifest.FileNames = fileNames
	st.saveManifest()
}
