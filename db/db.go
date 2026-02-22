package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/golang-db/memtable"
	sqlparser "github.com/golang-db/sql_parser"
	"github.com/golang-db/sstable"
	"github.com/golang-db/wal"
)

const (
	CatalogKey     = "_calatog"
	SchemaTemplate = "_schema:%s"
)

type LocksAcquired struct {
	writerTxnId  uint64
	readerTxnIds []uint64
}
type transactionManager struct {
	nextTransactionId     uint64
	mu                    sync.Mutex
	keyVsLocksAcquiredMap map[string]*LocksAcquired
}

type DB struct {
	mu                   sync.RWMutex
	wal                  *wal.Wal
	memTable             *memtable.Memtable
	ssTable              *sstable.SsTable
	tableNameVsSchemaMap map[string]sqlparser.CreateTable
	transactionManager   transactionManager
}

type Config struct {
	SsTableConfig sstable.Config
	WalFilePath   string
}

func NewDB(config Config) (*DB, error) {
	db := DB{}
	wal, err := wal.NewWal(config.WalFilePath)
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
	if err != nil {
		return nil, err
	}

	db.tableNameVsSchemaMap, err = db.getTableNameVsSchemaMap()
	if err != nil {
		return nil, err
	}

	db.transactionManager = transactionManager{
		nextTransactionId:     1,
		mu:                    sync.Mutex{},
		keyVsLocksAcquiredMap: map[string]*LocksAcquired{},
	}

	return &db, err
}

func (db *DB) getTableNameVsSchemaMap() (map[string]sqlparser.CreateTable, error) {
	tableNameVsSchemaMap := map[string]sqlparser.CreateTable{}
	tablesString, err := db.Get(CatalogKey)
	if err != nil || tablesString == "" {
		return nil, err
	}

	tableNames := strings.Split(tablesString, ",")
	for _, tableName := range tableNames {
		schemaStr, err := db.Get(fmt.Sprintf(SchemaTemplate, tableName))
		if err != nil {
			return nil, err
		}
		createTableInput, err := deserialiseCreateTableInput([]byte(schemaStr))
		if err != nil {
			return nil, err
		}
		createTableInput.TableName = tableName
		tableNameVsSchemaMap[tableName] = *createTableInput
	}
	return tableNameVsSchemaMap, nil
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
	return db.wal.WriteEntry([]byte(payload))
}

func handlePutCmd(memTable memtable.Memtable, line string) error {
	args := strings.Split(line, " ")
	if len(args) != 3 {
		return errors.New("Expected exactly 2 arguments for PUT command")
	}
	key := args[1]
	value := args[2]
	memTable.Put(key, value)
	return nil
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

		// read [command_length(4_bytes)][command_string] to figure out the command type
		// and accordingly deserialise.
		i := 0
		len := binary.BigEndian.Uint32(payload[i : i+4])
		i += 4
		cmd := string(payload[i : i+int(len)])
		i += int(len)
		if cmd == CmdTransaction {
			putCmds := deserialiseTransactionCommand(payload[i:])
			for _, cmd := range putCmds {
				if err := handlePutCmd(memTable, cmd); err != nil {
					return nil, err
				}
			}
		} else {
			if err := handlePutCmd(memTable, string(payload)); err != nil {
				return nil, err
			}
		}
	}
}

func (db *DB) CreateTable(query string) error {
	parser := sqlparser.NewParser(query)
	input, err := parser.ParseCreateTable()
	if err != nil {
		return err
	}
	return db.createTable(*input)
}

func (db *DB) Begin() (*Transaction, error) {
	db.transactionManager.mu.Lock()
	defer db.transactionManager.mu.Unlock()

	txn := Transaction{
		id: db.transactionManager.nextTransactionId,
		db: db,
	}
	db.transactionManager.nextTransactionId++
	return &txn, nil
}

func (db *DB) createTable(createTableInput sqlparser.CreateTable) error {
	// todo: checking for table name already exists
	// todo: since we are doing 2 different Put operations, createTable is not actually atomic
	db.tableNameVsSchemaMap[createTableInput.TableName] = createTableInput

	var tableNames string
	for _, table := range db.tableNameVsSchemaMap {
		tableNames += table.TableName
		tableNames += ","
	}
	tableNamesLength := len(tableNames)
	tableNames = tableNames[:tableNamesLength-1]

	// PERFORM PUT operation with _catalog key and all table names.
	db.Put(CatalogKey, tableNames)

	// PERFORM PUT operation with _schema:[table_name] key
	db.Put(fmt.Sprintf(SchemaTemplate, createTableInput.TableName), string(
		serialiseCreateTableInput(createTableInput)))

	return nil
}

func (db *DB) InsertIntoTable(query string) error {
	parser := sqlparser.NewParser(query)
	input, err := parser.ParseInsertIntoTable()
	if err != nil {
		return err
	}
	return db.insertIntoTable(*input)
}

// key: table_name:primary_key_value
// value: [value1][size_of_value2][value2][value3]
// value1 and value2 are fixed sized datatype like int and bool while value2 is variable sized
// datatype like string.
// todo: value of primary_key is stored unnecessarily twice (both in key and value)
// todo: lexicographic ordering is currently as per string: 100 will come before 11. this won't
// work for SELECT range queries.
func (db *DB) serialiseInsertIntoTableInput(insertIntoTableInput sqlparser.InsertIntoTable) (
	key string, valueSchemaBuf []byte, err error) {
	tableName := insertIntoTableInput.TableName
	table := db.tableNameVsSchemaMap[tableName]
	primaryKeyValue := ""
	for i, columnValue := range insertIntoTableInput.ColumnValues {
		if i == table.PrimaryKeyColumnPosition {
			primaryKeyValue = columnValue
		}
		switch table.ColumnDetails[i].DataType {
		case sqlparser.Int:
			valueInt, err := strconv.Atoi(columnValue)
			if err != nil {
				return "", nil, err
			}
			valueSchemaBuf = binary.BigEndian.AppendUint32(valueSchemaBuf, uint32(valueInt))
		case sqlparser.String:
			valueSchemaBuf = binary.BigEndian.AppendUint32(valueSchemaBuf, uint32(len(columnValue)))
			valueSchemaBuf = append(valueSchemaBuf, []byte(columnValue)...)
		case sqlparser.Bool:
			// only 0, 1 supported and not true, false
			valueInt, err := strconv.Atoi(columnValue)
			if err != nil {
				return "", nil, err
			}
			if valueInt != 0 && valueInt != 1 {
				valueSchemaBuf = append(valueSchemaBuf, uint8(valueInt))
			}
		}
	}

	return fmt.Sprintf("%s:%s", tableName, primaryKeyValue), valueSchemaBuf, nil
}

func (db *DB) insertIntoTable(insertIntoTableInput sqlparser.InsertIntoTable) error {
	table := db.tableNameVsSchemaMap[insertIntoTableInput.TableName]
	if len(insertIntoTableInput.ColumnValues) != len(table.ColumnDetails) {
		return errors.New("INSERT INTO requires all columns to be present. ")
	}
	key, valueSchemaBuf, err := db.serialiseInsertIntoTableInput(insertIntoTableInput)
	if err != nil {
		return err
	}
	return db.Put(key, string(valueSchemaBuf))
}

func (db *DB) ShowTables() []string {
	tableNames := []string{}
	for _, table := range db.tableNameVsSchemaMap {
		tableNames = append(tableNames, table.TableName)
	}
	return tableNames
}

func (db *DB) ShowCreateTable(tableName string) (*sqlparser.CreateTable, error) {
	for _, table := range db.tableNameVsSchemaMap {
		if table.TableName == tableName {
			return &table, nil
		}
	}
	return nil, fmt.Errorf("table: '%s' not found", tableName)
}

// serialisation strategy: [PK_column_position][columnDataType1][columnNameLength1][columnName1][columnDataType2][columnNameLength2][columnName2]...
func serialiseCreateTableInput(createTableInput sqlparser.CreateTable) []byte {
	serialisedSchema := []byte{}
	serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(createTableInput.PrimaryKeyColumnPosition))

	for _, col := range createTableInput.ColumnDetails {
		serialisedSchema = append(serialisedSchema, byte(col.DataType))
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
		columnMeta.DataType = sqlparser.DataType(dataType)
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
