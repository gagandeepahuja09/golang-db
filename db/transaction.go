package db

import (
	"encoding/binary"
	"fmt"

	"errors"
)

const (
	WriteLockNotAcquiredDueToReadLocksError = "cannot acquire write lock as read lock acquired by one or more transactions"
	CmdTransaction                          = "TRANSACTION"
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
				return fmt.Errorf("cannot acquire read lock as write lock acquired by transaction '%d'", writerTxnId)
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

func (txn *Transaction) releaseAllLocks() {
	for _, key := range txn.lockAcquiredKeys {
		locksAcquired := txn.db.transactionManager.keyVsLocksAcquiredMap[key]
		if locksAcquired.writerTxnId == txn.id {
			locksAcquired.writerTxnId = 0
		} else {
			currentTxnIdFound := false
			updatedReaderTxnIds := []uint64{}
			for _, readerTxnId := range locksAcquired.readerTxnIds {
				if readerTxnId == txn.id {
					currentTxnIdFound = true
					continue
				}
				updatedReaderTxnIds = append(updatedReaderTxnIds, readerTxnId)
			}
			if !currentTxnIdFound {
				fmt.Println("INCONSISTENCY_OBSERVED_lockAcquiredKeys_AND_keyVsLocksAcquiredMap", map[string]interface{}{
					"lockAcquiredKeys":      txn.lockAcquiredKeys,
					"keyVsLocksAcquiredMap": txn.db.transactionManager.keyVsLocksAcquiredMap[key],
					"key":                   key,
				})
			}
			locksAcquired.readerTxnIds = updatedReaderTxnIds
		}
	}
	txn.lockAcquiredKeys = []string{}
}

func (txn *Transaction) cleanupBufferedWriteMap() {
	txn.bufferedWriteMap = map[string]string{}
}

func (txn *Transaction) Rollback() {
	txn.releaseAllLocks()
	txn.cleanupBufferedWriteMap()
}

// structure: [length][payload][offset]
// payload structure:
// ----
// [length_of_command][command][number_of_writes]
// [payload_length_for_1st][payload_for_1st][payload_length_for_2nd][payload_for_2nd]...
// till N = number_of_writes
// ----
// command is "TRANSACTION" in this case
// todo: we need to change the structure of payload for PUT command also similar to this.
func serialiseTransactionCommitPayload(writeMap map[string]string) []byte {
	payloads := []string{}
	for key, value := range writeMap {
		payload := fmt.Sprintf("PUT %s %s", key, value)
		payloads = append(payloads, payload)
	}

	buf := []byte{}
	transactionStr := "TRANSACTION"
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(transactionStr)))
	buf = append(buf, []byte(transactionStr)...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(payloads)))

	for _, payload := range payloads {
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(payload)))
		buf = append(buf, []byte(payload)...)
	}
	return buf
}

// [number_of_writes][payload_length_for_1st][payload_for_1st][payload_length_for_2nd][payload_for_2nd]...
func deserialiseTransactionCommand(buf []byte) (payloads []string) {
	i := 0
	numWrites := binary.BigEndian.Uint32(buf[i : i+4])
	i += 4
	for j := 0; j < int(numWrites); j++ {
		payloadLength := binary.BigEndian.Uint32(buf[i : i+4])
		i += 4
		payload := string(buf[i : i+int(payloadLength)])
		i += int(payloadLength)
		payloads = append(payloads, payload)
	}
	return payloads
}

// necessary to do in a single WAL write for atomicity
func (txn *Transaction) writeSingleWalEntryForCommit() {
	buf := serialiseTransactionCommitPayload(txn.bufferedWriteMap)
	txn.db.wal.WriteEntry(buf)
}

func (txn *Transaction) Commit() {
	txn.writeSingleWalEntryForCommit()

	// put in memtable done separately instead of db.Put as that would lead to separate writes in WAL
	for key, value := range txn.bufferedWriteMap {
		txn.db.memTable.Put(key, value)
	}

	txn.releaseAllLocks()
	txn.cleanupBufferedWriteMap()
}
