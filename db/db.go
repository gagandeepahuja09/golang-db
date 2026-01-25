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

const (
	CatalogKey     = "_calatog"
	SchemaTemplate = "_schema%s"
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
	// tablesString, err :=
	db.Get(CatalogKey)
	if err != nil {
		return nil, err
	}

	// tables := strings.Split(tablesString, ",")
	// for _, table := range tables {
	// 	schemaStr, err := db.Get(fmt.Sprintf(SchemaTemplate, table))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
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

	var tableNames string
	for i, table := range db.tables {
		tableNames += table.TableName
		if i < len(db.tables)-1 {
			tableNames += ","
		}
	}

	// PERFORM PUT operation with _catalog key and all table names.
	db.Put(CatalogKey, tableNames)

	// PERFORM PUT operation with _schema:[table_name] key
	db.Put(fmt.Sprintf(SchemaTemplate, createTableInput.TableName), string(
		serialiseCreateTableInput(createTableInput)))

	return nil
}

// serialisation strategy: [PK_column_position][columnDataType1][columnNameLength1][columnName1][columnDataType2][columnNameLength2][columnName2]...
func serialiseCreateTableInput(createTableInput sqlparser.CreateTable) []byte {
	serialisedSchema := []byte{}
	serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(createTableInput.PrimaryKeyColumnPosition))

	for _, col := range createTableInput.ColumnDetails {
		serialisedSchema = append(serialisedSchema, col.DataType)
		serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(len(col.ColumnName)))
		serialisedSchema = append(serialisedSchema, []byte(col.ColumnName)...)
	}

	return serialisedSchema
}

func deserialiseCreateTableInput(buf []byte) (*sqlparser.CreateTable, error) {
	var createTableMeta sqlparser.CreateTable
	i := 0
	if len(buf) < 4 {
		return nil, errors.New("unexpected error while reading primary key column position")
	}
	primaryKeyColumnPosition := binary.BigEndian.Uint32(buf[i : i+4])
	createTableMeta.PrimaryKeyColumnPosition = int(primaryKeyColumnPosition)
	i += 4

	columnDetails := []sqlparser.Column{}
	for i < len(buf) {
		var columnMeta sqlparser.Column
		dataType := buf[i]
		if i+1 > len(buf) {
			return nil, errors.New("unexpected error while reading column data type")
		}
		columnMeta.DataType = dataType
		i++

		if i+4 > len(buf) {
			return nil, errors.New("unexpected error while reading column length")
		}
		columnNameLength := binary.BigEndian.Uint32(buf[i : i+4])
		i += 4

		if i+int(columnNameLength) > len(buf) {
			return nil, errors.New("unexpected error while reading column name")
		}
		columnMeta.ColumnName = string(buf[i : i+int(columnNameLength)])
		i += int(columnNameLength)

		columnDetails = append(columnDetails, columnMeta)
	}
	createTableMeta.ColumnDetails = columnDetails

	return &createTableMeta, nil
}
