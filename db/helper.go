package db

func (db *DB) getColPositionFromColName(tableName, colName string) int {
	for i, col := range db.tableNameVsSchemaMap[tableName].ColumnDetails {
		if col.ColumnName == colName {
			return i
		}
	}
	return -1
}
