# Making Application Thread Safe
* Why do we need thread safety? 
* Functions like Db.Put can be hit by hundreds of different applications hitting the same server concurrently.
* Example: if we expose a web server for our application, it needs to handle concurrent HTTP requests. Go's HTTP server spawns a new goroutine for each request.

## Race Conditions In Put
    
    func (db *DB) Put(key, value string) error {
        if err := db.writeToWal(key, value); err != nil {
            return errors.New("Something went wrong")
        }
        db.memTable.Put(key, value)

        if db.memTable.ShouldFlush() {
            if err := db.createSsTableAndClearWalAndMemTable(); err != nil {
                return err
            }
        }
        return nil
    }
1. **Memtable: Using google's B-tree which is not thread safe by default.**
    * 2 goroutines modifying internal tree pointers simultaneously can lead to memory corruption, panics or lost data.
2. **Put and ShouldFlush when running in parallel**
    ```
    Thread1: Put => size = 1000
    Thread2: Put => size = 1001
    Thread1: ShouldFlush => true
    Thread2: ShouldFlush => true
    --> flush happened twice
    ```
3. **WriteToWal and Put when running in parallel**
    ```
    T1: WriteToWal => a, 100
    T2: WriteToWal => b, 200
    T1: Put => b, 200
    ----- Process crash
        After process crash => a, 100 is also there but it was never in memtable.
    ```

## Golang Race Detector + Race Condition Test
* Wrote this test: db/concurrent_put_test.go and ran with `go test -race ./...`.
* Saw multiple WARNING DATA RACE in the output.

## Solving Race Condition
### Solution 1: Brute Force Sync.Mutex
- Coarse grained mutex.
- **todo:** On re-running the tests, the race conditions decreased from around 15 to 1 which is in compaction flow.

### Solution 2: Fine-grained Sync.RWMutex [Taken this approach for now]
- Use sync.RWMutex.
- `go test -race` also pointed out another race condition which I fixed.

### Solution 3: Single Writer, Lock-free Readers - Channel Based [Todo for later: Prove it by benchmarking]
- Serialize all writes to a single goroutine that owns the data.
- It is quite conter-intuitive that serializing makes it faster. Then it feel like what is the point of goroutines. We could have done that without doing anything for single-threaded programming languages with event loop?
- We still get the benefits of parallelism. Note that only the "writer" is single thread. What still happens in parallel or in the background:
    - Get (Reads) happen in parallel.
    - Parallel flush to ssTable. **todo**
    - Compaction happening in the background.
- So, the principle becomes:
    - **Use serialized writes in high lock contention**: This case. Shared data being written by multiple writers.
    - **Use mutex in low lock contention**
- We will also be using 
- **Why this wins?**

### Todo: Benchmarking
# What to run:
go test -bench=. -benchmem -cpu=1,4,8,16

# Key metrics to compare:

┌────────────────────────────────────────────────────────────────┐
│ Benchmark                          │ Option 2 │ Option 3      │
├────────────────────────────────────┼──────────┼───────────────┤
│ BenchmarkPut_SingleThread          │ Faster   │ Slightly slow │
│ BenchmarkPut_Parallel-16           │ Plateaus │ Scales up ✓   │
│ BenchmarkGet_WhileWriting-16       │ Slow     │ 10x faster ✓  │
│ BenchmarkMixed_80Read20Write-16    │ OK       │ Much better ✓ │
│ P99 latency under load             │ Spiky    │ Consistent ✓  │
│ BenchmarkPut_Batched               │ N/A      │ 50x faster ✓  │
└────────────────────────────────────────────────────────────────┘