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
* **What real databases do in case of a crash**
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

## Adding graceful termination

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

### Logic for reading SSTable
* Check the index offset from footer.
* Load the entire index in-memory.
* Run binary search for getting the appropriate block which has key <= "required key".
* Run linear search on the block.

## Todo: bloom filters