package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	sqlparser "github.com/golang-db/sql_parser"
)

func isPointedPrimaryKeyQuery(selectFromTableInput sqlparser.SelectFromTable, pkColumnName string) bool {
	return len(selectFromTableInput.QueryConditions) == 1 && selectFromTableInput.QueryConditions[0].ColumnName == pkColumnName &&
		selectFromTableInput.QueryConditions[0].QueryType == sqlparser.Equals
}

func isFullTableScanQuery(selectFromTableInput sqlparser.SelectFromTable) bool {
	return len(selectFromTableInput.QueryConditions) == 0
}

func (db *DB) getRowForPrimaryKey(tableName, primaryKeyId string) ([]string, error) {
	key := fmt.Sprintf("%s:%s", tableName, primaryKeyId)
	value, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	rowValues, err := db.deserializeRowValues(tableName, value)
	if err != nil {
		return nil, err
	}
	return rowValues, nil
}

// go through each secondary index. check if any of the secondary index can independently cover all WHERE conditions
// todo: not solving for AND queries right now. only using secondary index when all columns are covered
// returns nil if no secondary index is applicable
func getSecondaryIndexForQueryIfApplicable(selectFromTableInput sqlparser.SelectFromTable, secondaryIndexes []sqlparser.SecondaryIndex) *sqlparser.SecondaryIndex {
	for _, secondaryIndex := range secondaryIndexes {
		secIdxColsCoveredInInputQuery := 0
		for _, secIdxCol := range secondaryIndex.Columns {
			colFoundInInputQuery := false
			for _, qc := range selectFromTableInput.QueryConditions {
				if qc.ColumnName == secIdxCol {
					colFoundInInputQuery = true
					break
				}
			}
			if colFoundInInputQuery {
				secIdxColsCoveredInInputQuery++
			} else {
				break
			}
		}
		if secIdxColsCoveredInInputQuery == len(secondaryIndex.Columns) {
			return &secondaryIndex
			// 1. prefixScan on the index based key
			// prefixScan
			// 2. Read all the primary keys. Run GET for each, collect and return result
			// we can use this secondary index
			// how to use it?
			// run a prefix scan GET
		}
	}
	return nil
}

// todo: not solving for AND queries right now. only using secondary index when all columns are covered
func (db *DB) getQueryResultFromSecondaryIndexIfApplicable(tableName string, selectFromTableInput sqlparser.SelectFromTable, schema sqlparser.CreateTable) ([][]string, error) {
	secondaryIndex := getSecondaryIndexForQueryIfApplicable(selectFromTableInput, schema.SecondaryIndexes)
	if secondaryIndex == nil {
		return nil, errors.New("query not supported")
	}
	columnValues := []string{}
	for _, condition := range selectFromTableInput.QueryConditions {
		columnValues = append(columnValues, condition.Value)
	}
	prefixKey := getSecondaryIndexKeyOrPrefix(tableName, secondaryIndex.IndexName, columnValues, "")
	primaryKeyIds, err := db.secondaryIndexPrefixScan(prefixKey)
	if err != nil {
		return nil, err
	}
	// Run GET query for each primary key id separately and combine the result of each.
	queryResult := [][]string{}
	for _, pkId := range primaryKeyIds {
		fmt.Printf("pkId555: %v\n", pkId)
		rowValues, err := db.getRowForPrimaryKey(tableName, pkId)
		if err != nil {
			return nil, err
		}
		queryResult = append(queryResult, rowValues)
	}
	return queryResult, nil
}

// todo: without index scan, AND queries support to be added.
func (db *DB) selectFromTable(selectFromTableInput sqlparser.SelectFromTable) ([][]string, error) {
	tableName := selectFromTableInput.TableName
	schema, ok := db.tableNameVsSchemaMap[selectFromTableInput.TableName]
	pkPos := schema.PrimaryKeyColumnPosition
	pkColumnName := ""
	if !ok {
		return nil, fmt.Errorf("table with name %q not found", tableName)
	}
	// improve performance by storing primary key column name also apart from primary key column position?
	for i, col := range schema.ColumnDetails {
		if i == pkPos {
			pkColumnName = col.ColumnName
		}
	}
	if pkColumnName == "" {
		return nil, errors.New("primary key column position is incorrect")
	}
	// todo: not solving for returning limited columns right now
	if isPointedPrimaryKeyQuery(selectFromTableInput, pkColumnName) {
		rowValues, err := db.getRowForPrimaryKey(tableName, selectFromTableInput.QueryConditions[0].Value)
		if err != nil {
			return nil, err
		}
		return [][]string{rowValues}, nil
	}
	if isFullTableScanQuery(selectFromTableInput) {
		return db.fullTableScan(tableName)
	}

	// todo: not solving for AND queries right now. only using secondary index when all columns are covered
	return db.getQueryResultFromSecondaryIndexIfApplicable(tableName, selectFromTableInput, schema)
}

// value: [value1][size_of_value2][value2][value3]
func (db *DB) deserializeRowValues(tableName, value string) ([]string, error) {
	// read byte inputs
	schema := db.tableNameVsSchemaMap[tableName]
	fmt.Printf("value4444: %v\n", value)
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
	memTableMap := db.memTable.PrefixScan(key)
	ssTableMap, err := db.ssTable.PrefixScan(key)
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

// returns an array of primary key IDs which satisfy the index.
func (db *DB) secondaryIndexPrefixScan(prefixKey string) ([]string, error) {
	memTableMap := db.memTable.PrefixScan(prefixKey)
	ssTableMap, err := db.ssTable.PrefixScan(prefixKey)
	if err != nil {
		return nil, err
	}

	primaryKeyIds := []string{}
	for key, _ := range ssTableMap {
		fmt.Printf("key777: %v\n", key)
		keyElements := strings.Split(key, ":")
		primaryKeyIds = append(primaryKeyIds, keyElements[len(keyElements)-1])
	}
	for key, _ := range memTableMap {
		fmt.Printf("key888: %v\n", key)
		keyElements := strings.Split(key, ":")
		primaryKeyIds = append(primaryKeyIds, keyElements[len(keyElements)-1])
	}
	return primaryKeyIds, nil
}
