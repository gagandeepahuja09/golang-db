package main

import (
	"errors"
	"fmt"
)

func (db *DB) cmdGet(args []string) (string, error) {
	if len(args) != 2 {
		return "", errors.New("Expected exactly 1 argument for GET command\n")
	}
	key := args[1]
	value, ok := db.memTable.Get(key)
	if !ok {
		value, err := db.getValueFromSsTable(key)
		if err != nil {
			return "", fmt.Errorf("No value found for GET %s. Error: %s", key, err)
		} else {
			if value == "" {
				return "", fmt.Errorf("No value found for GET %s", key)
			}
			return value, nil
		}
	}
	return value, nil
}

func (db *DB) cmdPut(args []string) error {
	if len(args) != 3 {
		return errors.New("Expected exactly 2 arguments for PUT command\n")
	}
	key := args[1]
	value := args[2]
	if err := db.writeToWal(key, value); err != nil {
		return errors.New("Something went wrong")
	}
	db.memTable.Put(key, value)

	if db.memTable.ShouldFlush() {
		// todo: log for error
		return db.flushMemtableToSsTable()
	}
	return nil
}
