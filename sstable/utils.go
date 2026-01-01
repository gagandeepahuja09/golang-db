package sstable

import (
	"sort"
)

func sortedKeys(mp map[string]string) (keys []string) {
	for key, _ := range mp {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
