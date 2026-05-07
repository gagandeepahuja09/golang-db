# Part 2: Memtable and SSTable

## Why WAL is not sufficient?
1. **Repeat keys**: WAL is ever-growing in size. If there are frequent updates in the system, then there would also be a lot of **repeat keys** present in the WAL. In such cases, only the latest value of the key matters and all other values can be deleted. So, one optimisation becomes deleting stale values for keys. But what if there are very infrequent updates and mostly inserts? We can't solve the efficient reads problem via just periodic deletes.
2. **Inefficient (linear) search at scale**: If we wish to serve fast reads, we need to read from RAM and not disk. Even if we need to read from disk, we should have a way to read fast pointed queries. If we have 100 GB of data in disk, we shouldn't require going through each of the key value pair **sequentially**, hence in the worst case going through the entire 100 GB of data. The most common solve for avoid sequential reads is to have **sorted data** such that one can apply **binary search**. LSM trees solve exactly that via Memtable and Sstable.

## LSM Trees: Memtable And SSTable

### Intuition
- *Applying binary search on disk is not as straightforward as applying binary search on an array in-memory. The key aspect is to structure the data within a file such that one can apply binary search.*
- LSM trees which use Memtable and SSTable are one way of solving the problem of storing data efficiently for fast reads and writes.
- **What is a memtable?**: Memtable is nothing but a sorted map which is exactly where we keep our key value pairs in-memory before writing to the disk.
- So, the approach is that when the memtable reaches a particular size, the memtable is reset in-memory and flushed to disk. We cannot always keep writing to memtable and reading from memtable due to RAM limitations.
- When the memtable is written to disk, it should be written in a format which makes it possible to apply binary search on disk.
### Structured file-write to apply binary search
- To solve for this problem, we divide our file into **data blocks** and also have an **index block**.
- Index block is the main enabler for applying binary search.
- The list of key value pairs are written as fixed-size data blocks to disk. Let's take an example to understand better. Let's say that we have sorted keys from "1", "2", "3",  "4", ... till 1000. (value could be anything)
- Let's assume that this needs to be flushed to disk. We will first divide the keys into fixed size blocks. The logic usually for completing a data block is that it reaches a certain memory, let's say 100 kB.
- For our example, let's consider that we have data blocks as data block 1: [1 to 100] that is all keys from 1 to 100., data block 2: [101 to 200], data block 3: [201 to 300], ..., data block 10: [901 to 1000].
- Since we know that all data blocks are sorted within themselves and that all keys in data block N will be greater than all keys in data block N - 1, we can create an index block which just maintains the data block starting keys and their offsets.
- So, the structure of index block in this case becomes: [ first_key_for_data_block_1, offset_for_data_block_1, first_key_for_data_block_2, offset_for_data_block_2, ] 
- So, rather than trying to apply binary search within one big file, we have an index block which stores the first key of each of the data-block along with their offsets. So during reads we first go to the index block and pull the entire index block in-memory.
- While searching for a specific key, once we pull the index-block in memory, we can apply regular binary search to find 

### Implementation Details

#### Read Path

#### Write Path

Todo:
1. Size when memtable should be flused to ss-table.
2. Data-block size.