# SQL Support
- Now we need to extend the design for SQL to KV store.

## Modelling SQL operations as KV operations (GET, PUT, DELETE)
- One key insight is that each SQL operation will be a GET or PUT or DELETE operation in database.
- Example: how to Support INSERT operation?
    - PUT operation [table_id:primary_key] --> [serialized_row]
    - : is a separator here. though not the safest one. for simplicity, we will use it in the current design.
    - How to serialize and deserialize the row?

## Row Serialization
- col1 (int32) | col2(string) | col3(bool)
- Need to handle both variable and fixed length data types.
- Represent each data type with a unique number. For simplicity: bool = 0, int32 = 1, string = 2
- For fixed length data types: `[dataTypeNumber][data]`
- For variable length data types: `[dataTypeNumber][dataLength][data]`
- We don't necessarily need to store the data type number. Instead we can rely on the sequence.
- Todo:
    - What happens in case of ALTER TABLE if a column is added or removed?
    - Not supporting NULL or things like auto-increment or UUID for now.
- So, what happens when we search by primary key, let's say?
    - Run GET operation with key as "table_id:primary_key".
    - Read the value and deserialize it. byte to data types.
    - Output the result as per the data types.
- Note: this serialization and deserialization will happen both during memtable and SS Table reads and writes.

### Note on binary serialisation, deserialisation performance compared to JSON

## SQL Parsing
- check command O ==> CREATE TABLE ==> call insertTable function ==> split by comma to get datatype and column name.

## What happens during CREATE TABLE? [Done]
- CREATE TABLE payments (id int, is_refundable bool, amount int, status string)
- maintain a struct for Table in-memory.
- For V1, not adding any table_id
- We also need to persist this in the database. Key: [create_table:<table_name>], Value: serialized view of the schema: [columnIndexOfPrimaryKey][dataTypeNumber1][columnNameLength1][columnName1][dataTypeNumber2][columnNameLength2][columnName2]... till last column.
- So, this will be saved by calling Put function. (Memtable + WAL) ==> flush to SS-Table when size reaches threshold as per existing design. 
- since datatypes will be determined at runtime, we need to store it mostly as key value pair
- In the DB struct, add a tableNameVsMetadataMap map[string]TableMetadata
```go
    type TableMetadata struct {
        primaryKeyColumnIndex int
        columns []TableColumn
    }

    type TableColumn struct {
        columnName string
        dataType string // enum: bool, int32, string for now
    }
```

### Handling startup recovery
* We should have a key like _catalog which has the list of all tables.
* Then we can perform individual GET operations during application startup.
* This works but it requires multiple random reads. We can instead adopt an approach of a single sequential read.
    * Not solving this as of now as it is a COLD read path.

## What happens during INSERT INTO ...?
- Call Put function, Key: [<table_name>:<primary_key_id>], Value: [dataLength1][data1][data2][dataLength3][data3].
- length will be prefix for tables which have variable length data type. 1 and 3 are variable types in above example.
- This is written in Memtable, Wal and SSTable.

## What happens during SELECT ?
- Starting with only SELECT * and only SELECT on primary key.
- Call Get function, Key: [<table_name>:<primary_key_id>].
- Get the value in-memory and deserialize it. When deserializing check for whether the data type is of fixed length or of variable length and accordingly deserialize it.
- Once the data is being deserialized, we can just return an output string out of it. We just keep on appending to the string based on the datatype. Example: `""" "data_value1", 2, "value2" """`.

### Claude Review Comments To Think About Later
Things to think about:

  1. Key prefix collision: Your schema key is create_table:<table_name>. What if someone creates a table literally named create_table? Then you'd have create_table:create_table as schema key, and create_table:123 could be ambiguous (is it a row in table "create_table" or schema for table "123"?).

  1. Consider: use a prefix that can't be a valid table name, like _schema: or \x00schema:. [Done]
  2. Startup recovery: You have tableNameVsMetadataMap in memory. When your DB restarts, how do you rebuild this map? You'll need to scan for all create_table:* keys and reload schemas. [Done]

### Todo: SQL Planner ==> When is that useful?
* When there are multiple WHERE conditions (multiple indexes) or multiple tables or multiple join algorithms.

* Parser → converts query string to a struct
* Serialiser → serialises in DB in byte array such that it can be easily deserialised during reads.

### Todo: SELECT range-based queries
* As of now, the data is sorted in lexicographic order. This means that 100 will come before 11.
* For range-based queries like `WHERE age > 10 AND age < 15`, it won't work.

In order to learn internals of database better, I am building one in golang. I have built a key-value store with ss table, index blocks, compaction and was now adding SQL capabilities. added create table and then will add insert and select commands. there are multiple path ways: focus on alter after that, like column addition or deletion OR select capabilities range based / joins / query planner OR potentially most interesting: Transaction capabilities and ISOLATION levels. help me plan what to choose?