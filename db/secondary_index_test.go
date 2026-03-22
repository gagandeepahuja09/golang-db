package db

import (
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/stretchr/testify/assert"
)

func TestSecondaryIndexCreation(t *testing.T) {
	// 1. call create table with secondaryIndex params
	// 2. check the db.tableNameVsSchemaMap
	dbInstance, cleanupFunc, err := newDBForTest()
	defer cleanupFunc()

	colDetails := []sqlparser.Column{
		{
			ColumnName: "c1",
			DataType:   sqlparser.String,
		},
		{
			ColumnName: "c2",
			DataType:   sqlparser.String,
		},
		{
			ColumnName: "c3",
			DataType:   sqlparser.Int,
		},
		{
			ColumnName: "c4",
			DataType:   sqlparser.Bool,
		},
	}

	secondaryIndexes := []sqlparser.SecondaryIndex{
		{
			Columns:   []string{"c1"},
			IndexName: "idxc1",
		},
		{
			Columns:   []string{"c2"},
			IndexName: "idxc2",
		},
		{
			Columns:   []string{"c3", "c4"},
			IndexName: "idxc3c4",
		},
		// todo: try with incorrect column name as well
	}

	err = dbInstance.createTable(sqlparser.CreateTable{
		TableName:        "t1",
		ColumnDetails:    colDetails,
		SecondaryIndexes: secondaryIndexes,
	})
	assert.NoError(t, err)
	t1Schema := dbInstance.tableNameVsSchemaMap["t1"]
	assert.Equal(t, secondaryIndexes, t1Schema.SecondaryIndexes)
	assert.Equal(t, colDetails, t1Schema.ColumnDetails)

	// spin up a new instance with exactly the same config (wal and ss-table path directories)
	dbInstance2, _, err := newDBForTest()
	assert.NoError(t, err)
	t1Schema = dbInstance2.tableNameVsSchemaMap["t1"]
	assert.Equal(t, secondaryIndexes, t1Schema.SecondaryIndexes)
	assert.Equal(t, colDetails, t1Schema.ColumnDetails)
}
