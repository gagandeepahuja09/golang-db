package main

import (
	"fmt"
	"testing"

	"github.com/golang-db/memtable"
	"github.com/stretchr/testify/assert"
)

func buildMemtableTestData() *memtable.Memtable {
	memtable := memtable.NewMemtable()

	for i := 0; memtable.GetSize() < 2000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		memtable.Put(key, value)
	}

	return &memtable
}

func TestSsTableIndex(t *testing.T) {
	db, err := newDB("")
	assert.NoError(t, err)
	db.memTable = buildMemtableTestData()
	err = db.createSsTableAndClearWalAndMemTable()
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

	for i := 0; i <= 101; i++ {
		value, err = db.cmdGet([]string{"GET", fmt.Sprintf("key_%d", i)})
		assert.Equal(t, fmt.Sprintf("value_%d", i), value)
	}

	for i := 600; i <= 700; i++ {
		_, err = db.cmdGet([]string{"GET", fmt.Sprintf("key_%d", i)})
		assert.Equal(t, fmt.Sprintf("No value found for GET key_%d", i), err.Error())
	}

	// 3. delete the sstable file
}
