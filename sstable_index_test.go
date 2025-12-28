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
	assert.NoError(t, err)

	value, err := db.cmdGet([]string{"GET", "key_101"})
	assert.NoError(t, err)
	assert.Equal(t, "value_101", value)

	_, err = db.cmdGet([]string{"GET", "key_1010"})
	assert.Equal(t, "No value found for GET key_1010", err.Error())

	_, err = db.cmdGet([]string{"GET", "key_10100"})
	assert.Equal(t, "No value found for GET key_10100", err.Error())

	value, err = db.cmdGet([]string{"GET", "key_121"})
	assert.Equal(t, "value_121", value)

	// 3. delete the sstable file
}
