package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	sqlparser "github.com/golang-db/sql_parser"
)

func (db *DB) selectFromTable(selectFromTableInput sqlparser.SelectFromTable) ([][]string, error) {
	// what should be the output structure?
	// array of rows ==> [][]string?
	// only support primary key based queries for now
	// we need to know the primary key column
	tableName := selectFromTableInput.TableName
	schema, ok := db.tableNameVsSchemaMap[selectFromTableInput.TableName]
	pkPos := schema.PrimaryKeyColumnPosition
	pkColumnName := ""
	if !ok {
		return nil, fmt.Errorf("table with name %q not found", tableName)
	}
	// improve performance by storing primary key column name also?
	for i, col := range schema.ColumnDetails {
		if i == pkPos {
			pkColumnName = col.ColumnName
		}
	}
	if pkColumnName == "" {
		// throw error?
		return nil, errors.New("primary key column position is incorrect")
	}
	// only pointed primary key query supported
	if len(selectFromTableInput.QueryConditions) == 1 && selectFromTableInput.QueryConditions[0].ColumnName == pkColumnName &&
		selectFromTableInput.QueryConditions[0].QueryType == sqlparser.Equals {
		// todo: check for columns required
		key := fmt.Sprintf("%s:%s", tableName, selectFromTableInput.QueryConditions[0].Value)
		value, err := db.Get(key)
		if err != nil {
			return nil, err
		}
		rowValues, err := db.deserializeRowValues(tableName, value)
		if err != nil {
			return nil, err
		}
		return [][]string{rowValues}, nil
	}
	// full-table scan
	// todo: not solving for returning limited columns right now
	if len(selectFromTableInput.QueryConditions) == 0 {
		return db.fullTableScan(tableName)
	}
	return nil, errors.New("query not supported")
}

// value: [value1][size_of_value2][value2][value3]
func (db *DB) deserializeRowValues(tableName, value string) ([]string, error) {
	// read byte inputs
	schema := db.tableNameVsSchemaMap[tableName]
	valueBuf := []byte(value)
	i := 0
	rowValues := []string{}
	for _, col := range schema.ColumnDetails {
		switch col.DataType {
		case sqlparser.Int:
			val := strconv.FormatUint(uint64(binary.BigEndian.Uint32(valueBuf[i:i+4])), 10)
			rowValues = append(rowValues, val)
			i += 4
		case sqlparser.String:
			len := int(binary.BigEndian.Uint32(valueBuf[i : i+4]))
			i += 4
			val := string(valueBuf[i : i+len])
			rowValues = append(rowValues, val)
			i += len

		case sqlparser.Bool:
			val := strconv.FormatUint(uint64(valueBuf[i]), 2)
			rowValues = append(rowValues, val)
			i++
		}
	}
	return rowValues, nil
}

func (db *DB) fullTableScan(tableName string) ([][]string, error) {
	key := fmt.Sprintf("%s:", tableName)
	memTableMap := db.memTable.FullTableScan(key)

	ssTableMap, err := db.ssTable.FullTableScan(key)
	if err != nil {
		return nil, err
	}

	scanOutput := [][]string{}
	for _, value := range memTableMap {
		values, err := db.deserializeRowValues(tableName, value)
		if err != nil {
			return nil, err
		}
		scanOutput = append(scanOutput, values)
	}

	for key, value := range ssTableMap {
		if _, ok := memTableMap[key]; ok {
			// memtable would have the most up-to date value. no need to rely on sstable value when key is found in memtable
			continue
		}
		values, err := db.deserializeRowValues(tableName, value)
		if err != nil {
			return nil, err
		}
		scanOutput = append(scanOutput, values)
	}

	return scanOutput, nil
}
