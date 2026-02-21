package wal

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWal_WriteAndReadSingleEntry
// 1. run Put, 2. read the data from wal.

var tempWalFile = "temp.log"

// Testing pattern ==> writes few entries --> restart --> write few more entries --> restart
// --> read and assert all entries
// note: separate instances of wal: read and write created for following reasons:
// 1. Matches production pattern: ReadEntry() during init --> WriteEntry -->
// application restarts -->
// 2. wal file opens in append only mode (O_APPEND), hence the read offset will always be
// at the end of the file.
func TestWal_PersistsAcrossRestart(t *testing.T) {
	defer os.Remove(tempWalFile)
	walWrite, err := NewWal(tempWalFile)
	assert.NoError(t, err)
	for i := 0; i <= 10; i++ {
		payload := fmt.Sprintf("PUT Key_%d Value_%d", i, i)
		err = walWrite.WriteEntry([]byte(payload))
		assert.NoError(t, err)
	}
	walWrite.Close()

	walWriteAfterRestart, err := NewWal(tempWalFile)
	assert.NoError(t, err)
	for i := 20; i <= 30; i++ {
		payload := fmt.Sprintf("PUT Key_%d Value_%d", i, i)
		err = walWriteAfterRestart.WriteEntry([]byte(payload))
		assert.NoError(t, err)
	}
	walWriteAfterRestart.Close()

	walRead, err := NewWal(tempWalFile)
	i := 0
	for {
		readPayload, err := walRead.ReadEntry()
		if i <= 30 {
			assert.NoError(t, err)
		} else {
			assert.Equal(t, "EOF", err.Error())
			break
		}
		expectedPayload := fmt.Sprintf("PUT Key_%d Value_%d", i, i)
		assert.Equal(t, expectedPayload, string(readPayload))
		if i == 10 {
			i += 10
		} else {
			i++
		}
	}

	walRead.Close()
}

func TestWal_PartialWrites(t *testing.T) {
	testCases := []struct {
		name          string
		truncateSize  int
		expectedError string
	}{
		{
			name:          "incomplete length",
			truncateSize:  3,
			expectedError: "partial write: incomplete length",
		},
		{
			name:          "incomplete payload",
			truncateSize:  13,
			expectedError: "partial write: incomplete payload",
		},
		{
			name:          "incomplete checksum",
			truncateSize:  20,
			expectedError: "partial write: incomplete checksum",
		},
		{
			name:          "complete write",
			truncateSize:  21,
			expectedError: "",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tempWalFile)
			walWrite, err := NewWal(tempWalFile)
			assert.NoError(t, err)

			err = walWrite.WriteEntry([]byte("PUT key value"))
			assert.NoError(t, err)

			os.Truncate(tempWalFile, int64(tt.truncateSize))

			walRead, err := NewWal(tempWalFile)
			assert.NoError(t, err)

			_, err = walRead.ReadEntry()
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.expectedError, err.Error())
			}
		})
	}
}

// todo: corrupted test case
func TestWal_CorruptedWrites(t *testing.T) {
	testCases := []struct {
		name          string
		corruptOffset int
	}{
		{
			name:          "corrupted payload",
			corruptOffset: 6,
		},
		{
			name:          "corrupted checksum",
			corruptOffset: 17,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tempWalFile)
			walWrite, err := NewWal(tempWalFile)
			assert.NoError(t, err)

			err = walWrite.WriteEntry([]byte("PUT key value"))
			assert.NoError(t, err)

			file, err := os.OpenFile(tempWalFile, os.O_RDWR, 0644)
			file.WriteAt([]byte("A"), int64(tt.corruptOffset))

			walRead, err := NewWal(tempWalFile)
			assert.NoError(t, err)

			_, err = walRead.ReadEntry()
			assert.Equal(t, "corrupt: checksum mismatch", err.Error())
		})
	}
}

func TestWal_Clear(t *testing.T) {
	defer os.Remove(tempWalFile)
	walWrite, err := NewWal(tempWalFile)
	assert.NoError(t, err)

	payload := "PUT key value"
	err = walWrite.WriteEntry([]byte(payload))
	assert.NoError(t, err)

	walRead, err := NewWal(tempWalFile)
	assert.NoError(t, err)

	readPayload, err := walRead.ReadEntry()
	assert.Equal(t, payload, string(readPayload))

	walReadAfterClear, err := NewWal(tempWalFile)
	walReadAfterClear.Clear()
	readPayload, err = walReadAfterClear.ReadEntry()
	assert.Equal(t, "EOF", err.Error())
}
