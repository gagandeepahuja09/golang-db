package db

import (
	"path/filepath"
	"testing"

	"github.com/golang-db/sstable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDBForWalCommandTest(t *testing.T) (*DB, Config) {
	t.Helper()

	dir := t.TempDir()
	config := Config{
		SsTableConfig: sstable.Config{
			DataFilesDirectory: filepath.Join(dir, "sstable"),
		},
		WalFilePath: filepath.Join(dir, "wal.log"),
	}

	dbInstance, err := NewDB(config)
	require.NoError(t, err)
	return dbInstance, config
}

func TestSerialisePutCommandRoundTrip(t *testing.T) {
	payload := serialisePutCommand("key with spaces", "value with spaces\nand newline")

	offset := 0
	cmd, err := readLengthPrefixedString(payload, &offset)
	require.NoError(t, err)
	assert.Equal(t, CmdPut, cmd)

	key, value, err := deserialisePutCommand(payload, &offset)
	require.NoError(t, err)
	assert.Equal(t, "key with spaces", key)
	assert.Equal(t, "value with spaces\nand newline", value)
}

func TestReadLengthPrefixedStringMalformedPayload(t *testing.T) {
	testCases := []struct {
		name string
		buf  []byte
	}{
		{
			name: "missing length",
			buf:  []byte{0, 0, 0},
		},
		{
			name: "string length exceeds payload",
			buf:  []byte{0, 0, 0, 5, 'a'},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			offset := 0
			_, err := readLengthPrefixedString(tt.buf, &offset)
			assert.Error(t, err)
		})
	}
}

func TestDeserialisePutCommandRejectsTrailingBytes(t *testing.T) {
	payload := append(serialisePutCommand("key", "value"), 'x')

	offset := 0
	cmd, err := readLengthPrefixedString(payload, &offset)
	require.NoError(t, err)
	require.Equal(t, CmdPut, cmd)

	_, _, err = deserialisePutCommand(payload, &offset)
	assert.Error(t, err)
}

func TestSerialiseTransactionCommitPayloadRoundTrip(t *testing.T) {
	payload := serialiseTransactionCommitPayload(map[string]string{
		"txn key with spaces": "txn value with spaces",
		"txn key newline":     "txn value\nwith newline",
	})

	offset := 0
	cmd, err := readLengthPrefixedString(payload, &offset)
	require.NoError(t, err)
	assert.Equal(t, CmdTransaction, cmd)

	putCmds, err := deserialiseTransactionCommand(payload[offset:])
	require.NoError(t, err)

	actual := map[string]string{}
	for _, putCmd := range putCmds {
		actual[putCmd.key] = putCmd.value
	}

	assert.Equal(t, map[string]string{
		"txn key with spaces": "txn value with spaces",
		"txn key newline":     "txn value\nwith newline",
	}, actual)
}

func TestDeserialiseTransactionCommandRejectsMalformedPayload(t *testing.T) {
	_, err := deserialiseTransactionCommand([]byte{0, 0, 0})
	assert.Error(t, err)
}

func TestDBRecoversPutValuesWithSpacesAndNewlines(t *testing.T) {
	dbInstance, config := newDBForWalCommandTest(t)

	expected := map[string]string{
		"simple":          "value with spaces",
		"key with spaces": "value with\nnewline",
	}
	for key, value := range expected {
		require.NoError(t, dbInstance.Put(key, value))
	}
	dbInstance.Close()

	dbAfterRestart, err := NewDB(config)
	require.NoError(t, err)
	defer dbAfterRestart.Close()

	for key, expectedValue := range expected {
		value, err := dbAfterRestart.Get(key)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, value)
	}
}

func TestDBRecoversLatestValueAfterOverwrite(t *testing.T) {
	dbInstance, config := newDBForWalCommandTest(t)

	require.NoError(t, dbInstance.Put("same key", "old value"))
	require.NoError(t, dbInstance.Put("same key", "new value\nwith newline"))
	dbInstance.Close()

	dbAfterRestart, err := NewDB(config)
	require.NoError(t, err)
	defer dbAfterRestart.Close()

	value, err := dbAfterRestart.Get("same key")
	require.NoError(t, err)
	assert.Equal(t, "new value\nwith newline", value)
}

func TestDBRecoversTransactionValuesWithSpacesAndNewlines(t *testing.T) {
	dbInstance, config := newDBForWalCommandTest(t)

	txn, err := dbInstance.Begin()
	require.NoError(t, err)
	require.NoError(t, txn.Put("txn key with spaces", "txn value with spaces"))
	require.NoError(t, txn.Put("txn key newline", "txn value\nwith newline"))
	require.NoError(t, txn.Commit())
	dbInstance.Close()

	dbAfterRestart, err := NewDB(config)
	require.NoError(t, err)
	defer dbAfterRestart.Close()

	value, err := dbAfterRestart.Get("txn key with spaces")
	require.NoError(t, err)
	assert.Equal(t, "txn value with spaces", value)

	value, err = dbAfterRestart.Get("txn key newline")
	require.NoError(t, err)
	assert.Equal(t, "txn value\nwith newline", value)
}
