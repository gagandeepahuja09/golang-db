# Part 1: WAL

## Motivation and Learning Philosophy

As a software engineer, I have always been interested in going one level deeper in understanding systems and was most curious with deep-diving into database internals as a backend engineer. My preference was building hands-on rather than trying to read books to understand. 

AI assisted learning helped fast track my learning process and keep me more engaged. I used Codex / Claude as a learning companion. I ask for hints, explanations and reviews, not for code. An internal `AGENTS.md` ensures it doesn't spoon-feed implementations.

This blog series serves two purposes:
1. Be a guide for anyone building a database from scratch. Even a college undergraduate should be able to follow along and understand the core concepts.
2. Be a revision tool for me. We learn best by first building and then explaining to others.

**What is not covered:** This series is for learning, not for writing a production database. Within the github repo, you will some TODOs, some missed edge cases, and some simplifications made to focus on the core ideas.

## What this series covers?

The series documents my learnings from building a relational database purely for learning purpose.

## Where to Start: Why a Key-Value Store?

My initial instinct was to start with the query layer. This includes supporing SQL parsing, operations like CREATE TABLE, multiple data types and so on. I presented this plan to Claude and got some push back basis my plan and prompt that I should focus on the storage layer first.

The key insight is that:

> A key-value store can be extended to support everything a relational database needs.

I had heard about this earlier before starting this project also but didn't really understand why and how is that possible.

The reason for this is that every SQL operation can be mapped to a key-value operation. For example:
- `CREATE TABLE payments (...)` can be mapped to storing the schema as a value under key `_schema:payments`. The value can be stored in a seralised manner and can be deserialised when we need to read it.
- `INSERT INTO payments VALUES (1, 500, 'pending')` can be mapped to a PUT operation: `PUT payments:1 <serialized_row>`. This design also ensures efficient lookup by primary key.

Even if the rationale doesn't make sense, don't worry. We will deep-dive again on this from the 5th blog of the series. By that time, this will make much more sense.

## Starting Simple: An In-Memory Key-Value Store

Whatever problem we are solving, we start with the simplest thing that works.

> Correctness before performance. Performance before scale.

The simplest database is a hashmap in memory to support GET and PUT commands. We will build a REPL (Read-Eval-Print-Loop) around it so users can interact via the command line.

### Building the REPL

A REPL is just a loop: accept input, process it, print the result, repeat. In Go, `bufio.Scanner` reads from STDIN line by line:

```go
func main() {
    db := map[string]string{}
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        line := scanner.Text()
        args := strings.Split(line, " ")
        cmd := args[0]
        switch cmd {
        case "GET":
            fmt.Println(db[args[1]])
        case "PUT":
            db[args[1]] = args[2]
        case "EXIT":
            return
        default:
            fmt.Println("command not supported")
        }
    }
}
```

This works. We can store and retrieve data. But there is an obvious problem.

## Problem: The Process Crashes, Data Is Gone

If we kill the process and restart it, everything is gone. The hashmap lived in memory and memory does not survive process restarts.

> How do we make data survive crashes?

This is the problem of **durability**: guaranteeing that once a write succeeds, the data is permanently saved.

The answer is straightforward: write to disk. Memory is volatile. Disk is not. But this raises a new question.

## Problem: Should We Write Disk-First or Memory-First? [todo]

We need to write to both memory (for fast reads) and disk (for durability). What should the order be?

> If we write in-memory first and then to disk, what happens if the process crashes between the two?

The memory write succeeds. The disk write never happens. On restart, the data is lost. We told the user "write successful" but the data is gone.

Now consider the reverse: write to disk first and then in-memory.

If the disk write succeeds but the process crashes before the memory write, the data is still on disk. On restart, we can easily recover it and no data is lost.

There is a deeper reason too. Disk I/O has a much larger potential of failing compared to writing in-process memory. The common pattern in systems design is: **do the harder or riskier operation first.** If the disk write fails, we haven't touched memory yet — nothing to roll back. If memory allocation fails, well, the process is probably crashing anyway.

So the rule is: **always write to disk first and then memory.**

## Problem: How Should We Write to Disk?

There is a golden rule for database performance 
> Sequential writes are significantly faster than random writes.

Let's go over what sequential and random writes actually are and why that is the case.

**Sequential write** means that while writing data, we write in such a way that the data is stored in adjacent blocks on the storage drive. 

On the other hand, **random write** means that the data is written in a scattered way across non-contiguous locations. 

In case of random writes, the disk head has to jump around which makes it much slower than sequential writes where the disk head doesn't have to jump around and progessively moves through contiguous locations.

Let's take an example to understand this in greater detail.

Consider two updates to the same key:
```text
PUT user_1 Alice
PUT user_1 Bob
```

We could try to find and overwrite the first entry. But that is a random write, we have to seek to the location of `user_1`, figure out if the new value fits in the same space, and handle the case where it doesn't.

Instead, we can just append both entries and consider the latest value for the key as the source of truth:
```text
[user_1 = Alice]
[user_1 = Bob]
```

> The fastest possible write pattern is **append-only**: always add data to the end of the file, never go back to modify earlier data.

Yes, we store the key twice. That is wasted space. But the write is fast, and we know the latest value is always at the end. We will deal with the duplication problem later.

This append-only file is called a **Write-Ahead Log (WAL)**.

## Problem: How Do We Encode Entries in the File?

We need to write `PUT key value` to the file. The naive approach is to use a separator like a space or newline:

```text
PUT user_1 Alice\n
PUT user_2 Bob\n
```

But what if a key or value contains a space or a newline? The reader would then split in the wrong place and corrupt all subsequent reads.

> We need an encoding that works regardless of what characters the key or value contains.

The solution is **binary serialization with length prefixes**. Instead of separating fields with special characters, we prefix each field with its length in bytes:

```text
[length_of_payload][payload]
```

Where payload is the string `"PUT user_1 Alice"`.

Here is a concrete byte-level example. Suppose the payload is `"PUT a 1"` (7 bytes):

```text
[00 00 00 07][50 55 54 20 61 20 31]
 └─ length ─┘└────── payload ──────┘
     = 7        P  U  T     a     1
```

The reader knows: read 4 bytes for the length (7), then read exactly 7 bytes for the payload. No ambiguity, no matter what the payload contains.

In Go, the write flow looks like:

```go
buf := make([]byte, 4+len(payload)) // add short and easy to understand comments for each line
binary.BigEndian.PutUint32(buf[0:4], uint32(len(payload)))
copy(buf[4:], payload)
file.Write(buf)
```

And for reading, we use `io.ReadFull` to read exactly N bytes:

```go
lengthBuf := make([]byte, 4)
io.ReadFull(file, lengthBuf)
payloadLength := binary.BigEndian.Uint32(lengthBuf)

payload := make([]byte, payloadLength)
io.ReadFull(file, payload)
```

This pattern of `[length][data]` shows up everywhere in storage systems. We will use it again in future blogs for SSTable data blocks, index blocks, and transaction payloads.

## Problem: What If the Process Crashes Mid-Write?

Our WAL writes `[length][payload]` for each entry. Now consider this scenario:

1. We start writing a 100-byte payload
2. The OS writes the 4-byte length header: `[00 00 00 64]`
3. The process crashes after writing only 50 bytes of the payload

The file now contains:
```text
...[valid entries]...[00 00 00 64][50 bytes of partial data]
```

On restart, the reader reads the length (100), tries to read 100 bytes, but only 50 exist, hence it reads 50 additional bytes which is garbage and corrupts any further data.

Above scenario is considered a partial write. But apart from partial writes, corrupted writes are also possible. Corrupted write is the case when some of the bytes are wrongly written.

todo: Add a short 1 or 2 liner telling that why and how these kind of things are possible in a hardware.

> How does the reader distinguish a valid entry from a corrupt or partial one?

The solution: add a **checksum** after the payload.

```text
[length][payload][checksum]
```

This problem is commonly solved in distributed systems using CRC32, which is a fast hash that produces a 4-byte fingerprint of the payload. The writer computes the checksum over the payload bytes and appends it. The reader reads the payload, independently computes the checksum, and compares (todo: add some intuition sort of thing about this CRC32):

```go
// Writer
func (w *Wal) WriteEntry(payload []byte) error {
    buf := make([]byte, 4+len(payload)+4)
    checksum := crc32.ChecksumIEEE(payload)
    binary.BigEndian.PutUint32(buf[0:4], uint32(len(payload)))
    copy(buf[4:4+len(payload)], payload)
    binary.BigEndian.PutUint32(buf[4+len(payload):], checksum)
    if _, err := w.file.Write(buf); err != nil {
        return err
    }
    return w.file.Sync()
}
```

Now the reader can handle every failure mode:
- **Incomplete length** (less than 4 bytes written): `io.ReadFull` returns `io.ErrUnexpectedEOF`. We know this is a partial write.
- **Incomplete payload**: Length says 100 bytes but only 50 exist. Same error.
- **Incomplete checksum**: Payload looks complete but checksum is truncated.
- **Corrupted data**: All bytes present but checksum does not match. Data is bad.

```go
func (w *Wal) ReadEntry() ([]byte, error) {
    // 1. Read length
    lengthBuf := make([]byte, 4)
    _, err := io.ReadFull(w.file, lengthBuf)
    if err == io.EOF { return nil, io.EOF }
    if err == io.ErrUnexpectedEOF {
        return nil, errors.New("partial write: incomplete length")
    }

    payloadLength := binary.BigEndian.Uint32(lengthBuf)

    // 2. Read payload
    payload := make([]byte, payloadLength)
    _, err = io.ReadFull(w.file, payload)
    if err == io.ErrUnexpectedEOF {
        return nil, errors.New("partial write: incomplete payload")
    }

    // 3. Read and verify checksum
    checksumBuf := make([]byte, 4)
    _, err = io.ReadFull(w.file, checksumBuf)
    if err == io.ErrUnexpectedEOF {
        return nil, errors.New("partial write: incomplete checksum")
    }

    storedChecksum := binary.BigEndian.Uint32(checksumBuf)
    computedChecksum := crc32.ChecksumIEEE(payload)
    if storedChecksum != computedChecksum {
        return nil, errors.New("corrupt: checksum mismatch")
    }

    return payload, nil
}
```

The recovery strategy: if we detect a partial write at the **end** of the file, we truncate it as it was the last write that did not complete. On the other hand, if corruption is detected mid-file, that is a more serious problem (bit rot, hardware failure) that requires manual intervention.

## Problem: The OS Lies About Writes

There is one more subtlety. Consider this code:

```go
file.Write(buf)
// process reports "write successful"
// power goes out
```

Did the data actually reach the disk? **No, not necessarily.**

When you call `file.Write()`, the OS copies your data into a **page cache** (a buffer in RAM). The OS returns success immediately. It will eventually flush the page cache to the physical disk, but "eventually" might be seconds or minutes later. If power fails before that flush, the data is gone despite `Write()` returning success.

> `file.Write()` guarantees the data reached the OS. It does not guarantee the data reached the disk.

The solution is `fsync`:

```go
file.Write(buf)
file.Sync() // forces OS to flush page cache to physical disk
```

`file.Sync()` (which calls the `fsync` system call) blocks until the OS confirms the data is on the physical storage device. Only after `Sync()` returns can we be confident the data is durable.

There is a tradeoff though. `fsync` is expensive. On an HDD, it can take 5-10ms. On an SSD, it is faster but still significant. Calling it after every single write gives maximum durability but reduces throughput. Production databases like PostgreSQL batch multiple writes before a single `fsync`, trading a small window of vulnerability for much higher throughput.

For our database, we call `file.Sync()` after every WAL entry. Maximum safety, simpler reasoning but with performance tradeoff.

## The Write Path

Now we can put it all together. When a user calls `PUT key value`:

1. **Serialize the command** into bytes: `[length][payload][checksum]`
2. **Append to WAL file** and `fsync`
3. **Insert into the in-memory hashmap**

```go
func (db *DB) Put(key, value string) error {
    // Step 1-2: Write to WAL (disk first)
    buf := serialiseCommand("PUT", fmt.Sprintf("PUT %s %s", key, value))
    if err := db.wal.WriteEntry(buf); err != nil {
        return err
    }
    // Step 3: Write to memory
    db.data[key] = value
    return nil
}
```

The WAL write happens first (disk before memory). If it fails, we return an error — the user knows the write did not succeed. If the process crashes after the WAL write but before the in-memory map is updated, the data is safe on disk and will be recovered on restart.

## The Read Path

Reads are simple right now. We just look up the in-memory map:

```go
func (db *DB) Get(key string) (string, error) {
    value, ok := db.data[key]
    if !ok {
        return "", nil // key not found
    }
    return value, nil
}
```

The WAL is **never read during normal operation**. It is purely a durability mechanism for writes. The in-memory map serves all reads.

## Application Init: Rebuilding State from WAL

When the database process starts, the in-memory hashmap is empty. But the WAL on disk contains every write since the last checkpoint. We need to replay it:

```go
func (db *DB) buildInMemoryMapFromWal() (map[string]string, error) {
    data := map[string]string{}
    for {
        payload, err := db.wal.ReadEntry()
        if err == io.EOF {
            return data, nil // reached end of WAL
        }
        if err != nil {
            return nil, err // corruption or partial write
        }
        // parse and replay the PUT command
        handlePutCmd(data, string(payload))
    }
}
```

This is the **only time the WAL is read**. The loop reads entries oldest to newest. For duplicate keys, the newest value naturally overwrites the older one in the hashmap — exactly the behavior we want.

After replay, the in-memory map contains the same state as before the crash. The database is ready to serve reads.

The full initialization sequence:
1. Open WAL file
2. Replay all WAL entries into a fresh in-memory map
3. Start accepting reads and writes

## What's Next

WAL gives us durability. Writes are fast (append-only, sequential). Reads go through the in-memory hashmap.

But there is a problem brewing. The WAL grows forever. Every update to the same key adds another entry leading to a lot of redundant data. And the in-memory hashmap gives us O(1) lookups but cannot scale beyond RAM.

> What happens when the dataset exceeds memory? What happens when we need to read from disk efficiently?

Right now, a "read from disk" means replaying the entire WAL — that is O(N) where N is every write ever made. As data grows, this becomes painfully slow.

In Part 2, we will evolve this hashmap into a sorted in-memory structure and introduce **SSTables** (sorted disk files) — the core of an LSM tree. These let us flush memory to disk periodically and search disk files efficiently using binary search.
