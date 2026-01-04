package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-db/db"
	"github.com/stretchr/testify/assert"
)

func buildTestData(db *db.DB) {
	for i := 0; i < 300; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}

	time.Sleep(100 * time.Millisecond)
	for i := 300; i <= 377; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		db.Put(key, value)
	}
}

// we take large enough keys so that the flow can be tested for flushing memtable to sstable
func TestGetAndPutInBulk(t *testing.T) {
	db, err := db.NewDB(db.Config{})
	assert.NoError(t, err)
	buildTestData(db)

	// let the old files delete
	time.Sleep(4 * time.Second)

	value, err := db.Get("key_101")
	assert.NoError(t, err)
	assert.Equal(t, "value_101", value)

	value, err = db.Get("key_1010")
	assert.Equal(t, "", value)

	value, err = db.Get("key_10100")
	assert.Equal(t, "", value)

	value, err = db.Get("GET")
	assert.Equal(t, "", value)

	for i := 250; i <= 377; i++ {
		value, err = db.Get(fmt.Sprintf("key_%d", i))
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("value_%d", i), value)
	}

	for i := 600; i <= 625; i++ {
		value, err = db.Get(fmt.Sprintf("key_%d", i))
		assert.Equal(t, "", value)
	}

	// 3. delete the sstable file
}
