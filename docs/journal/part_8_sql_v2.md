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

## Handling non-primary key based queries (Full-table scans) [V1 Done]
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

## Secondary Index
- While storing indexes, we would again utilise the Put command.
- Index would be updated both during INSERT and UPDATE queries.
- How do we ensure atomicity?
- We need to solve for both secondary index and composite indexes.
- Let's start with a simpler problem first: index on a single column.
- **How will we store the data?** 
    - One column value will be associated to multiple rows in the table. Hence this is a one-to-many-mapping.
    - We can store the data in a way such that the key itself gives us all the relevant data.
    - Key: `index:<table_name>:<column_name>:<column_value>:pk_value_1`, Value: can be empty.
    - Example: Index on city column.
    - index:city:NYC:id_1, index:city:NYC:id_2, index:city:NYC:id_5, ... 
- **How will we get the data during reads?**
    - Prefix scan on `index:<table_name>:<column_name>:<column_value>` --> extract all primary keys 
    - Do a get for each primary key to extract all the relevant 
    - Question: is this really optimal? Full-table scan instead has mostly sequential scans while this approach might have a lot of random seeks as well due to prefix scan + GET.
    - As per my understanding, this approach well but in skewed cases where an index value contributes to 50% of more of the rows, this approach would not scale well.
        - Query planner can maybe choose to not use index in such cases. 

### Inserts And Updates
- As of now, we have not yet solved for Updates. Updates are similar to delete + insert.
- We would have to implement delete functionality first by adding some sort of indicators called tombstones that a particular key (or row) is deleted. During compaction, such old entries can be removed.
- Both during inserts and updates, we would need to update the secondary index by issuing PUT commands.
- During update, we would need to remove the association with the old row by marking the old one as tombstone. 
- We would also have to handle for PK value itself getting updated as that is a "key" change itself.

### Index Metadata Storage
- We also need to store what all indexes are present in a table (both in DB and in-memory). 
- An `_index:<table_name>` can be added for it.
- Value can have all comma separated indexes.

### CREATE INDEX and updates in CREATE TABLE
- In case of CREATE INDEX, we would also need to backfill by running PUT queries in bulk.
- **Open Questions:**
    - What about DB updates that are happening during that backfill?
    - How do we ensure that they don't go stale?

### Do we even need atomicity?

### Todo:
- We have used : as a delemiter everywhere. We instead would need to move to binary serialisation and deserialisation.

### Composite Indexes
- The design would extend from secondary index
- `index:table_name:index_name:col1:col2:col3:col_value1:col_value2:col_value3:pk_id` --> where we would have separate key for all pk satisfying condition WHERE col1 = AND col2 = AND col3 = and all pk_ids satisfying that.
- Why column ordering matters in composite index?
- This is based on how the data or key is stored. col1 is used first, hence if a query doesn't have col1 in the query, the secondary index won't be used.  
- Following queries would work: 
    1. WHERE col1 = AND col2 = AND col3 = 
    2. WHERE col1 = AND col2 =
    3. WHERE col1
    4. WHERE col1 = AND col3 = (only use index for col1 and then filter for col3 during scan)
- But not following:
    1. WHERE col2 = 
    2. WHERE col3 = 

## Query planner
- Query planner is going to be a very interesting thing to build. Estimate which direction would produce the most efficient result without actually executing the query.
- **Design before query planner**: SQL string --> Parser --> AST --> Execute.
- **Design after query planner**: SQL string --> Parser --> AST `Planner --> Physical Plan` --> Execute.
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
- Let's take a query example: `WHERE col1 = 'abc' AND col2 > 20 AND col3 = 'def' AND col4 = 'ghi'`.
- Assume we have separate indexes on col1 and col2 and a composite index on col3 and col4.
- *Should we be taking a direction based on the estimate of no. of rows returned? The index which returns the estimate on least no. of rows should be returned?*
    - Fewer rows != Cheaper I/O.
    - Full-table scan: sequential I/O.
    - Index scan: N random seeks, one GET per PK.  (each seek traverses index block -> data block).
    - *So, how to estimate when index-scan would be more useful than a full-table scan?*
- We saw that cost is estimated on the basis of disk I/O but what all are the other factors which we should be aware of and think about apart from estimate of no. of rows?

### Yugabyte DB Article Insights
- https://www.yugabyte.com/blog/yugabytedb-cost-based-optimizer/#what-is-the-difference-between-cost-based-and-rule-based-optimizer 
- Rather than calculating the cost of each possible path, they take a dynamic programming based approach.
- They model the cost using a combination of seeks, nexts and previous.
- Seek: Operation to lookup a key.
- Next: Operation to fetch the next key.
- Prev: Operation to fetch the preceeding key.
- Does the next and preceeding key mean the keys from the prefix scan?
- Cost of seek is much higher compared to next and prev. Why?
    - Seek requires much more file I/O to go through multiple files, then the index block and then the data block.
- How to calculate the cost of each seek, next and prev?

### Full-table scan vs Index scan
- **Full-table scan**: B blocks read --> [1 seek + (B - 1) nexts]. 1 seek for the prefix scan on the table_name key
- **Index only scan**: R rows --> [1 seek + (R - 1) nexts]. 1 seek for the prefix scan on the index key.
- **Index scan**: [1 seek + (R - 1) nexts + R seeks] (R additional seeks for each row for running GET for each)
    - (R + 1) seeks + (R - 1) nexts
- Hence, if we can find a ratio between seek and next (let's call it seek_multiplier = seek / next), we would be able to effectively estimate costs. This ratio is found by carrying out some benchmarking. This ratio is quite hardware dependent as the ratio of random scan latency to sequential scan latency is much higher in case of HDD compared SSDs and NVMe.
- We can consider next --> as sequential scan and seek --> as random scan.

### Seek-multiplier benchmarking
- We need to carry out some benchmarking to find out what should be the value of seek-multiplier.
- **What operation in the system isolates a single random step? [Seek Cost]**
    - GET key
- **What operation in the system isolates a single sequential step? [Next Cost]**
    - Total cost - seek cost / N where N is no. of keys read during sequential scan.  

**Benchmark Plan**
- **Seek Cost** => Measure N individual GET on random keys (total_time / N)
    - If we read them sequentially, then the OS read-ahead will pre-fetch blocks and we will accidentally measure sequential I/O.
        - When we read block 5 sequentially, OS predicts that we will read block 6 next.
    - In order to ensure random behaviour, keep all the keys in an array and use random number generator of identify a random index to pick the key which we need to GET.
- **Next Cost** ==> Measure full-table scan. full_scan_time = seek_cost + ((N - 1) * next_cost)
    - next_cost = (full_scan_time - seek_cost) / (N - 1)
- **Benchmark Code**
    - We need to create table and insert N rows first. We will not keep this section in `for b.Loop` so that this is not part of benchmark result.
    - How will we numerically get the seek cost and next cost as result? Basis what I know, Golang benchmark provides a final output only which is function specific. How to modify for our use case such that next cost and seek cost are directly in the output?
- **Dataset size**
    - OS has a page cache. If the dataset fits in memory, every "random seek" is served from RAM leading to seek multiplier ratio closer to 1.
    - We should also test for varying access patterns: high cache misses, high cache hits and mixed workloads.
- **Docker container approach**
    - This approach is necessary and quite useful in setting limitations on RAM. Rather than exhausting the 8 or 16 GB RAM limitation of the system, we will set up a docker container. We can set the memory limitation by setting the --memory flag.
    - What we can do is: write 60 MB of data.
        - high cache hits: 64 MB of RAM
        - high cache misses: 8 or 16 MB of RAM
        - mixed workloads: 32 MB of RAM
    - This also shows that the seek multiplier is a factor of RAM as well apart from the hardware. This again proves the importance of seek ratio being a tunable parameter as is the case in Postgres.
    - Docker container approach also helps with busting the cache in between runs. This is necessary because after a full-table scan, everything is in page cache and hence we are unknowingly measuring cache hits and not disk seeks. 
        - Run each benchmark in a separate container run. OR
        - Call `echo 3 > /proc/sys/vm/drop_caches` between benchmarks.
- **Benchmark data must be in SSTable**
    - Since the flush default limit is only 1kB, almost all of the data would reside in SSTable for the benchmark. 

### Enumerating all access paths

### Stats storage
- When should we update the stats related tables? Updating on each and every write would impact disk I/O, reads and writes performance unnecessarily.
- Updating them during flush to sstable makes the most sense.
- We can keep maintaining the stats in-memory.

#### Counter Stats
- Should we be storing the count aggregate for each value? If not, we would have to consider uniform distribution? Can we take some other approach?

#### Histogram Stats
- Only to be stored for numeric data-types.

### Resources
- Postgres: https://www.postgresql.org/docs/current/runtime-config-query.html — search for random_page_cost. The official docs explain what each cost parameter means and what values to set for SSDs.                    
- https://wiki.postgresql.org/wiki/Tuning_Your_PostgreSQL_Server  community wiki with practical guidance on tuning cost parameters including the SSD recommendation.                                       
- YugabyteDB: https://www.yugabyte.com/blog/yugabytedb-cost-based-optimizer/: read on how they assign weights
- MySQL: https://dev.mysql.com/doc/refman/8.0/en/cost-model.html — MySQL has an explicit cost model table stored in mysql.server_cost and mysql.engine_cost that you can actually query and modify. Much more transparent than Postgres.
- CMU Database Course: Andy Pavlo's 15-445 lectures on query planning are freely available at https://15445.cs.cmu.edu. Lecture on "Cost Models" covers this rigorously with actual numbers and derivations.

## Plan Order
Suggested order:
  1. Full-table scan (prefix approach) — unblocks everything else [Done]
  2. AND support — straightforward filter composition post-scan
  3. Secondary indexes — most interesting design challenge, teaches index maintenance on writes [Plan mostly done, Execution pending]
  4. Range-based queries — needs index seek, connects back to storage layer
  5. Query planner — only becomes meaningful once you have multiple access paths (full scan vs index scan) to choose between [Plan WIP]
  6. Unique index, composite indexes, NULL support — refinements