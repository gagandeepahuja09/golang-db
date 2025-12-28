package main

import (
	"fmt"
	"testing"

	"github.com/golang-db/memtable"
	"github.com/stretchr/testify/assert"
)

func buildMemtableTestData() memtable.Memtable {
	memtable := memtable.NewMemtable()

	for i := 0; memtable.GetSize() < 2000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		memtable.Put(key, value)
	}

	return memtable
}

func TestSsTableIndex(t *testing.T) {
	db, err := buildMemtableFromWal()
	assert.NoError(t, err)
	db.ssTableMaxBlockLength = 200
	db.memTable = buildMemtableTestData()
	err = db.flushMemtableToSsTable()
	fmt.Printf("err111: %v\n", err)
	assert.NoError(t, err)

	fmt.Printf("db.ssTableFiles: %+v\n", db.ssTableFiles)

	fmt.Printf("db.ssTableIndexes: %+v\n", db.ssTableIndexes)

	db.cmdGet([]string{"GET", "key_101"})

	db.cmdGet([]string{"GET", "key_1010"})

	db.cmdGet([]string{"GET", "key_10100"})

	db.cmdGet([]string{"GET", "key_121"})

	// 3. delete the sstable file
}
