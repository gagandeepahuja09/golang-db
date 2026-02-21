package sqlparser

type CreateTable struct {
	TableName                string
	ColumnDetails            []Column
	PrimaryKeyColumnPosition int
}

type InsertIntoTable struct {
	TableName    string
	ColumnValues []string // can be any of the data types but during SQL parsing we would store them
	// as []string and while storing in disk, they will be serialised to consume space as per the data type.
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
