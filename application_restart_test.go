package main

import (
	"testing"

	"github.com/golang-db/db"
	"github.com/stretchr/testify/assert"
)

func TestPutFirst_GetAfterAppRestart(t *testing.T) {
	defer dbDirCleanUp(t)
	// what happens if tests run in parallel?

	dbForPut, err := db.NewDB(testDbConfig)
	assert.NoError(t, err)
	buildTestData(dbForPut)

	// new instance created to test for app restart
	// creating a separate instance is similar to testing for app restart
	dbForGet, err := db.NewDB(testDbConfig)
	assert.NoError(t, err)

	assertValuesForTestData(t, dbForGet)
}
