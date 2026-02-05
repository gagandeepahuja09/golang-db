package db

import (
	"fmt"

	"errors"
)

const (
	WriteLockNotAcquiredDueToReadLocksError = "cannot acquire write lock as read lock acquired by one or more transactions"
)

type Transaction struct {
	id               uint64
	db               *DB
	bufferedWriteMap map[string]string
	lockAcquiredKeys []string
}

func (txn *Transaction) tryAcquireWriteLock(key string) error {
	locksAcquired, ok := txn.db.transactionManager.keyVsLocksAcquiredMap[key]
	readLockAlreadyAcquired := false
	if !ok {
		locksAcquired = &LocksAcquired{}
	} else {
		writerTxnId := locksAcquired.writerTxnId
		if writerTxnId != 0 && writerTxnId != txn.id {
			// todo: can we have a wait feature where we wait for lock to be released instead of error?
			return fmt.Errorf("cannot acquire write lock as write lock acquired by transaction '%d'", writerTxnId)
		}

		readerTxnIds := locksAcquired.readerTxnIds
		if len(readerTxnIds) > 1 {
			return errors.New(WriteLockNotAcquiredDueToReadLocksError)
		}
		if len(readerTxnIds) == 1 {
			if readerTxnIds[0] == txn.id {
				readLockAlreadyAcquired = true
				locksAcquired.readerTxnIds = []uint64{}
			} else {
				return errors.New(WriteLockNotAcquiredDueToReadLocksError)
			}
		}
	}
	locksAcquired.writerTxnId = txn.id
	if txn.lockAcquiredKeys == nil {
		txn.lockAcquiredKeys = []string{}
	}
	if !readLockAlreadyAcquired {
		txn.lockAcquiredKeys = append(txn.lockAcquiredKeys, key)
	}
	txn.db.transactionManager.keyVsLocksAcquiredMap[key] = locksAcquired
	return nil
}

func (txn *Transaction) tryAcquireReadLock(key string) error {
	locksAcquired, ok := txn.db.transactionManager.keyVsLocksAcquiredMap[key]
	writeLockAlreadyAcquired := false
	if !ok {
		locksAcquired = &LocksAcquired{}
	} else {
		writerTxnId := locksAcquired.writerTxnId
		if writerTxnId != 0 {
			if writerTxnId != txn.id {
				return fmt.Errorf("cannot acquire reader lock as write lock acquired by transaction '%d'", writerTxnId)
			} else {
				writeLockAlreadyAcquired = true
			}
		}
	}
	for _, txnId := range locksAcquired.readerTxnIds {
		if txnId == txn.id {
			return nil
		}
	}
	locksAcquired.readerTxnIds = append(locksAcquired.readerTxnIds, txn.id)
	if txn.lockAcquiredKeys == nil {
		txn.lockAcquiredKeys = []string{}
	}
	if !writeLockAlreadyAcquired {
		txn.lockAcquiredKeys = append(txn.lockAcquiredKeys, key)
	}
	txn.db.transactionManager.keyVsLocksAcquiredMap[key] = locksAcquired
	return nil
}

// todo: optimisation for later: sharded locks
func (txn *Transaction) Put(key, value string) error {
	txn.db.transactionManager.mu.Lock()
	err := txn.tryAcquireWriteLock(key)
	txn.db.transactionManager.mu.Unlock()
	if err != nil {
		return err
	}

	if txn.bufferedWriteMap == nil {
		txn.bufferedWriteMap = map[string]string{}
	}
	txn.bufferedWriteMap[key] = value
	return nil
}

func (txn *Transaction) Get(key string) (string, error) {
	txn.db.transactionManager.mu.Lock()
	err := txn.tryAcquireReadLock(key)
	txn.db.transactionManager.mu.Unlock()
	if err != nil {
		return "", err
	}
	if value, ok := txn.bufferedWriteMap[key]; ok {
		return value, nil
	}
	return txn.db.Get(key)
}

func (txn *Transaction) Commit() {

}
