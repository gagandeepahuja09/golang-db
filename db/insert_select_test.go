package db

import (
	"fmt"
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/stretchr/testify/assert"
)

func TestInsertSelectTest(t *testing.T) {
	db, cleanupFunc, err := newDBForTest()
	defer cleanupFunc()
	assert.NoError(t, err)

	err = db.CreateTable("CREATE TABLE student (age INT, id STRING, isActive BOOL, PRIMARY KEY (id));")
	assert.NoError(t, err)

	err = db.InsertIntoTable("INSERT INTO student VALUES (id1234, 15, 1)")
	assert.NoError(t, err)

	res, err := db.selectFromTable(sqlparser.SelectFromTable{
		TableName:       "student",
		ColumnsRequired: []string{"*"},
		QueryConditions: []sqlparser.QueryCondition{{
			ColumnName: "id",
			QueryType:  "=",
			Value:      "id1234",
		}},
	})
	assert.NoError(t, err)
	fmt.Printf("res11111: %+v\n", res)
}
