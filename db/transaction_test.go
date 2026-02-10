package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// todo: handle cleanup from persistent store in this test

// same transaction get -> put -> get should read from buffered writes
func TestSameTransactionPutAndGet(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_101"
	expectedValue := "value_101"

	val, err := txn.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, "", val)

	err = txn.Put(testKey, expectedValue)
	assert.Nil(t, err)

	val, err = txn.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, val)
}

// t1 acquires read lock first. t1 upgrades to write lock. t2 will not be able to acquire read lock after that.
func TestDifferentTransactionPutAndGetWithPutAcquiringLock(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_101"
	expectedValue := "value_101"

	_, err = txn.Get(testKey)
	assert.Nil(t, err)

	err = txn.Put(testKey, expectedValue)
	assert.Nil(t, err)

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	_, err = txn2.Get(testKey)
	assert.Equal(t, "cannot acquire read lock as write lock acquired by transaction '1'", err.Error())
}

// t1 acquires read lock first, hence t2 will not be able to acquire write lock
func TestDifferentTransactionPutAndGetWithGetAcquiringLock(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_101"

	_, err = txn.Get(testKey)
	assert.Nil(t, err)

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	err = txn2.Put(testKey, "value")
	assert.Equal(t, "cannot acquire write lock as read lock acquired by one or more transactions", err.Error())
}

// multiple transactions should be able to acquire read lock for a key at the same time
func TestMultipleOpenTransactionsGetSameKey(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	testKey := "key_101"
	expectedValue := "value_101"
	err = dbInstance.Put(testKey, expectedValue)
	assert.Nil(t, err)

	for i := 0; i < 10; i++ {
		txn, err := dbInstance.Begin()
		assert.Nil(t, err)

		val, err := txn.Get(testKey)
		assert.Nil(t, err)
		assert.Equal(t, expectedValue, val)
	}
}

// if t1 and t2 are calling put on different keys, it should not lead to any conflict
func TestDifferentOpenTransactionPutWithDifferentKeys(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_101"
	expectedValue := "value_101"

	err = txn.Put(testKey, expectedValue)
	assert.Nil(t, err)

	val, err := txn.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, val)

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	err = txn2.Put("key_102", expectedValue)
	assert.Nil(t, err)
}

// if t1 and t2 are calling put on the same key, t2 (which tried acquiring later)
// should get an error
func TestDifferentOpenTransactionPutWithSameKey(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_101"
	expectedValue := "value_101"

	err = txn.Put(testKey, expectedValue)
	assert.Nil(t, err)

	val, err := txn.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, val)

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	err = txn2.Put(testKey, expectedValue)
	assert.Equal(t, "cannot acquire write lock as write lock acquired by transaction '1'", err.Error())
}

// t1 acquires write lock, t2 will not be able to acquire read lock
// t2 rollsback, t2 will be able to read and reads an old value
func TestRollbackReleasesLockAndCleansUpBufferedWrite(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	testKey := "key_105"
	expectedValue := "value_105"

	err = txn.Put(testKey, expectedValue)
	assert.Nil(t, err)

	val, err := txn.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, val)

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	_, err = txn2.Get(testKey)
	assert.Equal(t, "cannot acquire read lock as write lock acquired by transaction '1'", err.Error())

	txn.Rollback()

	val, err = txn2.Get(testKey)
	assert.Nil(t, err)
	assert.Equal(t, "", val)
}

// t1 acquires write lock and does multiple write operations
// t2 will not be able to acquire read lock on any of the keys
// t1 commits, t2 will be able to read and reads all of the update values
func TestCommitReleasesLockAndPersistsAllWrites(t *testing.T) {
	dbInstance, err := NewDB(Config{})
	assert.Nil(t, err)

	txn, err := dbInstance.Begin()
	assert.Nil(t, err)

	for i := 200; i <= 210; i++ {
		err = txn.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		assert.Nil(t, err)
	}

	txn2, err := dbInstance.Begin()
	assert.Nil(t, err)

	for i := 200; i <= 210; i++ {
		_, err := txn2.Get(fmt.Sprintf("key_%d", i))
		assert.Equal(t, "cannot acquire read lock as write lock acquired by transaction '1'", err.Error())
	}

	txn3, err := dbInstance.Begin()
	assert.Nil(t, err)

	txn.Commit()

	for i := 200; i <= 210; i++ {
		val, err := txn3.Get(fmt.Sprintf("key_%d", i))
		assert.Nil(t, err)
		assert.Equal(t, fmt.Sprintf("value_%d", i), val)
	}
}

// todo: any test we can add to prove atomic nature of commit?
// todo: test where we are firing goroutines.
