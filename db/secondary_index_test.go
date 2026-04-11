package db

import (
	"fmt"
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/stretchr/testify/assert"
)

func TestSecondaryIndexBasedQueries(t *testing.T) {
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

	for i := 0; i < 10; i++ {
		// todo: we should also support insert with actual data types and not just string
		err = dbInstance2.insertIntoTable(sqlparser.InsertIntoTable{
			TableName: "t1",
			ColumnValues: []string{
				fmt.Sprintf("val1_%d", i),
				fmt.Sprintf("val2_%d", i),
				fmt.Sprintf("%d", i%5),
				fmt.Sprintf("%d", i%2),
			},
		})
		assert.NoError(t, err)
	}

	// part 1: test the result for c1, c2 indexes separately
	for i := 0; i < 20; i++ {
		columnName := "c1"
		valueTemplate := "val1_%d"
		arrIdx := i
		if i >= 10 {
			arrIdx = i - 10
			columnName = "c2"
			valueTemplate = "val2_%d"
		}
		queryRes, err := dbInstance2.selectFromTable(sqlparser.SelectFromTable{
			TableName: "t1",
			QueryConditions: []sqlparser.QueryCondition{
				{
					ColumnName: columnName,
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf(valueTemplate, arrIdx),
				},
			},
		})
		assert.NoError(t, err)
		assert.Len(t, queryRes, 1)
		assert.Equal(t, []string{
			fmt.Sprintf("val1_%d", arrIdx),
			fmt.Sprintf("val2_%d", arrIdx),
			fmt.Sprintf("%d", arrIdx%5), fmt.Sprintf("%d", arrIdx%2)}, queryRes[0])
	}

	// part 2: test the result for c3 and c4 combined composite index
	for i := 0; i < 10; i++ {
		queryRes, err := dbInstance2.selectFromTable(sqlparser.SelectFromTable{
			TableName: "t1",
			QueryConditions: []sqlparser.QueryCondition{
				{
					ColumnName: "c3",
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf("%d", i%5),
				},
				{
					ColumnName: "c4",
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf("%d", i%2),
				},
			},
		})
		assert.NoError(t, err)
		assert.Len(t, queryRes, 1)
		assert.Equal(t, []string{
			fmt.Sprintf("val1_%d", i),
			fmt.Sprintf("val2_%d", i),
			fmt.Sprintf("%d", i%5), fmt.Sprintf("%d", i%2)}, queryRes[0])
	}

	// part 3: test the result for c3 which is a prefix of c3, c4 composite index
	for i := 0; i < 5; i++ {
		queryRes, err := dbInstance2.selectFromTable(sqlparser.SelectFromTable{
			TableName: "t1",
			QueryConditions: []sqlparser.QueryCondition{
				{
					ColumnName: "c3",
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf("%d", i),
				},
			},
		})
		assert.NoError(t, err)
		assert.ElementsMatch(t, [][]string{
			{fmt.Sprintf("val1_%d", i), fmt.Sprintf("val2_%d", i), fmt.Sprintf("%d", i), fmt.Sprintf("%d", i%2)},
			{fmt.Sprintf("val1_%d", i+5), fmt.Sprintf("val2_%d", i+5), fmt.Sprintf("%d", i), fmt.Sprintf("%d", (i+5)%2)}}, queryRes)
	}

	// part 4: test the result for c4: no index exists
	for i := 0; i < 2; i++ {
		_, err := dbInstance2.selectFromTable(sqlparser.SelectFromTable{
			TableName: "t1",
			QueryConditions: []sqlparser.QueryCondition{
				{
					ColumnName: "c4",
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf("%d", i),
				},
			},
		})
		assert.Equal(t, "query not supported", err.Error())
	}

	// todo: benchmarking for performance: with and without indexes.

	// todo: as of now partial indexes are not supported.
	// Check the result
}
