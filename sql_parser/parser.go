package sqlparser

import (
	"errors"
	"fmt"
)

// not handling \n for now.
const (
	ExpectedSyntaxCmdCreateTable = "expected more arguments in CREATE TABLE command. Expected syntax: CREATE TABLE name_of_table ( c1 int, c2 bool, c3 string )"
)

type CreateTable struct {
	TableName                string
	ColumnDetails            []Column
	PrimaryKeyColumnPosition int
}

type Column struct {
	ColumnName string
	DataType   uint8
}

// parses create table query to extract all of the required metadata.
//
//	input: first 2 words are removed: CREATE TABLE and the rest of the query.
func ParseCreateTable(args []string) (*CreateTable, error) {
	if len(args) < 2 {
		return nil, errors.New(ExpectedSyntaxCmdCreateTable)
	}
	tableName := args[0]
	schemaSlice := args[1:]

	fmt.Printf("tableName: %v\n", tableName)
	fmt.Printf("schemaSlice: %v\n", schemaSlice)

	if len(schemaSlice) < 4 ||
		(schemaSlice[0] != "(" && schemaSlice[len(schemaSlice)-1] != ")") {
		return nil, errors.New(ExpectedSyntaxCmdCreateTable)
	}

	columnMeta := Column{}
	// example: CREATE TABLE payment ( id int )
	for i, schemaAttr := range schemaSlice {
		if i == 0 || i == len(schemaSlice)-1 {
			continue
		}
		if i%2 == 1 {
			columnMeta.ColumnName = schemaAttr
		} else {
			// columnMeta.DataType = schemaAttr
		}
	}
	return nil, nil

	// if len(schemaSlice) == 4 {
	// 	schemaArg := schemaSlice[0]
	// 	if schemaArg[len(schemaArg)-1] != ')' {
	// 		return nil, errors.New(ExpectedSyntaxCmdCreateTable)
	// 	}
	// 	return &CreateTableInput{
	// 		tableName: tableName,
	// 		ColumnDetails: []Column{
	// 			{
	// 				columnName: schemaSlice[1],
	// 				dataType:   schemaSlice[2],
	// 			},
	// 		},
	// 	}, nil
	// }

	// return nil
}
