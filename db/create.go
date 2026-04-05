package db

import (
	"encoding/binary"
	"errors"
	"fmt"

	sqlparser "github.com/golang-db/sql_parser"
)

func (db *DB) CreateTable(query string) error {
	parser := sqlparser.NewParser(query)
	input, err := parser.ParseCreateTable()
	if err != nil {
		return err
	}
	return db.createTable(*input)
}

func (db *DB) createTable(createTableInput sqlparser.CreateTable) error {
	// todo: checking for table name already exists
	// todo: since we are doing multiple different Put operations, createTable is not actually atomic

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

	tableName := createTableInput.TableName

	// PERFORM PUT operation with _schema:[table_name] key
	db.Put(fmt.Sprintf(SchemaTemplate, tableName), string(
		serialiseCreateTableInput(createTableInput)))

	// update the secondary index catalog
	secondaryIndexCatalogBuf, err := db.serialiseSecondaryIndexCatalog(tableName, createTableInput.SecondaryIndexes)
	if err != nil {
		return err
	}
	db.Put(fmt.Sprintf(SecondaryIndexesCatalogKeyTemplate, tableName), string(secondaryIndexCatalogBuf))

	return nil
}

// serialisation: [number_of_indexes][idx_1_name_len][idx_1_name]
// [number_of_columns_in_idx_1][col_1_idx_1][col2_idx_2]...
// column idx is as per the order stored in _schema:[table_name].
// during creation, we don't need to do any GET to check the status of the secondary indexes key
// as no index exists before CREATE TABLE.
// but during CREATE INDEX, we need to do GET first.
func (db *DB) serialiseSecondaryIndexCatalog(tableName string, secondaryIndexes []sqlparser.SecondaryIndex) ([]byte, error) {
	serialisedSchema := []byte{}

	// 1. append no. of indexes
	serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(len(secondaryIndexes)))
	for _, secondaryIndex := range secondaryIndexes {
		// 2. append index name length and then index name for each index
		serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(len(secondaryIndex.IndexName)))
		serialisedSchema = append(serialisedSchema, []byte(secondaryIndex.IndexName)...)

		// 3. append columns count
		serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(len(secondaryIndex.Columns)))

		// 4. append list of column indexes (positions). column position is as per the order stored in table catalog.
		tableColumns := db.tableNameVsSchemaMap[tableName].ColumnDetails
		for _, col := range secondaryIndex.Columns {
			colIdx := -1
			for i, tableCol := range tableColumns {
				if tableCol.ColumnName == col {
					colIdx = i
				}
			}
			if colIdx == -1 {
				return nil, fmt.Errorf("column: '%s' not found", col)
			}
			// 4.1. append column index (position) for the the current column in the column secondary index.
			serialisedSchema = binary.BigEndian.AppendUint32(serialisedSchema, uint32(colIdx))
		}
	}
	return serialisedSchema, nil
}

// deserialise: [number_of_indexes][idx_1_name_len][idx_1_name]
// [number_of_columns_in_idx_1][col_1_idx_1][col2_idx_2]...
func (db *DB) deserialiseSecondaryIndexCatalog(tableName string, buf []byte, tableColumns []sqlparser.Column) ([]sqlparser.SecondaryIndex, error) {
	i := 0
	// 1. read number of indexes
	if i+4 > len(buf) {
		return nil, errors.New("unexpected error while reading number of indexes")
	}
	secondaryIndexes := []sqlparser.SecondaryIndex{}
	numIndexes := binary.BigEndian.Uint32(buf[i : i+4])
	i += 4
	for j := 0; j < int(numIndexes); j++ {
		// 2. read index name length for each index
		if i+4 > len(buf) {
			return nil, errors.New("unexpected error while reading index name length")
		}
		indexNameLen := binary.BigEndian.Uint32(buf[i : i+4])
		i += 4
		// 3. read index name for each index
		if i+int(indexNameLen) > len(buf) {
			return nil, errors.New("unexpected error while reading index name")
		}
		indexName := string(buf[i : i+int(indexNameLen)])
		i += int(indexNameLen)
		// 4. read number of columns for each index
		if i+4 > len(buf) {
			return nil, errors.New("unexpected error while reading number of columns in index")
		}
		columns := []string{}
		numColumns := binary.BigEndian.Uint32(buf[i : i+4])
		i += 4
		// 5. read column position for each column for each index
		for k := 0; k < int(numColumns); k++ {
			if i+4 > len(buf) {
				return nil, errors.New("unexpected error while reading column index position")
			}
			colPosition := int(binary.BigEndian.Uint32(buf[i : i+4]))
			i += 4
			// 6. get the column name as per column position
			if colPosition < 0 || colPosition >= len(tableColumns) {
				// col position must be within 0 and 0. Got -1
				return nil, fmt.Errorf("col position must be within 0 and %d. Got %d", len(tableColumns)-1, colPosition)
			}
			columns = append(columns, tableColumns[colPosition].ColumnName)
		}
		secondaryIndexes = append(secondaryIndexes, sqlparser.SecondaryIndex{
			IndexName: indexName,
			Columns:   columns,
		})
	}
	return secondaryIndexes, nil
}

// serialisation strategy: [PK_column_position][columnDataType1][columnNameLength1][columnName1][columnDataType2][columnNameLength2][columnName2]...
// secondary index serialisation is covered separately even though it is part of the same CREATE TABLE input.
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
