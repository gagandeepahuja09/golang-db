package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/golang-db/memtable"
	"github.com/golang-db/sstable"
	"github.com/golang-db/wal"
)

type indexBlockEntry struct {
	key    string
	offset int
}

type DB struct {
	wal      *wal.Wal
	memTable *memtable.Memtable
	ssTable  *sstable.SsTable
}

func (db *DB) flushMemtableToSsTable() error {
	ssTableFile, err := db.ssTable.NewFile()
	if err != nil {
		return err
	}

	return db.ssTable.Write(ssTableFile, db.memTable.Iterate)
}

func (db *DB) writeToWal(key, value string) error {
	payload := fmt.Sprintf("PUT %s %s\n", key, value)
	return db.wal.WriteEntry(payload)
}

func (db *DB) buildMemtableFromWal() (*memtable.Memtable, error) {
	memTable := memtable.NewMemtable()
	for {
		payload, err := db.wal.ReadEntry()
		if err == io.EOF {
			return &memTable, nil
		}
		// for now, I will abort even in case of partial write
		// todo: in case of partial write we should just truncate that log.
		// we can also do that as part of listening to signal SIGTERM and SIGKILL?
		if err != nil {
			return nil, err
		}
		line := string(payload)
		args := strings.Split(line, " ")
		if len(args) != 3 {
			return nil, errors.New("Expected exactly 2 arguments for PUT command\n")
		}
		key := args[1]
		value := args[2]
		memTable.Put(key, value)
	}
}

func newDB(walFilePath string) (*DB, error) {
	db := DB{}
	wal, err := wal.NewWal("")
	if err != nil {
		return nil, err
	}
	db.wal = wal

	memTable, err := db.buildMemtableFromWal()
	if err != nil {
		return nil, err
	}
	db.memTable = memTable
	db.ssTable, err = sstable.NewSsTable("", 0)
	return &db, nil
}

func main() {
	db, err := newDB("")
	defer db.wal.Close()
	if err != nil {
		log.Fatal("Error while setting up DB: ", err.Error())
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		breakLoop := false
		switch cmd {
		case "GET":
			value, err := db.cmdGet(args)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("GET %s returned: %s\n", args[1], value)
			}

		case "PUT":
			err := db.cmdPut(args)
			if err != nil {
				fmt.Printf("Error while performing PUT operation: '%s'\n", err.Error())
			} else {
				fmt.Println("PUT operation performed successfully")
			}
		case "EXIT":
			breakLoop = true
		default:
			fmt.Println("Command not supported")
		}
		if breakLoop {
			break
		}
	}
}
