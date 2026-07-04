# Building SaarDB, Part 1: Write-Ahead Log (WAL)

SaarDB is a learning project: build a relational database from first principles in Go, one layer at a time.

The goal is not to build a production database. The goal is to make database internals feel intuitive by building the core ideas directly and explaining the tradeoffs along the way.

When we think of a relational database, it is tempting to start with SQL parsing, tables, data types, and query execution. But that starts with the interface before understanding the storage layer underneath it.

So Part 1 starts smaller: a key-value store. From there, we will hit the first serious database problem: **how do we make writes survive a crash?**

## Where to Start: Why a Key-Value Store?

The initial instinct may be to start with the query layer. This includes SQL parsing, operations like `CREATE TABLE`, multiple data types, and so on. But the storage layer is the foundation beneath all of that.

The key insight is that:

> A key-value store can be extended to support everything a relational database needs.

This can feel surprising at first. How can a relational database, with tables and rows and SQL, start from something as small as key-value operations?

The reason for this is that every SQL operation can be mapped to a key-value operation. For example:
- `CREATE TABLE payments (...)` can be mapped to storing the schema as a value under key `_schema:payments`. The value can be stored in a serialized manner and can be deserialized when we need to read it.
- `INSERT INTO payments VALUES (1, 500, 'pending')` can be mapped to a PUT operation: `PUT payments:1 <serialized_row>`. This design also ensures efficient lookup by primary key.

If this mapping still feels abstract, that is fine. Later posts will come back to this when we build more of the SQL layer on top of the storage engine.

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
        args := strings.SplitN(line, " ", 3)
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

## The First Problem: Memory Is Volatile

If we kill the process and restart it, everything is gone. The hashmap lived in memory and memory does not survive process restarts.

> How do we make data survive crashes?

This is the problem of **durability**: guaranteeing that once a write succeeds, the data is permanently saved.

The answer is straightforward: write to disk. Memory is volatile. Disk is not. But this raises a new question.

## Choosing the Write Order

For every `PUT`, we need two updates:

1. Write to disk so the data survives a crash.
2. Write to the in-memory map so reads are fast.

The order matters.

If we update memory first, there is a dangerous window where memory contains data that disk does not. If the process crashes in that window, the update disappears after restart.

Even without a crash, if the disk write fails after the memory update, we now have to undo the memory change. That makes the write path more complicated.

If we write to disk first, the rule is simpler: only update memory after the durable write succeeds.

So the rule is: **write to disk first, then update memory.**

## Why Append-Only Writes Help

There is a golden rule for database performance:
> Sequential writes are significantly faster than random writes.

Let's go over what sequential and random writes actually are and why that is the case.

**Sequential write** means that while writing data, we write in such a way that the data is stored in adjacent blocks on the storage drive. 

On the other hand, **random write** means that the data is written in a scattered way across non-contiguous locations. 

For HDDs, random writes are slow because the disk head has to physically jump around. SSDs do not have a moving head, but the file-write pattern still matters.

Consider two updates to the same key:
```text
PUT user_1 Alice
PUT user_1 Bob
```

Suppose the file already contains:

```text
[user_1 = Alice]
```

Now we want to update `user_1` to `Bob`. If we overwrite the old value in place, we first have to find the old bytes, check whether the new value fits in the same space, and handle the case where it does not.

A simpler approach is to never rewrite old bytes. Just append the new value to the end of the file and treat the latest value as the source of truth:
```text
[user_1 = Alice]
[user_1 = Bob]
```

> The fastest possible write pattern is **append-only**: always add data to the end of the file, never go back to modify earlier data.

Yes, we store the key twice. That is wasted space. But the write is fast, and we know the latest value is always at the end. We will deal with the duplication problem later.

This append-only file is called a **Write-Ahead Log (WAL)**.

## Encoding WAL Entries

We need to write a command like `PUT user_1 Alice` to the file. The naive approach is to use a separator like a space or newline:

```text
PUT user_1 Alice\n
PUT user_2 Bob\n
```

This looks readable, but it hides two different problems.

### Problem 1: Where Does One WAL Record End?

Suppose the value contains a newline:

```text
PUT user_1 Alice
Bob
```

If our WAL reader treats newline as the end of an entry, it will think `Bob` is a separate command. So the first thing we need is a way to say: **this WAL record is exactly N bytes long.**

The solution is a length prefix around the whole WAL payload:

```text
[wal_payload_length][wal_payload]
```

Here is a concrete byte-level example. Suppose the WAL payload is `"PUT a 1"` (7 bytes):

```text
[00 00 00 07][50 55 54 20 61 20 31]
 └─ length ─┘└──── WAL payload ────┘
     = 7        P  U  T     a     1
```

The WAL reader knows: read 4 bytes for the length (7), then read exactly 7 bytes for the payload. This removes ambiguity about where this WAL record ends.

In plain English: the first 4 bytes say, "the next chunk is 7 bytes long."

That solves record boundaries. But it does not yet solve how to parse the command inside the payload.

### Problem 2: How Do We Parse the Command Inside the Payload?

If the payload itself is still a string like this:

```text
PUT user_1 Alice Bob
```

we are back to the same problem. Is the value `Alice`? Or is the value `Alice Bob`?

So the payload also needs length prefixes for its internal fields:

```text
[cmd_len][cmd][key_len][key][value_len][value]
```

For example, `PUT user_1 "Alice Bob"` becomes:

```text
[3][PUT][6][user_1][9][Alice Bob]
```

The exact bytes use 4-byte integers for the lengths, but conceptually this is the format. First we read the command, then the key, then the value. Spaces and newlines inside the value are just bytes; they do not confuse the parser.

In Go, the write flow looks like:

```go
func appendLengthPrefixedString(buf []byte, value string) []byte {
    buf = binary.BigEndian.AppendUint32(buf, uint32(len(value)))
    buf = append(buf, []byte(value)...)
    return buf
}

func serialisePutCommand(key, value string) []byte {
    buf := []byte{}
    buf = appendLengthPrefixedString(buf, "PUT")
    buf = appendLengthPrefixedString(buf, key)
    buf = appendLengthPrefixedString(buf, value)
    return buf
}
```

Think of `buf` as a growing byte slice: a resizable list of bytes. For each field, we first append its 4-byte length, then append the actual bytes.

And for reading one length-prefixed field:

```go
func readLengthPrefixedString(buf []byte, offset *int) (string, error) {
    valueLen := binary.BigEndian.Uint32(buf[*offset : *offset+4])
    *offset += 4

    value := string(buf[*offset : *offset+int(valueLen)])
    *offset += int(valueLen)
    return value, nil
}
```

Here, `offset` is just a cursor. It remembers where the next field starts inside the byte slice.

The important idea is that we use length prefixes at two levels:

1. The WAL layer frames the whole record.
2. The database command layer frames the fields inside that record.

This pattern of `[length][data]` shows up everywhere in storage systems. We will use it again in future blogs.

## Handling Partial Writes

Let's consider this scenario:

1. We start writing a 100-byte payload
2. The OS writes the 4-byte length header: `[00 00 00 64]`
3. The process crashes after writing only 50 bytes of the payload

The file now contains:
```text
...[valid entries]...[00 00 00 64][50 bytes of partial data]
```

This is where the length prefix helps. Because the payload length is written before the payload, the reader knows exactly how many bytes to expect:

```text
[wal_payload_length][wal_payload]
```

The writer side looks like this:

```go
func (w *Wal) WriteEntry(payload []byte) error {
    buf := make([]byte, 4+len(payload))
    binary.BigEndian.PutUint32(buf[0:4], uint32(len(payload)))
    copy(buf[4:], payload)

    _, err := w.file.Write(buf)
    return err
}
```

Here, `buf[0:4]` stores the length header. `buf[4:]` stores the payload right after it.

The reader uses the length to read exactly the expected number of bytes:

```go
func (w *Wal) ReadEntry() ([]byte, error) {
    lengthBuf := make([]byte, 4)
    _, err := io.ReadFull(w.file, lengthBuf)
    if err == io.EOF {
        return nil, io.EOF
    }
    if err == io.ErrUnexpectedEOF {
        return nil, errors.New("partial write: incomplete length")
    }
    if err != nil {
        return nil, err
    }

    payloadLength := binary.BigEndian.Uint32(lengthBuf)

    payload := make([]byte, payloadLength)
    _, err = io.ReadFull(w.file, payload)
    if err == io.ErrUnexpectedEOF {
        return nil, errors.New("partial write: incomplete payload")
    }
    if err != nil {
        return nil, err
    }

    return payload, nil
}
```

If the length says 100 bytes but only 50 bytes exist, `io.ReadFull` returns `io.ErrUnexpectedEOF`. That tells us the last WAL record was only partially written.

## Detecting Corrupted Bytes

A partial write is about missing bytes.

A corrupted write is different: all bytes may be present, but some bytes are wrong. This is not just a theory. Disks, SSDs, filesystems, OS crashes, power loss, and storage controllers can all fail in ways that leave bad bytes behind.

Most writes work fine. But databases are built for the cases where bytes are missing or wrong. The length prefix catches missing bytes. It does not prove that the bytes we read are the correct bytes.

> How does the reader know the bytes are correct?

A checksum solves this. A checksum is a small fingerprint computed from data. If the payload bytes are the same, the checksum will be the same. If even one byte changes accidentally, the checksum will most likely change.

For example, suppose the writer stores this payload:

```text
PUT user_1 Alice
```

The writer computes a checksum from these bytes and stores it next to the payload. Later, if the payload somehow becomes:

```text
PUT user_1 Aljce
```

the length may still be valid, but the checksum will likely be different. That tells us the payload bytes are not trustworthy.

Now we extend the same WAL record by adding a 4-byte checksum at the end:

```text
[wal_payload_length][wal_payload][checksum]
```

This problem is commonly solved in storage systems using CRC32, which is a fast checksum algorithm. The writer computes the CRC32 checksum over the payload bytes and appends it. The reader reads the payload, independently computes the checksum, and compares it with the stored checksum.

On the writer side, the length and payload logic stays the same. We only add space for the checksum, compute it from the payload, and append it:

```go
func (w *Wal) WriteEntry(payload []byte) error {
    buf := make([]byte, 4+len(payload)+4)

    // ...same length and payload writes as before...
    binary.BigEndian.PutUint32(buf[0:4], uint32(len(payload)))
    copy(buf[4:4+len(payload)], payload)

    // New part: append checksum after the payload.
    checksum := crc32.ChecksumIEEE(payload)
    binary.BigEndian.PutUint32(buf[4+len(payload):], checksum)

    _, err := w.file.Write(buf)
    return err
}
```

`buf[4+len(payload):]` means "start writing after the 4-byte length header and the payload." That is where the checksum goes.

With length prefixes and checksums together, the reader can handle these failure modes:
- **Incomplete length** (less than 4 bytes written): `io.ReadFull` returns `io.ErrUnexpectedEOF`. We know this is a partial write.
- **Incomplete payload**: Length says 100 bytes but only 50 exist. Same error.
- **Incomplete checksum**: Payload looks complete but checksum is truncated.
- **Corrupted data**: All bytes present but checksum does not match. Data is bad.

After the reader has already read the length and payload, the checksum check is the extra part:

```go
// ...same length and payload reads as before...

checksumBuf := make([]byte, 4)
_, err = io.ReadFull(w.file, checksumBuf)
if err == io.ErrUnexpectedEOF {
    return nil, errors.New("partial write: incomplete checksum")
}
if err != nil {
    return nil, err
}

storedChecksum := binary.BigEndian.Uint32(checksumBuf)
computedChecksum := crc32.ChecksumIEEE(payload)
if storedChecksum != computedChecksum {
    return nil, errors.New("corrupt: checksum mismatch")
}

return payload, nil
```

The intended recovery strategy is: if we detect a partial write at the **end** of the file, we can truncate it because it was the last write that did not complete. In the current implementation, we detect and return the error first; truncating the partial tail is a follow-up cleanup. On the other hand, if corruption is detected mid-file, that is a more serious problem (bit rot, hardware failure) that requires manual intervention.

## Making Writes Actually Reach Disk

There is one more subtlety. Consider this code:

```go
file.Write(buf)
// process reports "write successful"
// power goes out
```

Did the data actually reach the disk? **No, not necessarily.**

When we call `file.Write()`, the OS copies the data into a **page cache** (a buffer in RAM). The OS returns success immediately. It will eventually flush the page cache to the physical disk, but "eventually" might be seconds or minutes later. If power fails before that flush, the data is gone despite `Write()` returning success.

> `file.Write()` guarantees the data reached the OS. It does not guarantee the data reached the disk.

The solution is `fsync`:

```go
file.Write(buf)
file.Sync() // forces OS to flush page cache to physical disk
```

`file.Sync()` (which calls the `fsync` system call) blocks until the OS confirms the data is on the physical storage device. Only after `Sync()` returns, we can be confident that the data is durable.

There is a tradeoff though. `fsync` is expensive. On an HDD, it can take 5-10ms. On an SSD, it is faster but still significant. Calling it after every single write gives maximum durability but reduces throughput. Production databases like PostgreSQL batch multiple writes before a single `fsync`, trading a small window of vulnerability for much higher throughput.

For our database, we call `file.Sync()` after every WAL entry. This means maximum safety and simpler reasoning but with performance tradeoff.

## The Write Path

Now we can put it all together. When a user calls `PUT key value`:

1. **Serialize the command payload** into bytes: `[cmd_len][cmd][key_len][key][value_len][value]`
2. **Append it as a WAL record**: `[wal_payload_length][wal_payload][checksum]` and `fsync`
3. **Insert into the in-memory hashmap**

```go
func (db *DB) Put(key, value string) error {
    // Step 1-2: Write to WAL (disk first)
    walPayload := serialisePutCommand(key, value)
    if err := db.wal.WriteEntry(walPayload); err != nil {
        return err
    }
    // Step 3: Write to memory
    db.data[key] = value
    return nil
}
```

The WAL write happens first (disk before memory). If it fails, we return an error so that the user knows the write did not succeed. 
If the process crashes after the WAL write but before the in-memory map is updated, the data is safe on disk and will be recovered on restart.

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

        offset := 0
        cmd, err := readLengthPrefixedString(payload, &offset)
        if err != nil {
            return nil, err
        }
        if cmd == "PUT" {
            key, value, err := deserialisePutCommand(payload, &offset)
            if err != nil {
                return nil, err
            }
            data[key] = value
        }
    }
}
```

This is the **only time the WAL is read**. The loop reads entries from oldest to newest. 
For duplicate keys, the newest value naturally overwrites the older one in the hashmap, which is exactly the behavior we want.

After replay, the in-memory map contains the same state as before the crash. The database is ready to serve reads.

The full initialization sequence:
1. Open WAL file
2. Replay all WAL entries into a fresh in-memory map
3. Start accepting reads and writes

## What's Next

WAL gives us durability and fast writes (append-only, sequential). Reads go through the in-memory hashmap.

But we run into multiple issues at scale.
1. The WAL grows forever. Every update to the same key adds another entry, leading to a lot of redundant data.
2. An in-memory hashmap gives us O(1) lookups but cannot scale beyond RAM.
3. Replaying the entire WAL during application startup becomes painfully slow.

In Part 2, we will introduce LSM trees, which solve these problems by adding sorted in-memory structures and searchable disk files.
