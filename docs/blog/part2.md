# Part 2: Memtable and SSTable
In the previous part, we discussed how Write Ahead Log (WAL) helps us solve durability.
But WAL alone is not enough to build a high-performance database.

The next major problem is:
> How do we make reads fast when the dataset becomes huge?

This is exactly where Memtables and SSTables come into the picture.

## Why WAL is not sufficient?

WAL is fundamentally an append-only log. This is excellent for durability and sequential disk writes, but eventually it creates two major problems.

### Problem 1: Repeat keys 
Imagine the following operations:
```text
PUT user_1 Alice
PUT user_2 Bob
PUT user_1 Charlie
PUT user_1 David
````

The WAL now contains multiple values for the same key.
```text
[user_1 -> Alice]
[user_2 -> Bob]
[user_1 -> Charlie]
[user_1 -> David]
```

But during reads, only the latest value matters.
```text
user_1 -> David
```

As updates increase, the WAL keeps growing while also accumulating stale values. Even if we periodically delete stale values, another much bigger problem still remains.

### Problem 2: Reads Become Extremely Expensive

Suppose we have 100 GB of WAL data on disk. Now imagine serving this query:
```text
GET user_987654
```

If the WAL is just an append-only file, we may need to scan a massive portion of the file sequentially.
```text
[user_1]
[user_2]
[user_3]
...
[user_987654]
```

This does not scale. The core issue is:
> WAL is optimized for writes, not for efficient search.

To serve fast reads, we need a structure that allows us to quickly narrow down where a key may exist. And the most common way to reduce search space efficiently is:
> Binary Search

But this leads to another interesting problem.

## The Real Challenge: How Do You Apply Binary Search On Disk?
Binary search is trivial in memory. Arrays support:
* direct indexing,
* constant-time jumps,
* cheap random access.

Disk files do not naturally behave like arrays. So the real challenge becomes:
> How do we structure data on disk such that binary search becomes possible?

This is exactly what LSM Trees solve using:
* Memtables
* SSTables

## Memtable: Fast In-Memory Writes
A Memtable is simply an in-memory sorted map. Whenever writes arrive:
1. They are appended to WAL for durability.
2. They are inserted into Memtable for fast reads/writes.

Because Memtable is sorted, keys are always maintained in order. Example:
```text
apple   -> 10
banana  -> 20
cat     -> 30
dog     -> 40
```

But we cannot keep growing Memtable forever because RAM is limited. So eventually:

> Memtable is flushed to disk as an SSTable.

## SSTable: Making Disk Reads Efficient
SSTable stands for `Sorted String Table`.
The key idea is simple:
> Store keys on disk in sorted order to allow efficient search.

But applying binary search directly on one huge disk file is still not straightforward. So SSTables organize data carefully.

### How SSTables Are Structured

Instead of storing one massive blob of sorted data, SSTables divide data into smaller blocks.Typically:
* each block is bounded by size (for example ~100 KB),
* keys inside each block remain sorted.

Example:

```text
Data Block 1:
[apple]
[banana]
[cat]

Data Block 2:
[dog]
[elephant]
[fox]

Data Block 3:
[monkey]
[tiger]
[zebra]
```

Notice something important:
```text
All keys in Block 2 > all keys in Block 1
All keys in Block 3 > all keys in Block 2
```
This ordering enables efficient lookup.

### The Index Block: The Real Enabler

Each SSTable also contains an Index Block. The index block stores:
* the starting key of every data block,
* the offset of that block inside the file.

Example:

```text
apple  -> offset 0
dog    -> offset 1024
monkey -> offset 2048
```

Now suppose we want:

```text
GET fox
```

We first load the index block into memory.

Then we binary search the block-start keys:

```text
apple, dog, monkey
```

The largest key less than or equal to `"fox"` is:

```text
dog
```

So we immediately know:

> If `"fox"` exists, it must exist in the block starting with `"dog"`.

Instead of searching the entire SSTable, we reduced the search space to a single block. This is the core idea behind SSTables. 

This leads us to the core lookup rule used in SSTables:
> Find the largest block-start key less than or equal to the input key.

### SSTable File Layout

A typical SSTable layout looks like this:

```text
+-------------+
| Data Block 1|
+-------------+
| Data Block 2|
+-------------+
| Data Block 3|
+-------------+
| Index Block |
+-------------+
| Footer      |
+-------------+
```

### Why Do We Need A Footer?

Interesting problem:
> How do we know where the index block starts?

The index block is usually near the end of the file. But during reads, we don't want to scan the entire file just to find it.

So SSTables store a small Footer Block at the end.
The footer typically contains:

* index block offset,
* metadata about the SSTable.

This allows readers to directly jump to the index block.

## Write Path
Now let's connect the intuition with implementation.

### Challenge: How Do We Flush Memtable To SSTable?

When Memtable reaches a threshold size:

* a new SSTable file is created,
* Memtable contents are written in sorted order,
* Memtable is reset. 
* WAL is also reset as the data is now durably stored in disk via SSTable.

Since Memtable is already sorted, SSTable creation becomes efficient.

### Writing Data Blocks

While flushing:

1. Memtable keys are iterated in sorted order.
2. Key-value pairs are written sequentially into data blocks.
3. Once block size threshold is reached, a new block is started.

A common encoding format while writing to disk is:

```text
[length_of_key][key][length_of_value][value]
```

Example:

```text
[5][apple][2][10]
```

Length prefixes are important because:
> keys and values are variable-sized

Indicating length of the key before the start of the key enables readers to know how many bytes to read.

While writing data blocks, we also maintain:

* block starting key,
* block offset.

These are later used to build the index block.

### Writing Strategy: Preserving Sequential Writes
One of the biggest reasons WAL performs well is that it relies on append-only sequential disk writes Interestingly, SSTable creation also preserves this exact property.

During Memtable flush, we create a new SSTable file and then sequentially write:
1. Data Blocks
2. Index Block
3. Footer Block

The overall file structure becomes:
```text
    [Data Blocks][Index Block][Footer Block]
```

Notice something important here:
> We never modify older parts of the file.

The SSTable is constructed entirely in an append-only fashion. This is extremely efficient because sequential disk writes are significantly cheaper than random writes.

In implementation terms, once the SSTable file is created and opened in append mode:

* data blocks,
* index block,
* footer block

are all written sequentially as byte arrays. So even though SSTables solve the problem of efficient reads, they still preserve the append-only write characteristics that made WAL efficient in the first place.

### Writing Index Block And Footer Block

While writing data blocks, we used the following encoding format:

```text
[length_of_key][key][length_of_value][value]
```

This encoding becomes important because:

* keys are variable-sized,
* values are variable-sized,
* readers must know exactly how many bytes to read from disk.

Without length prefixes, parsing the file correctly would become difficult.

The same principle applies while writing the index block.

Each index block entry stores:

1. the starting key of a data block,
2. the offset of that block within the SSTable file.

This leads to the following structure:

```text
[length_of_start_key][start_key][offset]
```

Notice an important distinction here.

The `start_key` is variable-sized, so we prefix it with its length.

However, `offset` is a fixed-size unsigned 32-bit integer.

Since the size of the offset is already known during reads, we do not need to prefix it with a length.

This leads to an important encoding rule commonly used in storage systems:

> Variable-sized data types usually require length prefixes.
>
> Fixed-sized data types usually do not.

The footer block follows the same idea.

Since the footer only stores the index block offset, its structure simply becomes:

```text
[index_offset]
```

During reads, this footer allows us to directly jump to the index block without scanning the entire SSTable.



## Read Path



### To be covered in future blogs
1. Size when memtable should be flushed to ss-table.
2. Data-block size.

### Implementation Details
#### Write Path
##### Memtable Write

##### Flush Memtable To SS-Table
- Code reference: `flushMemtableToSsTable` function.
- The memtable to ss-table flush happens on a periodic basis, depending on the size of memtable.
- During flush, the ss-table write is done in a new file basis the ss-table file structure we discussed.
- This involves iterating through each memtable key in sorted order and writing data blocks. The strategy for writing data blocks is to define the max data block size and once that size is reached, we complete the data block write. We maintain the index block in-memory during data block write such that it can be written after data block write.
- **Blocks writing strategy: "[Data block 1][Data block 2]...[Data block N][Index block][Footer block]"**
    - **Data Blocks**
        - While writing a single data block, we write each of the key value pairs sequentially into the file in the format: [length_of_key][key][length_of_value][value].
        - As we covered in the first part of the block, it is important to prefix with length of the key to understand the length of key during read before writing to disk.
        - While writing to data block, we also need to maintain the start key for each data block and offset for each start key which is important for writing index block.
    - **Index Block**
        - All of the details: pair of starting keys and offset for writing index block are maintained while writing to data blocks. 
    - **Footer Block**
        - Footer block is necessary to identify the index offset, so that during file read we can directly reach the index block rather than requiring to read the entire file.

#### Read Path
- During `GET key`, the first check is done within the memtable. If the key is not found in memtable, we check ss-table.
- **Which file to search?**
    - During ss-table read, it is important to realise that even while writing to ss-table, it is possible that there are repeat keys. This will majorly happen due to updates where `PUT key value` operation is fired multiple times for the same key.
    - Due to this, the same key can reside in multiple ss-table files and in such cases, we honour the key in the newest file and don't read the key from older files.
    - To solve for this, it is important to maintain a monotonically increasing file name during writes like `file_<id>` where id is auto-incrementing number. This means that file_100 was created after file_25.
    - Hence, the read strategy becomes to read from the newest file to the oldest file. As soon as we find a key, we stop the search.
    - **How to get all files?**
        - For simplicity, we can take an approach where we read all files to a specific directory and read the directory stats during application init. 
        - This means that during each flush to ss-table, we also should update the files list in-memory so that we don't need to fetch the list during every read.
- **Search within a file**
    - When reading a specific file, we need to find the index block. This requires going through the footer block, which is towards the end of the file, finding the index offset and then reading the entire index block in-memory. Rather than going through all of these steps sequentially during each read, we can precompute and store the index-block in-memory so that we avoid above repetitive read operation in each of the files during each of the reads.
    - Once we have index block in-memory,  

#### Application Init
- List of files.
- Index blocks and index offsets?

## Note
- Lots of intricate details around file writes.
    - Opening file in append-only mode during writing file.
- Pointed reads at a specific offset within a file.