## Building LSM tree
https://www.alibabacloud.com/blog/starting-from-zero-build-an-lsm-database-with-500-lines-of-code_598114
- Let's build some benchmark on how is the performance currently?
- We need to start writing to disk but isn't LSM tree just a merged and compacted version of WAL?
    - No, that is append only while LSM trees keep data in SSTables (Sorted String Table) ==> which are sorted. 
- I guess we have multiple files (called as segments) in LSM tree. If yes, how do we break the files.

### How would the flow look like in case of LSM trees
* Step 1: Write to WAL.
* Step 2: Insert into Memtable. Sorted in-memory structure like red-black tree or skip list.
* Step 3: If the size of Memtable goes beyond a threshold eg. 40 MB, we should reset the memtable (basically create a new version) and before that, dump the existing memtable to a file in SSTable.
    * If we only do this much, how will our reads be performant?
    * What happens if not found in memtable?

ok, for simplicity, I am thinking of starting the implementation with only 1 level: L0 and we can later make our system more performant. 
* **Write logic**: When memtable size is more than 5 mb (for simplicity right now), create a new file with memtable content. each file will be of 5 mb. this memtable content file will only have unique keys. in order to keep the file names sorted, for now I will adopt a strategy of using epoch timestamp for now. that might not work later as potentially for a specific epoch timestamp (second or nanosecond), there can be multiple memtable created?? will look at this later.   
* **Read logic**: Check in memtable first, then in the most recent SSTable file and so on till the oldest SSTable file.
* **Benefits of the current design**
    * This is a stepping stone design for us to be able to efficiently be able to search for a key using binary search on disk.
    * We will also not keep duplicate key value pair in the WAL now ==> for the entire history.
    * Duplicacy will reduce to a certain extent as a single SSTable file won't have any duplicates.
* This approach has better performance than earlier approach + handles RAM limitations.

### What is still missing in above design
* WAL is still growing forever.
    * We will clear the WAL when we are flushing the SSTable snapshot to disk.
* We still require loading the entire file in-memory. We require adding indexes for faster lookups.
    * For v1, we can add streaming capability which will help with not requiring to load the entire SSTable file in-memory.

### Step 1: Memtable implementation: Will use below package
"github.com/google/btree"

### L0 SSTable Structure for effective binary search
* Before insertion, need to break the 5MB memtable into 4kb blocks.
* Can be done during iteration.
* Also, need to create another map for index. key ==> first key of the block, value ==> offset at which the block starts. How to identify the offset at which the block starts?
* In a block, there will only be key and value as string.
* Structure of a block "PUT key value"  ==> not keeping checksum or length here for now for simplicity. Payload will be PUT key value. (this is for simplicity, we can improve later)
* We will create a []byte array of this and then do file.Write. After doing file.Write, the n integer that is returned should be the offset. That way we will keep maintaining the index.
* After this, we will write the index block. Structure of index block: 
    * key ==> first key of the block, value ==> offset
    * [key_length -> 4 bytes][key][offset -> 4 bytes]
* After writing the index block, we need to write the footer. Footer structure:
    * [index offset -> 4 bytes]

### Need to add tests
* Logic for flushing memtable to sstable in the correct block based format with index and footer block (d)
* Logic for loading the sstable indexes in-memory during application startup. (d)
* Write a test which calls flushMemtableToSsTable first and then calls buildSsTableIndexes. Assert the output of buildSsTableIndexes. (wip)
    * Tests were failing with build errors because go test first runs go vet but this behaviour is not there during go run and go build.
    * So technically, passing a map does compile - it's valid Go code. The map is just treated as a single any argument.
        The issue is that go vet has a static analysis checker specifically for slog that validates the semantic usage of those arguments. The slog package expects args to follow a specific pattern:
        Alternating key-value pairs: "key1", value1, "key2", value2, ...
        Or slog.Attr values: slog.String("key", "value"), slog.Int("count", 42)
    * I had missed not writing the footer.
    * I had put incorrect file name in os.Stat.
    * I was missing to not add the last data block.
* Above are fixed now.


### Logic for reading SSTable
* Check the index offset from footer.
* Load the entire index in-memory.
* Run binary search for getting the appropriate block which has key <= "required key".
* Run linear search on the block.

## Todo
* Seeing flaky nature in tests. Sometimes they are failing.
    * Identified 2 issues: Issue with the last block.
        * Don't write last block if size is 0. [Done]
        * Since last block can also have size > limit, track the indexOffset also in map for effective search [Todo]
* what is write amplification?
* bloom filters

## Benchmarking for performance
* SS Table with index solution.
    * Fallback of memTable as ssTable instead of fallback as wal.
    * Everything in WAL solution. ==> create another DB implementation which doesn't use SSTable and only writes to WAL. Each time memtable size limit is reached, we create a new WAL file. 

* Result with 200 Loop
go test -bench=.
goos: darwin
goarch: arm64
pkg: github.com/golang-db
cpu: Apple M1 Pro
BenchmarkSSTableBinarySearchMixedWorkload-8          884           1333306 ns/op
BenchmarkSSTableLinearSearchMixedWorkload-8          201           5716382 ns/op
PASS
ok      github.com/golang-db    5.704s

* With 300 loop
go test -bench=.
goos: darwin
goarch: arm64
pkg: github.com/golang-db
cpu: Apple M1 Pro
BenchmarkSSTableBinarySearchMixedWorkload-8          744           1357287 ns/op
BenchmarkSSTableLinearSearchMixedWorkload-8          145           8045538 ns/op
PASS
ok      github.com/golang-db    6.043s