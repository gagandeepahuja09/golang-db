## Why do we need compaction [Problems solved by compaction]? 
- Read amplification: 
    - **High disk reads per user query**
    - Some old key might be present in the oldest file only and we would require going through each of the files.
    - There might also be deleted keys present.
- Space amplification: 
    - **High disk usage / logical data size**
    - Repeated keys take up unnecessary space.
    - Delete keys take up unnecessary space.

## Tradeoffs for compaction
- Since this happens in the background, we are competing for resources.
    - For compaction, we need to do both reads and writes.
    - Read L0 level, compact and then write L1 levels.
    - CPU (merging), Memory (buffer) and Disk I/O (reads and writes)
- Impacted metric 
    - Write Amplification
        **Total bytes written per user query**

## When will we do compaction?
- For simplicity, we can start doing compaction as soon as the L0 files count becomes 2.
- So, we can trigger the logic from Write function only based on certain condition shouldCompactSsTable.
- What if during the time compaction is going, a 3rd file comes up?
    - We will take an approach of doing compaction of the snapshot.
    - l0 -> f1, f2
    - l1 -> f1 (compacted from l0f1, l0f2) and l0f3
    - But how will ensure that we don't trigger another compaction when the 3rd file comes up?
        - We need a compacting boolean in our logic.

## What will be the appropriate time for file deletion after compaction?
- Readers will continue reading the old file. How will we ensure that they stop reading after we are done with the compaction?

## HLD

### Note: Everything for compaction will run in background (goroutine)
- **Todo for later**: Some systems also do flush in background with immutable memtable, but that's more complex.
- Note on concurrency. We need to take care of a lot of things in concurrency for compaction.
    - Files list
    - Running compaction boolean
- What are the shared variables in this struct?
```
type SsTable struct {
	dataFilesDirectory string //  remains static after initialisation
	firstLevelFiles    []*os.File // used by both compaction (R, W) and Get (R), Put (W)
	blockLength        int // remains static after initialisation
	indexBlocks        [][]indexBlockEntry // used by both compaction (R, W) and Get (R), Put (W)
	compacting         bool 
}
```

### Phase 1: Trigger (When to Trigger)
- After every flush to SS Table, we'll check the count of files. If files > 2 and no compaction ongoing, we will trigger one.
    - Instead of files > 2, let's keep it files > 4 ==> to avoid high write amplification. 
- We need to track "is_compaction_ongoing" in the SS table struct.
    - Does it need to be a shared variable (mutex required?)
    - **Todo before implementation** I don't think so as only one instance will be running at a time for our application as of now.
- **Todo for later**
    - There is also a concept of slowing down writes or completely stopping it if writes outpace the compaction.
    - Isn't there a point where we stop compaction so that file size doesn't become too large?

### Phase 2: Take a snapshot of files before starting compaction

### Phase 3: Merge [Read all files and create a common map]
- **Todo for later**: memory usage of this is high as all files need to be read in-memory.
    - How can we make it better?
    
### Phase 4: Write compacted file

### Phase 5: Atomic Swap Of Files Array And Indexes Array
- This solves one of the problem we faced and discussed earlier: "how will we come to know that now we can delete the old file as no one is reading it?"
- We will swap both the files array and the indexes array via a Write Lock.
    - In the get function, we introduce a read lock (RLock).
    - Writer waits until all readers release read lock.

### Phase 6: Delete old files


## Metrics [Todo: after V1 implementation]

