package db

import (
	"testing"

	sqlparser "github.com/golang-db/sql_parser"
	"github.com/stretchr/testify/assert"
)

func TestSerialiseAndDeserialiseCreateTableInput(t *testing.T) {
	expectedCreateTableInput := sqlparser.CreateTable{
		PrimaryKeyColumnPosition: 2,
		ColumnDetails: []sqlparser.Column{
			{
				DataType:   2,
				ColumnName: "is_payment_captured",
			},
			{
				DataType:   1,
				ColumnName: "status",
			},
		},
	}
	buf := serialiseCreateTableInput(expectedCreateTableInput)
	deserializedInput, err := deserialiseCreateTableInput(buf)

	assert.NoError(t, err)
	assert.Equal(t, expectedCreateTableInput, *deserializedInput)
}
