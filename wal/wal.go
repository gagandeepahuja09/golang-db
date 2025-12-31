package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
)

const (
	defaultWalPath = "wal.log"
)

type Wal struct {
	file *os.File
}

// WriteEntry writes [length][payload][checksum] to file
func (w *Wal) WriteEntry(payload string) error {
	buf := make([]byte, 4+len(payload)+4)
	checksum := crc32.ChecksumIEEE([]byte(payload))
	// 1. add length
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(payload)))
	// 2. add payload / payload
	copy(buf[4:4+len(payload)], []byte(payload))
	// 3. add checksum
	binary.BigEndian.PutUint32(buf[4+len(payload):], checksum)
	if _, err := w.file.Write(buf); err != nil {
		slog.Error("WAL_WRITE_FAILED", "error", err.Error())
		return err
	}
	return w.file.Sync()
}

// Close closes the walFile
func (w *Wal) Close() {
	w.file.Close()
}

// ReadEntry reads one [length][payload][checksum] record and returns the extracted payload
func (w *Wal) ReadEntry() (payload []byte, err error) {
	// 1. Read Length (4 bytes)
	lengthBuf := make([]byte, 4)
	_, err = io.ReadFull(w.file, lengthBuf)
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
	_, err = io.ReadFull(w.file, payload)
	if err == io.ErrUnexpectedEOF {
		return nil, errors.New("partial write: incomplete payload")
	}
	if err != nil {
		return nil, err
	}

	// 5. Read checksum
	checksumBuf := make([]byte, 4)
	_, err = io.ReadFull(w.file, checksumBuf)
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

// Clear clears the content in the wal file
// It is typically used when the data is persisted to some other persistent storage
// which is faster for retrieval, typically disk IO
func (w *Wal) Clear() {
	w.file.Truncate(0)
	w.file.Seek(0, 0)
}

// NewWal creates a new instance of Wal which can be utilised in any DB.
// If empty filePath is provided, reads and writes will happen from default location: "wal.log"
func NewWal(filePath string) (*Wal, error) {
	if filePath == "" {
		filePath = defaultWalPath
	}
	w := Wal{}
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		slog.Error("WAL_FILE_OPEN_FAILED", "error", err.Error())
		return nil, err
	}
	w.file = file

	return &w, nil
}
