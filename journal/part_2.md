## Logging Improvements
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("hello, world", "user", os.Getenv("USER"))

## Partial / Corruption Detection in WAL
### Partial Writes
* Process crashed mid-write. Only few bytes made it to disk.
* Wanted to write: PUT foo bar\n
* What got written: PUT foo b
* How to check? We can check by waiting for new line but that would mean that it get mixed. Example: PUT foo bPUT key value and cause *cascading failure*.
* Hence, we should utilise adding a prefix of length => [4 bytes: length][payload]

### Corrupted Writes
* Can be due to disk failure or hardware bugs.
    * Disk sector going bad.
    * RAM bit flip during write
    * "bit rot" over time on aging drives
* https://www.youtube.com/watch?v=izG7qT0EpBw
* We use Checksum for detecting corrupted writes
* CRC32 is a good standard for detecting corrupted writes.
* What to do? Store following: [4 bytes: length][payload][4 bytes: CRC32]
* *During reads*:
    * Read length.
    * If length doesn't match --> partial write
    * Get payload. Compute CRC32
    * Compare CRC32.
    * If doesn't match --> corruption --> skip OR abort
* **Question: to skip OR to abort**
* What to do if the CRC or length check fails
    * Option 1: Abort startup entirely
    * Option 2: My suggestion: If the check fails, we can still avoid aborting if PUT key value exists for the same key later when reading the WAL.
        * we can store a map[string]bool of keys which were corrupted.
        * keep on iterating this and updating.
        * even if a single key is corrupted abort.
    * I feel option 2 should be solution for an DB unless it is a cache instead where loosing out on some keys is fine. In case of cache we can just continue ahead and consider the key deleted.  
* **What real databases do in case of a crash**
    * **For corruption mid-WAL**: Everything before is trusted, Everything after is lost.
        * Throw error that manual intervention is needed.
    * **For corruption at the end of WAL**: This is due to a partial-write due to process crash
        * Truncate and continue
* *The idea of tracking corrupted key is not that useful because corruption can happen in the key also*
* so, based on that logic is simple, right? ==> check length, let's say it is N ==> check N characters. If there are less than N characters ==> partial write ==> truncate.
* After N characters, check the next 4 bytes. if they don't match ==> throw error and stop.
* any other case that I am not thinking?

* How to read specifically 4 bytes? How will we come to know of EOF with no. of bytes?