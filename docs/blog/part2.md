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

Disk files do not naturally behave like arrays. The fundamental reason is that entries are variable-length. In an array, every element is the same size, so computing the position of the Nth element is trivial. But in a disk file storing key-value pairs, each entry can be a different size. Without reading and parsing every entry before it, there is no way to know where the Nth entry begins.

So the real challenge becomes:
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

Instead of storing one massive blob of sorted data, SSTables divide data into smaller blocks. Typically:
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

Once the correct block is found, entries within the block are scanned sequentially. Each entry is decoded using its length prefix: read the key length, read that many bytes for the key, read the value length, read that many bytes for the value, and repeat. This sequential scan within a single block is fast because blocks are small (typically ~100 KB).

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

Here is the core loop from our implementation (`sstable/sstable.go`) that writes data blocks:

```go
iteratorFunc(func(key, value string) {
    if blockFirstKey == "" {
        blockFirstKey = key
    }
    // [length_of_key][key][length_of_value][value]
    ssTableEntryBuf := []byte{}
    ssTableEntryBuf = binary.BigEndian.AppendUint32(ssTableEntryBuf, uint32(len(key)))
    ssTableEntryBuf = append(ssTableEntryBuf, []byte(key)...)
    ssTableEntryBuf = binary.BigEndian.AppendUint32(ssTableEntryBuf, uint32(len(value)))
    ssTableEntryBuf = append(ssTableEntryBuf, []byte(value)...)
    offset += len(ssTableEntryBuf)
    blockLength += len(ssTableEntryBuf)
    ssTableBlockBuf = append(ssTableBlockBuf, ssTableEntryBuf...)
    if blockLength > st.blockLength {
        // one data block completed
        indexBlock = append(indexBlock, indexBlockEntry{
            key:    blockFirstKey,
            offset: blockStartOffset,
        })
        file.Write(ssTableBlockBuf)

        // start new block
        blockStartOffset = offset
        blockFirstKey = ""
        blockLength = 0
        ssTableBlockBuf = []byte{}
    }
})
```

Notice how this single loop handles everything we discussed: length-prefix encoding each entry, accumulating entries into a block buffer, flushing to disk when the block size threshold is exceeded, and tracking the first key and offset for the index block.

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

### Problem: A Key Can Exist In Multiple Places
An interesting property of LSM Trees is that the same key can exist:
- in Memtable,
- in multiple SSTables,
- and potentially with different values.

This happens because updates do not immediately overwrite older values on disk.

For example:

```text
PUT user_1 Alice
PUT user_1 Charlie
PUT user_1 David
````

Older SSTables may still contain stale values:

```text
SSTable 1 -> user_1 = Alice
SSTable 2 -> user_1 = Charlie
Memtable  -> user_1 = David
```

During reads, only the latest value matters.

So while serving a `GET key` query, the database must search in an order that guarantees the newest value is found first.

This leads to the following mental model for reads:

```text
Memtable -> Newest SSTable -> Older SSTables
```

As soon as the key is found, the search stops.

### Challenge: How Do We Search Efficiently Inside An SSTable?

Naively reading:

* footer,
* then index block,
* during every query
would still create repeated disk reads.

So databases usually cache:
* index blocks,
* metadata,
* file handles
in memory.

This makes reads significantly faster.

### Searching Inside A File

Once index block is in memory:

1. Binary search index block.
2. Find candidate data block.
3. Read only that block from disk.
4. Search within that smaller block.

Important distinction:

> We are not binary searching the entire file.

We are:

* binary searching the in-memory index,
* then reading a much smaller disk block.

This dramatically reduces disk IO.

Add code references here and also show how we avoid reading the entire file by specifying the exact block we need to read.

## Application Startup
During application initialization, databases typically load:

* list of SSTable files,
* SSTable metadata,
* index block information.

This avoids repeated filesystem scans during reads.

Problems Still Remaining

At this point, we have solved:

* Durable writes via WAL
* Fast in-memory writes via Memtable
* Efficient disk lookups via SSTables

But major problems still remain.

As writes continue:

* SSTables keep increasing,
* duplicate keys accumulate,
* reads become slower because multiple files must be searched.

This leads us to:

* Compaction
* Multi-level LSM Trees
* Read Amplification

which we will cover in the next part.

Also, few more important topics within storage layer which will be covered later:
* Bloom filters within SSTable to easily come to know if a key definitely doesn't exist.
* Coming up with the appropriate flush size by evaluating tradeoffs.
* Coming up with the appropriate data block size within SSTable by evaluating tradeoffs.