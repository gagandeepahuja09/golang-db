package sqlparser

type CreateTable struct {
	TableName                string
	ColumnDetails            []Column
	PrimaryKeyColumnPosition int
	// todo: some checks for validating that indexes are not created with similar column or group of columns
	SecondaryIndexes []SecondaryIndex
}

type SecondaryIndex struct {
	Columns         []string
	IndexName       string
	ReservoirSample []string
	// should we store the reservoir sample for each index here?
	// so, we will have to update the reservoir sample for each INSERT both in the following in-memory variable and
	// then also do db.Put for this which is an expensive operation?
}

type InsertIntoTable struct {
	TableName    string
	ColumnValues []string // can be any of the data types but during SQL parsing we would store them
	// as []string and while storing in disk, they will be serialised to consume space as per the data type.
}

type QueryType string

const (
	Equals = "="
	Lt     = "<"
	Lte    = "<="
	Gte    = ">="
	Gt     = ">"
)

// support for GROUP BY, HAVING, etc will be added progressively. focus as of now is on simple
// WHERE conditions and AND clause
type QueryCondition struct {
	ColumnName string
	QueryType  QueryType
	Value      string
}

type SelectFromTable struct {
	TableName       string
	ColumnsRequired []string
	QueryConditions []QueryCondition
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
