package db

import (
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

	err = db.InsertIntoTable("INSERT INTO student VALUES (15, id1234, 1)")

	// todo: tests and code handling cases where data is wrongly provided. eg string provided for INT.
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
	assert.Len(t, res, 1)

	assert.Equal(t, []string{"15", "id1234", "1"}, res[0])
}
