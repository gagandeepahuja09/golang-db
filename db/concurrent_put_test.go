package db

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Run this with go test -race ./...
func TestDb_ConcurrentPuts(t *testing.T) {
	db, err := NewDB(Config{})
	assert.NoError(t, err)

	var wg sync.WaitGroup

	// Spawn 100 goroutines, each doing 100 puts
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				db.Put(key, value)
			}
		}(i)
	}

	wg.Wait()

	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			expectedValue := fmt.Sprintf("value_%d_%d", i, j)
			actualValue, err := db.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, expectedValue, actualValue)
		}
	}
}
