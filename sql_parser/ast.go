package sqlparser

type CreateTable struct {
	TableName                string
	ColumnDetails            []Column
	PrimaryKeyColumnPosition int
}

type DataType uint8

const (
	Int    DataType = 0
	String DataType = 1
	Bool   DataType = 2
)

type Column struct {
	ColumnName string
	DataType   DataType
}
