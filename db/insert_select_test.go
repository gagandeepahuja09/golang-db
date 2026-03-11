package db

import (
	"fmt"
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/stretchr/testify/assert"
)

func TestInsertSelectTestWithPrimaryKey(t *testing.T) {
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

func TestSelectFullTableScan(t *testing.T) {
	// create 4 different tables.
	// insert few rows in each. do a full table scan of each of them separately.
	db, cleanupFunc, err := newDBForTest()
	defer cleanupFunc()
	assert.NoError(t, err)

	err = db.CreateTable("CREATE TABLE customer (age INT, id STRING, isActive BOOL, PRIMARY KEY (id));")
	assert.NoError(t, err)
	err = db.CreateTable("CREATE TABLE employee (age INT, id STRING, isActive BOOL, PRIMARY KEY (id));")
	assert.NoError(t, err)
	err = db.CreateTable("CREATE TABLE student (age INT, id STRING, isActive BOOL, PRIMARY KEY (id));")
	assert.NoError(t, err)
	err = db.CreateTable("CREATE TABLE teacher (age INT, id STRING, isActive BOOL, PRIMARY KEY (id));")
	assert.NoError(t, err)

	// insert few rows in student table
	for i := 0; i < 50; i++ {
		err = db.InsertIntoTable(fmt.Sprintf("INSERT INTO student VALUES (%d, id%d, 1)", i, i))
	}

	for i := 75; i < 120; i++ {
		err = db.InsertIntoTable(fmt.Sprintf("INSERT INTO teacher VALUES (%d, id%d, 1)", i, i))
	}

	val, ok := db.memTable.Get("student:id0")
	fmt.Printf("val111: %v %v\n", val, ok)

	_, err = db.selectFromTable(sqlparser.SelectFromTable{
		TableName:       "student",
		ColumnsRequired: []string{"*"},
	})

	_, err = db.selectFromTable(sqlparser.SelectFromTable{
		TableName:       "teacher",
		ColumnsRequired: []string{"*"},
	})
	assert.NoError(t, err)
}
