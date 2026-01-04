package main

import (
	"fmt"
	"testing"

	"github.com/golang-db/db"
	"github.com/golang-db/sstable"
)

func buildTestDataWithPutOnRepeatedKeys(db *db.DB) {
	for i := 0; i < 300; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}

	for i := 0; i < 300; i += 2 {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}

	for i := 0; i < 300; i += 5 {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}
}

func BenchmarkSSTableWithCompaction(b *testing.B) {
	db, _ := db.NewDB(db.Config{
		SsTableConfig: sstable.Config{
			SkipIndex: false,
		},
	})
	buildTestDataWithPutOnRepeatedKeys(db)
	for b.Loop() {
		testGetInBulk(db)
	}
}

func BenchmarkSSTableBinarySearchMixedWorkload(b *testing.B) {
	db, _ := db.NewDB(db.Config{
		SsTableConfig: sstable.Config{
			SkipIndex: false,
		},
	})
	buildTestData(db)
	for b.Loop() {
		testGetInBulk(db)
	}
}

func BenchmarkSSTableLinearSearchMixedWorkload(b *testing.B) {
	db, _ := db.NewDB(db.Config{
		SsTableConfig: sstable.Config{
			SkipIndex: true,
		},
	})
	buildTestData(db)
	for b.Loop() {
		testGetInBulk(db)
	}
}

func testGetInBulk(db *db.DB) {
	for i := 0; i < 300; i++ {
		db.Get(fmt.Sprintf("key_%d", i))
	}

	for i := 302; i <= 400; i++ {
		db.Get(fmt.Sprintf("key_%d", i))
	}
}
