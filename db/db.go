package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/golang-db/memtable"
	sqlparser "github.com/golang-db/sql_parser"
	"github.com/golang-db/sstable"
	"github.com/golang-db/wal"
)

type DB struct {
	mu       sync.RWMutex
	wal      *wal.Wal
	memTable *memtable.Memtable
	ssTable  *sstable.SsTable
	tables   []sqlparser.CreateTable
}

type Config struct {
	SsTableConfig sstable.Config
}

func NewDB(config Config) (*DB, error) {
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
	db.ssTable, err = sstable.NewSsTable(config.SsTableConfig)

	// todo:
	// GET _catalog key during application init
	// GET all individual schemas for each of the tables
	// store in db.tables
	return &db, err
}

func (db *DB) Close() {
	db.wal.Close()
	// todo: close all sstable files
}

func (db *DB) Get(key string) (value string, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	value, ok := db.memTable.Get(key)
	if !ok {
		value, err = db.ssTable.Get(key)
	}
	return value, err
}

func (db *DB) createSsTableAndClearWalAndMemTable() error {
	if err := db.flushMemtableToSsTable(); err != nil {
		return err
	}
	db.memTable.Clear()
	db.wal.Clear()
	return nil
}

func (db *DB) Put(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.writeToWal(key, value); err != nil {
		return errors.New("Something went wrong")
	}
	db.memTable.Put(key, value)

	if db.memTable.ShouldFlush() {
		if err := db.createSsTableAndClearWalAndMemTable(); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) flushMemtableToSsTable() error {
	ssTableFile, err := db.ssTable.NewFile()
	if err != nil {
		return err
	}

	err = db.ssTable.Write(ssTableFile, db.memTable.Iterate)
	if db.ssTable.ShouldRunCompaction() {
		go db.ssTable.RunCompaction()
	}
	return err
}

func (db *DB) writeToWal(key, value string) error {
	payload := fmt.Sprintf("PUT %s %s", key, value)
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

func (db *DB) CreateTable(createTableInput sqlparser.CreateTable) error {
	db.tables = append(db.tables, createTableInput)
	serializedSchema := serializeSchema(createTableInput)

	// PERFORM PUT operation with _catalog key and all table names.
	// PERFORM PUT operation with _schema:[table_name] key

	return nil
}

// serialization strategy: [PK_column_position][columnDataType1][columnNameLength1][columnName1][columnDataType2][columnNameLength2][columnName2]...
func serializeSchema(createTableInput sqlparser.CreateTable) []byte {
	serializedSchema := []byte{}
	serializedSchema = binary.BigEndian.AppendUint32(serializedSchema, uint32(createTableInput.PrimaryKeyColumnPosition))

	for _, col := range createTableInput.ColumnDetails {
		serializedSchema = append(serializedSchema, col.DataType)
		serializedSchema = append(serializedSchema, []byte(col.ColumnName)...)
	}

	return serializedSchema
}
