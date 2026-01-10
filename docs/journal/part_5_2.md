## Making Application Thread Safe
* Why do we need thread safety? 
* Functions like Db.Put can be hit by hundreds of different applications hitting the same server concurrently.
* Example: if we expose a web server for our application, it needs to handle concurrent HTTP requests. Go's HTTP server spawns a new goroutine for each request.
* **Race Condition Problems In Put**:
    1. Memtable: Using google's B-tree which is not thread safe by default.
        * 2 goroutines modifying internal tree pointers simultaneously can lead to memory corruption, panics or lost data.
    2. Put and ShouldFlush when running in parallel
    Thread1: Put => size = 1000
    Thread2: Put => size = 1001
    Thread1: ShouldFlush => true
    Thread2: ShouldFlush => true
    --> flush happened twice
    3. WriteToWal
    T1: WriteToWal => a, 100
    T2: WriteToWal => b, 200
    T1: Put => b, 200
    ----- Process crash
    After process crash => a, 100 is also there but it was never in memtable.

```
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
```