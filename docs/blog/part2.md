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
One of the biggest reasons WAL performs well is that it relies on append-only sequential disk writes. SSTable creation preserves this exact property. During a Memtable flush, we create a new file and write data blocks, then the index block, then the footer. All are written sequentially, never going back to modify earlier parts of the file:

```text
    [Data Blocks][Index Block][Footer Block]
```

So even though SSTables solve the problem of efficient reads, they still preserve the append-only write pattern that made WAL fast in the first place.

### Writing Index Block And Footer Block

Using the same length-prefix encoding from data blocks, each index block entry stores:

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

Once the index block is in memory, searching an SSTable for a key involves four steps:

**Step 1: Binary search the index block.** We use the same "largest key less than or equal to the input" rule from earlier.

**Step 2: Calculate the exact byte range to read.** Once we know which data block to search, we need to figure out: 
    * where it starts and 
    * ends on disk. 
The start offset comes directly from the matched index entry. The end offset comes from the *next* index entry's offset. Or if this is the last data block, the end offset is the start of the index block itself (which we already stored in memory during startup).

**Step 3: Read only that one block from disk.** This is the key insight. We use `file.ReadAt()` golang function to read exactly the bytes between the start and end offsets. We are not reading the entire file. We are not even reading from the beginning of the file. We jump directly to the relevant block.

**Step 4: Sequential scan within the block.** Once the block is in memory, we decode entries one by one using the length-prefix format. Which is to: 
    * read the key length, 
    * read that many bytes for the key, 
    * read the value length, 
    * read that many bytes for the value
    * and then check if the key matches.

Here is the `Get()` function from our implementation (`sstable/sstable.go`):

```go
func (st *SsTable) Get(key string) (string, error) {
	// newest file to oldest file
	for i := len(st.firstLevelFiles) - 1; i >= 0; i-- {
		file := st.firstLevelFiles[i]
		ssTableIndex := st.indexBlocks[i]
		lowerBoundSliceIndex := getLowerBound(key, ssTableIndex)
		if lowerBoundSliceIndex == -1 {
			continue
		}
		endOffset := st.indexOffsets[i]
		if lowerBoundSliceIndex < len(ssTableIndex)-1 {
			endOffset = ssTableIndex[lowerBoundSliceIndex+1].offset
		}
		value, err := st.getValueFromSsTableDataBlock(file, key,
			ssTableIndex[lowerBoundSliceIndex].offset, endOffset)
		if value == "" && err == nil {
			continue
		}
		return value, err
	}
	return "", nil
}
```

A few things to notice:

* **Reverse iteration** (`len - 1` down to `0`): Newest files are searched first so that the most recent value for a key is found first.
* **`getLowerBound` returning `-1`**: The key is smaller than every block-start key in this file. No point reading any block, so we `continue` to the next file.
* **`endOffset` calculation**: If the matched block is the last data block, the end offset defaults to `indexOffsets[i]`: the start of the index block. Otherwise, it uses the next index entry's offset. This gives us the precise byte range for the block we need to read.
* **`continue` on empty value with no error**: The key wasn't found in this file's candidate block, so we try the next (older) file.

If we exhaust all files without finding the key, `Get()` returns an empty string with no error — the key simply doesn't exist in the database.

## Application Startup

When the database process starts, it needs to reconstruct enough state to serve reads immediately. This happens in three steps:

**1. Scan the SSTable data directory.** The database reads all `.log` files from the data directory.

**2. Open all SSTable files.** File handles are opened once during startup and kept open for the lifetime of the process. This avoids the overhead of repeatedly opening and closing files on every read.

**3. Pre-build all index blocks into memory.** For each file, the database reads the footer (last 4 bytes) to get the index block offset, then reads the index block bytes from that offset to the footer, and parses them into in-memory structs. This is the `buildIndexes()` call in our implementation.

Here's `buildIndexFromFile`, which does the heavy lifting:

```go
func (st *SsTable) buildIndexFromFile(file *os.File) (int, []indexBlockEntry, error) {
	info, err := os.Stat(file.Name())
	if err != nil {
		return 0, nil, err
	}
	fileSize := info.Size()
	indexOffset, err := st.getIndexOffset(file)
	if err != nil {
		return 0, nil, err
	}

	indexBlockLength := (fileSize - 4) - int64(indexOffset)
	indexBlockBuf := make([]byte, indexBlockLength)
	if _, err = file.ReadAt(indexBlockBuf, int64(indexOffset)); err != nil {
		return 0, nil, err
	}

	ssTableIndex := []indexBlockEntry{}
	for i := 0; i < int(indexBlockLength); {
		keyLengthBuf := indexBlockBuf[i : i+4]
		keyLength := binary.BigEndian.Uint32(keyLengthBuf)

		i += 4
		key := string(indexBlockBuf[i : i+int(keyLength)])

		i += int(keyLength)
		offsetBuf := indexBlockBuf[i : i+4]
		offset := binary.BigEndian.Uint32(offsetBuf)

		ssTableIndex = append(ssTableIndex, indexBlockEntry{key: key, offset: int(offset)})
		i += 4
	}
	return int(indexOffset), ssTableIndex, nil
}
```

- **Footer read**: `getIndexOffset` reads the last 4 bytes to find where the index block starts.
- **Precise byte range**: reads exactly the bytes between `indexOffset` and `fileSize - 4`: no more, no less.
- **Same length-prefix decoding**: the loop mirrors the data block decoding pattern: 4-byte key length, key bytes, 4-byte offset and builds the in-memory index entry by entry.

After these three steps, the database is ready to serve reads. This is also why the `Get()` function shown earlier can binary search the index block without any disk IO. The index was already loaded into memory at startup. The only disk read during a query is for the single data block that might contain the key.

### Building Memtable from WAL on Restart

There is one more piece to the startup puzzle. SSTables contain data that was flushed from previous memtables, but what about writes that arrived *after* the last flush? Those exist only in the WAL.

On startup, the database replays the WAL to rebuild the current memtable. This is the `buildMemtableFromWal()` function in `db/db.go`. It reads every WAL entry from oldest to newest, parsing each command and inserting it into a fresh memtable. For duplicate keys, the newest value naturally overwrites the older one — exactly the behavior we need.

After this replay, the memtable contains all writes since the last SSTable flush. Combined with the SSTable index blocks loaded in the previous step, the database has the complete picture: recent writes in the memtable, older writes in SSTables, and index blocks in memory for fast lookups.

## What's Next

At this point, we have a working storage engine. WAL gives us durability. Memtable gives us fast in-memory writes with sorted ordering. SSTables give us efficient disk lookups using index blocks and binary search. And on startup, all the index blocks are pre-loaded so that reads only touch disk for the one data block that matters.

But several problems remain:

* **SSTable files keep growing.** Every Memtable flush creates a new file. Over time, the number of files on disk keeps increasing.
* **Duplicate keys accumulate across files.** If a key is updated 100 times, 100 different SSTable files may contain a value for it. Only the newest matters. The rest are wasted space.
* **Reads slow down.** Every `Get()` call may need to search through more and more files before finding the key (or confirming it doesn't exist).
* **Deletes don't exist yet.** In an append-only system, what does "delete" even mean? We can't go back and erase a key from an already-written SSTable file.

In upcoming parts, we will cover:
* **Compaction**: *to be covered in part 3*: merging multiple SSTables into fewer, larger ones, eliminating duplicates and reclaiming space.
* **Tombstones**: a way to represent deletes in an append-only world.
* **Bloom filters**: a probabilistic data structure that lets us skip SSTable files that definitely don't contain a key, without reading anything from them.
* **Tuning flush size and block size**: how large should the Memtable grow before flushing? How large should each data block be? These are not arbitrary choices and involve careful tradeoffs.
