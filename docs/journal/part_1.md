Database
{
    map[string => TableName]Table
}
Table {
    []TableColumn
    []TableRows
}
## Create Table Functionality 

(t Table) Create() {

}

Table Name: All the column names which we will have
- Datatypes for each of the columns.
map[string]string OR better
[]struct{} => create a struct called TableColumn{ columnName, dataType, default value (to be added later)}
validation for data type values
- Default would be null for each

## Insert Row Functionality
- NewTable was already done
- Append to Table rows

## Select Or Read Functionality
- For now, just implement reading SELECT and WHERE clause

## But before that we need to make it CLI based with REPL: Read Execute Print Loop
- Simply need to start a while loop that is never ending till we type exit.
- When someone writes CREATE TABLE, I need to create struct at runtime? Or I will have to create maps instead?
- Create a map for table schema? map[string => table name]Table

CREATE TABLE table_name (
    column1 varchar(30),
    column2 int,   
)

Once I know the data type, when I run INSERT query, I will insert rows.
Is it possible to create TableRow struct on the fly? 
TableRow {
    column1 ==> value of column1
} 

--------------------------------------------------------------------------------------------
## Suggestion from Cursor: Build Key-Value Store First [Done]
1. REPL: GET, PUT and DELETE
- Run a while loop. switch condition for the first keyword.
- Exit based on EXIT keyword
- Do we really need any other data types except "string"? ==> no
- Do we have the concept of table in this? ==> no, KV store is like a single table

2. Learnings
- I was using os.Args which gives command line arguments before we start a program.
- stdin: where a program recieves input
- stdout: where a program writes normal output
- stderr: where a program write error messages

- To continuously read from stdin after starting application, we use bufio.Scanner

--------------------------------------------------------------------------------------------

* Reminder: Add UTs
* Let's write code for WAL

## Why Write-ahead Logs is not sufficient and We Need B-trees or LSM trees
* Problem: WAL grows forever. This leads to:
    * Imagine millions of key value store pairs ==> key might be same but value might be changing, hence there is no compaction happening.
    * This also leads to taking too much space.
    * **RAM or in-memory limits**: What if your RAM is only 16 GB and you have 100 GB dataset.
* Solution:
    * Checkpointing: Flush in-memory state to the on-disk structure and then truncate the WAL.
    * Process becomes:
        * Load on-disk snapshot first.
        * Then replay the WAL after the checkpoint.

## WAL V1 Logic
* Write to WAL first. I think this should be written in sync mode only. Can't make it async to improve performance as it is the source of truth and we can't afford for this to fail.
* We will add all put commands executed in a WAL file (\n separated).
* For now, this will be a single file in append-only mode. We will soon add compaction.
* During startup, we will replay the entire file.
* I will read the entire file directly. There is also bufio.Scanner option but we will soon add compaction, so won't need that.
* https://labex.io/tutorials/go-how-to-use-bufio-scanner-in-golang-431346
* **Writing to WAL after every put**
    * We would have to read the entire file and then append the command in the string and then write back during every PUT.
    * Is there a better way to do that?
* What happens if the WAL write is successful and then later:
    * Map write fails for some reason? ==> But how can map write fail? It is just map[key] = value
    * Only thing is that the process can crash due to memory or CPU limits.
    * In that case the WAL would have something which shouldn't be there. 

## Cursor provided a lot of thinking points related to WAL
* **fsync is very important**
    * Without fsync, OS might buffer writes and we would loose data on crash.
    * Tradeoff of potential 5-10 ms in case of HDD.
    * file.Sync
* **Newline won't work if key or value has \n, we would need a better solution**
* **You don't need to read entire file, use append-only mode**
    * This ensure that OS handles cursor positioning to be at the very end.
    * This will help with fast sequential writes.
* **Plan to use bufio.Scanner from the very start**
    * Ensures that we don't need to load the entire file in memory.
    * It streams line-by-line.
* Few more improvements required now:
    * We should call os.OpenFile only once during application struct.
    * We need to change DB from a map[string]string to a struct now:
    `
        type DB struct {
            data map[string]string
            walFile *os.File
        }
    `
* TODO: add support for DELETE command also.

Prompt: """okay. I have implemented a basic version of WAL. review the code. I know that we can create a separate DB package now. we also need to add DELETE funcationality and newline support. We can also change cmdGet similar to cmdPut to directly return the error. any other review comments before I move to first adding tests and then these 2 improvements and then moving to LSM trees? highlight missing important areas for the db before that which I might not be thinking."""

* TODO: we can improve the logging much more

## Logging Improvements
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("hello, world", "user", os.Getenv("USER"))

## Partial / Corruption Detection in WAL
### Partial Writes
* Process crashed mid-write. Only few bytes made it to disk.
* Wanted to write: PUT foo bar\n
* What got written: PUT foo b
* How to check? We can check by waiting for new line but that would mean that it get mixed. Example: PUT foo bPUT key value and cause *cascading failure*.
* Hence, we should utilise adding a prefix of length => [4 bytes: length][payload]

### Corrupted Writes
* Can be due to disk failure or hardware bugs.
    * Disk sector going bad.
    * RAM bit flip during write
    * "bit rot" over time on aging drives
* https://www.youtube.com/watch?v=izG7qT0EpBw
* We use Checksum for detecting corrupted writes
* CRC32 is a good standard for detecting corrupted writes.
* What to do? Store following: [4 bytes: length][payload][4 bytes: CRC32]
* *During reads*:
    * Read length.
    * If length doesn't match --> partial write
    * Get payload. Compute CRC32
    * Compare CRC32.
    * If doesn't match --> corruption --> skip OR abort
* **Question: to skip OR to abort**
* What to do if the CRC or length check fails
    * Option 1: Abort startup entirely
    * Option 2: My suggestion: If the check fails, we can still avoid aborting if PUT key value exists for the same key later when reading the WAL.
        * we can store a map[string]bool of keys which were corrupted.
        * keep on iterating this and updating.
        * even if a single key is corrupted abort.
    * I feel option 2 should be solution for an DB unless it is a cache instead where loosing out on some keys is fine. In case of cache we can just continue ahead and consider the key deleted.  
* **What real databases do in case of a crash** ==> todo: this is not yet done.
    * **For corruption mid-WAL**: Everything before is trusted, Everything after is lost.
        * Throw error that manual intervention is needed.
    * **For corruption at the end of WAL**: This is due to a partial-write due to process crash
        * Truncate and continue
* *The idea of tracking corrupted key is not that useful because corruption can happen in the key also*
* so, based on that logic is simple, right? ==> check length, let's say it is N ==> check N characters. If there are less than N characters ==> partial write ==> truncate.
* After N characters, check the next 4 bytes. if they don't match ==> throw error and stop.
* any other case that I am not thinking?

* How to read specifically 4 bytes? How will we come to know of EOF with no. of bytes?
    * We used io.ReadFull to read exactly the no. of bytes that we want to.
    * It takes the byte slide in input and we can create byte array exactly of the length that we need.

-----------------------------------------------------------------------------------------------------

## Our partial write solution is not fully working
- In case of a partial write, we should truncate.
- Can't we take an approach where we do graceful termination and wait to the write to be complete?

## What all test cases do we need right now?
1. GET, PUT and DELETE core logic.
2. Command not supported case.
3. Case of partial write.
4. Case of corrupted write.

## Todo: Adding graceful termination