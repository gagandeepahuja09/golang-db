
# Part 2: Memtable and SSTable

In the previous part, we discussed how Write Ahead Log (WAL) helps us solve durability.

But WAL alone is not enough to build a high-performance database.

The next major problem is:

> How do we make reads fast when the dataset becomes huge?

This is exactly where Memtables and SSTables come into the picture.

---

# Why WAL Alone Is Not Sufficient

WAL is fundamentally an append-only log.

This is excellent for durability and sequential disk writes, but eventually it creates two major problems.

## Problem 1: Repeat Keys

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

As updates increase, the WAL keeps growing while also accumulating stale values.

Even if we periodically delete stale values, another much bigger problem still remains.

---

## Problem 2: Reads Become Extremely Expensive

Suppose we have 100 GB of WAL data on disk.

Now imagine serving this query:

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

This does not scale.

The core issue is:

> WAL is optimized for writes, not for efficient search.

To serve fast reads, we need a structure that allows us to quickly narrow down where a key may exist.

And the most common way to reduce search space efficiently is:

> Binary Search

But this leads to another interesting problem.

---

# The Real Challenge: How Do You Apply Binary Search On Disk?

Binary search is trivial in memory.

Arrays support:

* direct indexing,
* constant-time jumps,
* cheap random access.

Disk files do not naturally behave like arrays.

So the real challenge becomes:

> How do we structure data on disk such that binary search becomes possible?

This is exactly what LSM Trees solve using:

* Memtables
* SSTables

---

# Mental Model

```text
Writes:
Client -> WAL -> Memtable -> SSTable

Reads:
Memtable -> Newest SSTable -> Older SSTables
```

---

# Memtable: Fast In-Memory Writes

A Memtable is simply an in-memory sorted map.

Whenever writes arrive:

1. They are appended to WAL for durability.
2. They are inserted into Memtable for fast reads/writes.

Because Memtable is sorted, keys are always maintained in order.

Example:

```text
apple   -> 10
banana  -> 20
cat     -> 30
dog     -> 40
```

But we cannot keep growing Memtable forever because RAM is limited.

So eventually:

> Memtable is flushed to disk as an SSTable.

---

# SSTable: Making Disk Reads Efficient

SSTable stands for:

> Sorted String Table

The key idea is simple:

> Store keys on disk in sorted order.

This immediately gives us a powerful property:

> Sorted data allows efficient search.

But applying binary search directly on one huge disk file is still not straightforward.

So SSTables organize data carefully.

---

# How SSTables Are Structured

Instead of storing one massive blob of sorted data, SSTables divide data into smaller blocks.

Typically:

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

---

# The Index Block: The Real Enabler

Each SSTable also contains an Index Block.

The index block stores:

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

Instead of searching the entire SSTable, we reduced the search space to a single block.

This is the core idea behind SSTables.

---

# SSTable File Layout

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

---

# Why Do We Need A Footer?

Interesting problem:

> How do we know where the index block starts?

The index block is usually near the end of the file.

But during reads, we don't want to scan the entire file just to find it.

So SSTables store a small Footer Block at the end.

The footer typically contains:

* index block offset,
* metadata about the SSTable.

This allows readers to directly jump to the index block.

---

# Write Path

Now let's connect the intuition with implementation.

---

# Challenge: How Do We Flush Memtable To SSTable?

When Memtable reaches a threshold size:

* a new SSTable file is created,
* Memtable contents are written in sorted order,
* Memtable is reset.

Since Memtable is already sorted, SSTable creation becomes efficient.

---

# Writing Data Blocks

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

---

# Writing Strategy

The overall write structure becomes:

```text
[Data Blocks][Index Block][Footer Block]
```

---

# How To Identify The Source Of Truth If The Same Key Exists In Multiple SSTables?

Suppose:

```text
PUT user_1 Alice
```

This gets flushed into SSTable 1.

Later:

```text
PUT user_1 Charlie
```

This may get flushed into SSTable 2.

Now the same key exists in multiple files.

Which one is correct?

```text
Newest value wins.
```

This is why reads search:

1. newest SSTable first,
2. then older SSTables.

As soon as a key is found, search stops.

---

# How Do We Identify Newer SSTables?

A simple strategy is using monotonically increasing file IDs.

Example:

```text
sst_1
sst_2
sst_3
```

If `user_1` exists in both:

* `sst_1`
* `sst_3`

Then value from `sst_3` is considered latest.

---

# Read Path

During:

```text
GET key
```

The lookup flow becomes:

```text
1. Search Memtable
2. Search newest SSTable
3. Search older SSTables
```

---

# Challenge: How Do We Search Efficiently Inside An SSTable?

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

---

# Searching Inside A File

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

---

# Application Startup

During application initialization, databases typically load:

* list of SSTable files,
* SSTable metadata,
* index block information.

This avoids repeated filesystem scans during reads.

---

# Problems Still Remaining

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

---

# Suggested Images To Add

## Image 1: WAL Sequential Scan Problem

Place after:

> "Reads Become Extremely Expensive"

Suggested diagram:

```text
WAL:
[k1][k2][k3][k4][k5]...[100GB]

GET(k999999)
         ^
Sequential scan required
```

---

## Image 2: Overall LSM Mental Model

Place after:

> "Mental Model"

Suggested diagram:

```text
Writes:
Client -> WAL -> Memtable -> SSTable

Reads:
Memtable -> SSTables
```

---

## Image 3: SSTable Layout

Place after:

> "SSTable File Layout"

Suggested diagram:

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

---

## Image 4: Index Block Binary Search

Place after:

> "The Index Block: The Real Enabler"

Suggested diagram:

```text
Index:
apple  -> Block 1
dog    -> Block 2
monkey -> Block 3

GET(fox)

Binary search:
fox belongs in Block 2
```

---

# GitHub Markdown Image Syntax

Store images like:

```text
/blog-images/
    sstable-layout.png
```

Then reference using:

```md
![SSTable Layout](./blog-images/sstable-layout.png)
```

---

# Recommended Tool For Diagrams

Best option:

[Excalidraw](https://excalidraw.com?utm_source=chatgpt.com)

It works extremely well for:

* storage systems,
* distributed systems,
* educational engineering blogs.
