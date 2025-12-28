package main

import (
	"errors"
	"fmt"
	"os"
)

func (db *DB) cmdGet(args []string) {
	if len(args) != 2 {
		fmt.Fprint(os.Stderr, "Expected exactly 1 argument for GET command\n")
		return
	}
	key := args[1]
	value, ok := db.memTable.Get(key)
	if !ok {
		value, err := db.getValueFromSsTable(key)
		if err != nil {
			fmt.Printf("No value found for GET %s. Error: %s\n", key, err)
		} else {
			fmt.Printf("GET %s returned: %s\n", key, value)
		}
	} else {
		fmt.Printf("GET %s returned: %s\n", key, value)
	}
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
