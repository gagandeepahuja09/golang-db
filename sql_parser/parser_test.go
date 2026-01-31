package sqlparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCreateTable(t *testing.T) {
	testCases := []struct {
		name                string
		inputQuery          string
		expectedCreateTable CreateTable
		expectedError       string
	}{
		{
			name:                "Create only",
			inputQuery:          "CREATE",
			expectedCreateTable: CreateTable{},
			expectedError:       "syntax error: expected KEYWORD \"TABLE\", got EOF \"\"",
		},
		{
			name:                "Create database",
			inputQuery:          "CREATE DATABASE",
			expectedCreateTable: CreateTable{},
			expectedError:       "syntax error: expected KEYWORD \"TABLE\", got IDENTIFIER \"DATABASE\"",
		},
		{
			name:                "Create table only",
			inputQuery:          "CREATE TABLE",
			expectedCreateTable: CreateTable{},
			expectedError:       "syntax error: expected IDENTIFIER \"\", got EOF \"\"",
		},
		{
			name:                "Create table with name but no brackets",
			inputQuery:          "CREATE TABLE abc",
			expectedCreateTable: CreateTable{},
			expectedError:       "syntax error: expected SYMBOL \"(\", got EOF \"\"",
		},
		{
			name:                "Create table with name but no brackets",
			inputQuery:          "CREATE TABLE abc ()",
			expectedCreateTable: CreateTable{},
			expectedError:       "expected atleast one column detail, found none",
		},
		{
			name:                "Create table with no data type",
			inputQuery:          "CREATE TABLE abc (something)",
			expectedCreateTable: CreateTable{},
			expectedError:       "syntax error: expected IDENTIFIER \"\", got SYMBOL \")\"",
		},
		{
			name:                "Create table with no data type",
			inputQuery:          "CREATE TABLE abc (something sometype)",
			expectedCreateTable: CreateTable{},
			expectedError:       "data type 'sometype' not found. expected one of INT, STRING, BOOL",
		},
		{
			name:       "Create table with int datatype",
			inputQuery: "CREATE TABLE abc (something INT)",
			expectedCreateTable: CreateTable{
				TableName: "abc",
				ColumnDetails: []Column{{
					ColumnName: "something",
					DataType:   Int,
				}},
			},
			expectedError: "",
		},
		{
			name:       "Create table with multiple datatypes",
			inputQuery: "CREATE TABLE abc (someNum INT, someStr STRING, someBool BOOL)",
			expectedCreateTable: CreateTable{
				TableName: "abc",
				ColumnDetails: []Column{{
					ColumnName: "someNum",
					DataType:   Int,
				},
					{
						ColumnName: "someStr",
						DataType:   String,
					},
					{
						ColumnName: "someBool",
						DataType:   Bool,
					}},
			},
			expectedError: "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.inputQuery)
			input, err := parser.ParseCreateTable()
			if err != nil {
				assert.Equal(t, tt.expectedError, err.Error())
			} else {
				assert.Equal(t, tt.expectedCreateTable, *input)
			}
		})
	}
}
