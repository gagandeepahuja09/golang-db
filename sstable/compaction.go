package sstable

// todo: handle concurrent reads and writes between Compaction and regular Get, Write
// functions
import (
	"log/slog"
	"strings"
)

func (st *SsTable) ShouldRunCompaction() bool {
	return !st.compacting && len(st.firstLevelFiles) >= 4
}

// builds a compactedMap formed from all the key value pairs present in the files.
// we go from the oldest file to the newest one to ensure that the key has the most up-to-date value.
func (st *SsTable) getCompactedMap() (map[string]string, error) {
	files := st.firstLevelFiles
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
	compactedMap, err := st.getCompactedMap()
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
		slog.Error("COMPACTED_FILE_CREATION_FAILED", "error", err.Error())
	}
	st.Write(compactedFile, iterator)
}
