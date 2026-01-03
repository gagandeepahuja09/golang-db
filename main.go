package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"errors"

	"github.com/golang-db/db"
)

func main() {
	db, err := db.NewDB(db.Config{})
	defer db.Close()
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
			value, err := cmdGet(db, args)
			if err != nil {
				// todo: error messaging not good for not found cases right now. It should not
				// have error prefix
				fmt.Printf("Error while performing GET operation: '%s'\n", err.Error())
			} else {
				fmt.Printf("GET %s returned: %s\n", args[1], value)
			}

		case "PUT":
			err := cmdPut(db, args)
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

func cmdGet(db *db.DB, args []string) (string, error) {
	if len(args) != 2 {
		return "", errors.New("Expected exactly 1 argument for GET command\n")
	}
	key := args[1]
	value, err := db.Get(key)
	if err != nil {
		return "", fmt.Errorf("No value found for GET %s. Error: %s", key, err)
	}
	if value == "" {
		return "", fmt.Errorf("No value found for GET %s", key)
	}
	return value, nil
}

func cmdPut(db *db.DB, args []string) error {
	if len(args) != 3 {
		return errors.New("Expected exactly 2 arguments for PUT command\n")
	}
	key := args[1]
	value := args[2]
	if err := db.Put(key, value); err != nil {
		return fmt.Errorf("Something went wrong: %s", err.Error())
	}
	return nil
}
