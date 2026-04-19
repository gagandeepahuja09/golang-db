//go:build secondaryindex
// +build secondaryindex

package db

import (
	"fmt"
	"math/rand/v2"
	"os"
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/golang-db/sstable"
	"github.com/stretchr/testify/assert"
)

func newDBForTestForDir(dirNum int) (*DB, func(), error) {
	walFilePath := fmt.Sprintf("temp_wal_%d.log", dirNum)
	dataFilesDirectory := fmt.Sprintf("temp_%d", dirNum)
	dbInstance, err := NewDB(Config{
		SsTableConfig: sstable.Config{
			DataFilesDirectory: dataFilesDirectory,
		},
		WalFilePath: walFilePath,
	})
	cleanupFunc := func() {
		defer os.RemoveAll(dataFilesDirectory)
		defer os.Remove(walFilePath)
	}
	return dbInstance, cleanupFunc, err
}

// returns all possible unique values present in column c2
func (db *DB) insertTestData(t assert.TestingT, lowCardinality bool, rowsCount int) {
	for i := 0; i < rowsCount; i++ {
		c2Value := rand.IntN(10)
		if !lowCardinality {
			c2Value = rand.IntN(rowsCount / 2)
		}
		err := db.insertIntoTable(sqlparser.InsertIntoTable{
			TableName: "t1",
			ColumnValues: []string{
				fmt.Sprintf("pk_%d", i),
				fmt.Sprintf("%d", c2Value),
				fmt.Sprintf("%d", i%5),
				fmt.Sprintf("%d", i%2),
			},
		})
		assert.NoError(t, err)
	}
}

func benchmarkSecondaryIndexPerformance(b *testing.B, useIndex bool, lowCardinality bool, rowsCount, dirNum int) {
	dbInstance, cleanupFunc, err := newDBForTestForDir(dirNum)
	assert.NoError(b, err)
	defer cleanupFunc()

	_, _, err = createTestTable(dbInstance, useIndex)
	assert.NoError(b, err)

	dbInstance.insertTestData(b, lowCardinality, rowsCount)
	for b.Loop() {
		actualRowsCount := 0
		valuesRange := 10
		if !lowCardinality {
			valuesRange = rowsCount / 2
		}
		for i := 0; i < valuesRange; i++ {
			queryRes, err := dbInstance.selectFromTable(sqlparser.SelectFromTable{
				TableName: "t1",
				QueryConditions: []sqlparser.QueryCondition{{
					ColumnName: "c2",
					QueryType:  sqlparser.Equals,
					Value:      fmt.Sprintf("%d", i),
				}},
			})
			assert.NoError(b, err)
			actualRowsCount += len(queryRes)
		}
		assert.Equal(b, rowsCount, actualRowsCount)
	}
}

func BenchmarkSecondaryIndex_NotUsedLowCardinality100Rows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, false, true, 100, 0)
}

func BenchmarkSecondaryIndex_UsedLowCardinality100Rows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, true, true, 100, 1)
}

func BenchmarkSecondaryIndex_NotUsedHighCardinality100Rows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, false, false, 100, 2)
}

func BenchmarkSecondaryIndex_UsedHighCardinality100Rows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, true, false, 100, 3)
}

func BenchmarkSecondaryIndex_NotUsedLowCardinality10kRows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, false, true, 10000, 4)
}

func BenchmarkSecondaryIndex_UsedLowCardinality10kRows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, true, true, 10000, 5)
}

func BenchmarkSecondaryIndex_NotUsedHighCardinality10kRows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, false, false, 10000, 6)
}

func BenchmarkSecondaryIndex_UsedHighCardinality10kRows(b *testing.B) {
	benchmarkSecondaryIndexPerformance(b, true, false, 10000, 7)
}
