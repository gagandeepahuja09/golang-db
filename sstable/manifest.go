package sstable

import (
	"encoding/json"
	"fmt"
	"os"
)

type manifest struct {
	NextFileId int `json:"next_file_id"`
	// fileNames in manifest indicates the actual order. Example: due to compaction, it is
	// possible that 5.json has older data compared to 4.json
	// FileNames will always have the oldest file first
	FileNames []string `json:"file_names"`
}

func (st *SsTable) getManifest() (*manifest, error) {
	filePath := fmt.Sprintf("%s/%d", st.dataFilesDirectory, manifestJsonFileName)
	manifestJsonFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	fileInfo, _ := manifestJsonFile.Stat()
	manifestBuf := make([]byte, fileInfo.Size())
	manifestJsonFile.Read(manifestBuf)
	if err != nil {
		return nil, err
	}

	var manifest manifest
	err = json.Unmarshal(manifestBuf, &manifest)
	return &manifest, err
}

func (st *SsTable) saveManifest() error {
	manifestJsonBuf, err := json.MarshalIndent(st.manifest, "", " ")
	if err != nil {
		return err
	}
	filePath := fmt.Sprintf("%s/%d", st.dataFilesDirectory, manifestJsonFileName)
	err = os.WriteFile(filePath, manifestJsonBuf, 0644)
	return err
}

func (st *SsTable) getAllLogFiles() ([]*os.File, error) {
	fileNames := st.manifest.FileNames
	ssTableFiles := []*os.File{}
	for _, fileName := range fileNames {
		filePath := fmt.Sprintf("%s/%d", st.dataFilesDirectory, fileName)
		file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
		if err != nil {
			return nil, err
		}
		ssTableFiles = append(ssTableFiles, file)
	}
	return ssTableFiles, nil
}
