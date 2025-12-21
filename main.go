package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
)

type DB struct {
	data    map[string]string
	walFile *os.File
}

func (db *DB) cmdGet(args []string) {
	if len(args) != 2 {
		fmt.Fprint(os.Stderr, "Expected exactly 1 argument for GET command\n")
		return
	}
	key := args[1]
	value, ok := db.data[key]
	if !ok {
		fmt.Printf("No value found for GET %s\n", key)
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
	db.data[key] = value
	return nil
}

func (db *DB) writeToWal(key, value string) error {
	if _, err := db.walFile.WriteString(fmt.Sprintf("PUT %s %s\n", key, value)); err != nil {
		slog.Error("WAL_WRITE_FAILED", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	return db.walFile.Sync()
}

func buildDatabaseFromWal() (*DB, error) {
	data := make(map[string]string)
	file, err := os.OpenFile("wal.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		slog.Error("WAL_FILE_OPEN_FAILED", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}
	scanner := bufio.NewScanner(file)

	db := &DB{
		data:    data,
		walFile: file,
	}
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		if len(args) != 3 {
			return nil, errors.New("Expected exactly 2 arguments for PUT command\n")
		}
		key := args[1]
		value := args[2]
		db.data[key] = value
	}
	return db, nil
}

func main() {
	db, err := buildDatabaseFromWal()
	if err != nil {
		log.Fatal("Error while setting up DB")
	}
	defer db.walFile.Close()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		breakLoop := false
		switch cmd {
		case "GET":
			db.cmdGet(args)
		case "PUT":
			err := db.cmdPut(args)
			if err != nil {
				fmt.Printf("Error while performing PUT operation: '%s'\n", err.Error())
			} else {
				fmt.Println("PUT operation performed successfully")
			}
		case "EXIT":
			breakLoop = true
		}
		if breakLoop {
			break
		}
	}
}
