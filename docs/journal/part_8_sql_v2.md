## How do databases support full-table scan?

======================================= NOT NEEDED ====================================================
- If INSERT INTO is supported via [<table_name>:<primary_key_id>] ==> we can easily find a row using GET.
- Problems:
    - How to get all the rows of a table?
        - We will have to add key = [`primary_keys`:<table_name>]
        - value = ['primary_key_1', 'primary_key_2', ] ... ==> this is the list of sorted primary keys. 
        - How will this list be serialised in DB?
            - If string: [length_of_list][length_of_pk1][pk1][length_of_pk2][pk2]...
            - If int: [length_of_list][pk1][pk2]...
        - So, INSERT INTO will do PUT on both <table_name>:<primary_key_id> and `primary_keys`:<table_name>.
        - Why sorted? This will assist in range based queries on primary key as well.
        - How will we insert in sorted array? Finding the index for insertion is a straightforward binary search problem.
        - But update in array is O(N). C++ Vectors make it ammortized O(1)?
- Query example: 
    - SELECT * FROM payments WHERE id >= 'abc123' AND id <= 'efg456'
    - GET primary_keys:payments ==> what if there are millions of keys?
    - Then we will have to create an index for this primary_keys key also.
======================================= NOT NEEDED ====================================================

- Above can instead be solved by prefix scan.
- Todo: see how these commands: CREATE TABLE, INSERT INTO and SELECT perform at scale and what needs to be done differently for supporting them.
    - How does full-table scan work with millions of rows?
    - How to support constraints like UNIQUE?
    - Adding INDEX support
    - ALTER Table command support
    - NOT NULL constraint support
    - Support 2 kinds of INSERT INTO syntaxes and the implications of it on how much space is consumed. 
- Supporting NULL values in database.
    - NULL values are different from zero values.
    - One key feature of NULL value should be that it doesn't take up space in the database.
    - How do we implement it?
    - Currently how we serialise data is such that we reserve 4 bytes for integer type while reading.
    - Alternately we can store an additional byte telling whether it is NULL or NOT NULL.
    - If it is null, we don't read the 4 bytes.

## Handling non-primary key based queries (Full-table scans)
- Before building the approach for full-table scan, lets take a step back and revise how the data is structured.
- Each file is a mem-table snapshot of key-value pairs.
- When we require doing a full-table scan, we need to go through each file. This is because the same key can be found in multiple files even after compaction and the latest file is the most up-to-date value for the same key.
- Another critical point is that we also need to check for the in-memory memtable which is not yet written to the ss-table.
- As of now, the memtable and ss-table functions are structured to GET a single key.
- Both of them need to have a prefix-scan function. How will this be implemented?
- **SS-Table Prefix Scan**
    - Go from the newest file to the oldest file.
    - For each file:
        - Run a binary search on the index block to find the appropriate data block (Run a lower bound for `table_name:`).
        - Few reads from the data block might be wasteful. This is because the data block binary search would be based on the lower bound.
        - Note: We are applying binary search to find the appropriate data block but not applying binary search within the data block as of now. We should also apply binary search there.
        - Once one data block is done, move to the other one. Stop checking in the current data block and move to next file if you encounter a key where prefix is not equal to `table_name` prefix.
        - Keep on building each of the row (key, value pairs) in-memory in a map.
        - If a key is already present in the map, don't update it as we are going through the newest file first. Similarly, we would go over the memtable first as that has the newest data.
- **Memtable Prefix Scan** 
    - We should be able to utilise the **AscendGreaterOrEqual** function of the google/btree package.
    - Key here also would be `table_name:` and as soon as the prefix changes, we stop reading from the memtable.
- **Open Questions**:
    - Claude had highlighted in a session that certain databases use merge iterator which makes the time complexity O (N logK). Didn't understand it. Let's discuss if there are more performant ways of doing this.
- **Todo list:**
    - Tombstone handling and delete support.
    - We are building the entire result-set in-memory in map. What if there are billions of rows in a table?
    - **Merge iterator**: How can this solution improve our performance? Does this provide parallel reads and then merge instead of going sequential?
    - **Transaction isolation during scan**:
        - What happens if writes are going in parallel when we are running full-table scan? 

### Todo: Prefix approach vs Per-table SS-Table Instance
- Creating a separate ss-table could be a little bit more performant as we would not be requiring to go through index block and read few unnecessary keys at the boundary.
- Todo: Pros and cons of both the approaches would be looked at later.
- Focusing on implementing prefix approach for now as it is more inline with the current architecture.
    - Compaction isolation is one clear advanatge of per-table ss-table instance.
- We can optimise later if needed.

### Todo
- Need to look at failing tests
- How to efficiently store CHAR(14) and fixed length CHAR? 

## Query planner
- Query planner is going to be a very interesting thing to build. Estimate which direction would produce the most efficient result without actually executing the query.
- There can be multiple access paths to execute a query. 
    - How to return how many rows a query would return without actually estimating? Stats: But how does stats solve it?
    - Apart from storing data in tables, we would also be storing table level stats in some table.
    - Example one starting point for estimation could be: no. of rows in the table, and average cardinality for each column. Example: WHERE city = 'NYC'. estimated rows ==> 1k, avg column cardinality ==> 20 (20 distinct cities). estimated value of count of rows returned ==> 1k / 20 ==> 50.
    - Above is just a starting point to understand the concept. The above solution assumes uniform distribution (every city appears equally often).
- **Role of histogram:** 
    - A histogram breaks down the value range into buckets. Key --> value range, Value --> row count in that value range or bucket. age range --> count of rows in that age range.
    - [0 - 20] - 100 rows
    - [21 - 40] - 200 rows
    - [41 - 60] - 150 rows
    - age > 20 ==> sum last 2 buckets.
    - age >= 18 ==> do some estimations --> last 2 buckets + (2 / 20) of first bucket.
## Plan Order
Suggested order:
  1. Full-table scan (prefix approach) — unblocks everything else
  2. AND support — straightforward filter composition post-scan
  3. Secondary indexes — most interesting design challenge, teaches index maintenance on writes
  4. Range-based queries — needs index seek, connects back to storage layer
  5. Query planner — only becomes meaningful once you have multiple access paths (full scan vs index scan) to choose between
  6. Unique index, composite indexes, NULL support — refinements