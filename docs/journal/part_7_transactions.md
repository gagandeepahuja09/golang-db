- Plan is to add transactions capability.
    - To keep data correct when things go wrong or happen at the same time.
- **Atomicity**: All or nothing
- **Consistency**: Rules are never broken. Achieved via CONSTRAINTS in SQL databases.
- **Isolation**: Concurrent users don't mess each other up.
- **Durability**: Once committed, it stays committed even if the server crashes.

## Isolation Anomalies
1. Dirty Reads (Read uncommitted)
    User reads uncommitted data which might be rolled-back later.
2. Lost Updates
    - LWW: Last Write Wins.
    - One update overwrites the other.
    - How databases solve it:
        - Write locks
        - Optimistic locking
3. Write Skew
    - Concurrent updates on different rows affecting the results
    - both A and B do count(*) to check if someone is oncall
    - since count > 0, both update at the same time.
    - Both read valid snapshots but the combined result is invalid.

## Concurrency Control Strategies
1. Pessimistic Locking: 2PL
    * Why is it called 2PL?
        * Acquire phase: between BEGIN and COMMIT
        * Release phase: after COMMIT
2. Optimistic (MVCC): Snapshot Isolation
    - Not full serializability: write skew can still happen
3. Serializable Snapshot Isolation

## Basic Transactions Implementation Plan With 2 PL [Done]
1. **BEGIN: starts a transaction**. [Done]
    - Create a txn id.
    - Each transaction should be associated with a transaction id.
        - *Why do we need a transaction id?* When we COMMIT or ROLLBACK, all of the acquired locks for different keys need to be released. If we don't create a relevant transaction id, how would we come to know that lock acquired by which txn needs to be released. 
    - We can keep the txn id in-memory and auto-increment. Because if someone exits the application, then the open transaction needs to be rolled back. (no persistence required)
2. **How to store locks acquired for a key?**
    - map[key]struct{ readers: []string => txn_ids, writer: string => txn_id }
    - Also, a mutex to update this shared variable
    - While this map is useful for GET and PUT functions to come to know if a key is locked or not and whether by readers or writers, we also need a way to come to know that what all keys were locked by a transaction_id. This is necessary to release the required locks during GET and PUT.
        map[transaction_id]{ list of keys }. Instead of map, the list of keys acquired
    - When we release locks for a transaction, we need to update both the maps.  
3. **PUT within a transaction**: [Done]
    - **Acquiring Lock**
    - Check for the specific key if some read or write lock is acquired.
    - Read from above map
    - Check writer
        - If write txn_id is present and len > 0 and txn_id in map is not equal to current txn_id: BLOCK or ABORT
        - Else: update map: writer
    - Check readers
        - If size > 1: BLOCK or ABORT
        - If size == 1
            - If reader txn_id == current txn_id
                - Updgrade to write lock: remove from readers and add in write struct
            - Else: BLOCK or ABORT
    - **Buffered Write**
    - Don't persist this by calling Put functon.
    - Maintain the key, value pair in some in-memory map.
    - map[transaction_id][]struct{ key, value }
        - This is not a shared variable as within a goroutine, there can be only one transaction_id
    - These operations will be performed in order during COMMIT.
4. **GET within a transaction:** [Done]
    - Check buffered write map first for the transaction_id.
    - Check writer
        - If writer txn_id is present and len > 0 and not equal to current txn_id: BLOCK or ABORT
        - Else: update map: readers
            - While updating, check if txn_id is already present in readers slice.
    - If not found, call the existing GET function.
5. **COMMIT** 
    - List of all buffered writes need to be replayed as PUT commands.
    - They need to be applied atomically at the WAL also.
    - We will call wal.WriteEntry function only once where the payload would have all of the PUT commands in a single entry.    
    - Payload for WAL can be something like: TRANSACTION_BEGIN PUT key1 value1 PUT key2 value2 TRANSACTION_END
    - The function buildMemtableFromWal needs to be updated to also handle above type of payload. Above payload will not be able to handle space related key-value pairs. Such conditions can be handled by serialising further in a better way: [length_of_trasaction_begin_string][TRANSACTION_BEGIN][length_of_key1][key1][length_of_value_1][value_1][length_of_key_2]...[length_of_TRANSACTION_END][TRANSACTION_END][checksum_of_the_entire_payload]
    - Once WAL is written successfully, we move to calling db.memTable.Put function sequentially. Even it this fails, we are fine as when the application restarts, buildMemtableFromWal will take care of populating all the keys ensuring atomicity.
    - After WAL write and Put calls, are locks acquired for txn_id are released.
    - Buffered writes are also cleaned up.
6. **ROLLBACK**
    - All locks released by updating map
    - Buffered writes cleaned up.

## Note On Current vs Expected Concurrency Which We Need to Handle
- We have handled multiple goroutines / threads in the same process via sync.RWMutex.
- We need to solve for multiple processes, each having their own DB instances.
- They need a file-level lock to coordinate.

## MVCC
* With 2 PL, we have implemented Serializable isolation.
* 2PL gives us the strongest level of isolation but that comes at the cost of concurrency as readers block writers and writers block readers.
* MVCC takes an optimistic approach of detecting data integrity issues before they are committed.
* Problem with MVCC: Write Skew.
* https://vladmihalcea.com/write-skew-2pl-mvcc/

### Write Skew Problem
Constraint: x + y >= 0
x = 5, y = 5
T1: reads x = 5, y = 5, sets x = -5 ==> constraint valid ==> commits
T2: reads x = 5, y = 5, sets y = -5 ==> constraint valid ==> commits
Result ==> x + y = -10

### How can we improve the concurrency? How can we ensure that readers and writers don't block each other?
-  T1, T2 begin
- PUT abc, 123 --> T2 
- GET abc --> T1
- We can solve read uncommitted by ensuring transactions only read their buffered writes. But this doesn't solve the non-repeatable read problem.

### Non-repeatable read problem
- Same key read twice, gives different values.
- GET abc 50 -> T1
- PUT abc 100 -> T2
- T2 COMMIT
- GET abc -> 100
#### Why is non-repeatable read such a big problem?
- Many transactions do this:
    - Read something
    - Make a decision 
    - Perform an operation
- In such cases, while perform an operation, some transaction committed, hence they are instead operating on inconsistencies.
- T1: GET abc -> 50
- T2 committed and did PUT abc 100
- T1: PUT abc = abc + 20 ==> T1 is acting on stale assumptions.
- One way to solve the non-repeatable read problem is by writing smarter queries. This is called as **compare-and-swap** also. Example:
    - BEGIN
    - SELECT amount FROM payments where id = 123;
    - UPDATE amount SET amount = amount + 100 WHERE id = 123 and amount = {read_from_query_above};
    - COMMIT
- Compare-and-swap might be common in production systems but it only works when :
    - Limited to 1 row.
    - Applications have ability to retry or handle cases where no row is updated. 

### Intuition Behind MVCC   
- Implementing this doesn't seem that interesting?

#### PLAN
* Should I build INSERT --> SELECT --> Multiple AND SUPPORT --> RANGE QUERIES SUPPORT --> INDEXES --> COMPOSITE INDEXES --> null COLUMNS SUPPORT --> ALTER COMMAND SUPPORT?