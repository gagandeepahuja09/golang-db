package memtable

import (
	"github.com/google/btree"
)

const (
	memtableSizeLimit = 100 // 100 bytes for testing (for now)
)

type Memtable struct {
	tree *btree.BTree
	size int
}

type Entry struct {
	Key   string
	Value string
}

func (e *Entry) Less(than btree.Item) bool {
	return e.Key < than.(*Entry).Key
}

func NewMemtable() Memtable {
	return Memtable{
		tree: btree.New(32),
	}
}

func (m *Memtable) Get(key string) (string, bool) {
	item := m.tree.Get(&Entry{Key: key})
	if item == nil {
		return "", false
	}
	return item.(*Entry).Value, true
}

// Iterate loops through each of the key, value pair in the memTable
func (m *Memtable) Iterate(fn func(key, value string)) {
	m.tree.Ascend(func(item btree.Item) bool {
		e := item.(*Entry)
		fn(e.Key, e.Value)
		return true
	})
}

func (m *Memtable) Put(key, value string) {
	entry := Entry{
		Key:   key,
		Value: value,
	}
	if old := m.tree.ReplaceOrInsert(&entry); old != nil {
		m.size += (len(value))
		m.size -= (len(old.(*Entry).Value))
	} else {
		m.size += (len(key) + len(value))
	}
}

func (m *Memtable) ShouldFlush() bool {
	return m.size >= memtableSizeLimit
}

func (m *Memtable) GetSize() int {
	return m.size
}

func (m *Memtable) Clear() {
	m.tree.Clear(false)
	m.size = 0
}
