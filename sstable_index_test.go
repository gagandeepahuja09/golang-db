package main

import (
	"fmt"
	"testing"

	"github.com/golang-db/db"
	"github.com/stretchr/testify/assert"
)

func buildMemtableTestData(db *db.DB) {
	for i := 0; i < 300; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}
}

func TestSsTableIndex(t *testing.T) {
	db, err := db.NewDB("")
	assert.NoError(t, err)
	buildMemtableTestData(db)

	value, err := db.Get("key_101")
	assert.NoError(t, err)
	assert.Equal(t, "value_101", value)

	value, err = db.Get("key_1010")
	assert.Equal(t, "", value)

	value, err = db.Get("key_10100")
	assert.Equal(t, "", value)

	value, err = db.Get("GET")
	assert.Equal(t, "", value)

	for i := 0; i <= 151; i++ {
		value, err = db.Get(fmt.Sprintf("key_%d", i))
		assert.Equal(t, fmt.Sprintf("value_%d", i), value)
	}

	for i := 600; i <= 625; i++ {
		value, err = db.Get(fmt.Sprintf("key_%d", i))
		assert.Equal(t, "", value)
	}

	// 3. delete the sstable file
}
