# golang-db
Hobby project to learn database internals by building it in Golang. Started this repo with the aim of gradually adding database features to understand its core working.

What we have achieved so far:
Table on Notes and journal:
**Step 1:** 
    * What I Did: Built a simple REPL CLI application with in-memory GET and PUT operations along with WAL for durability. 
    * What I Learnt: 
        * bufio.Scanner for reading STDIN.
        * Significance of append only writes in performance for WAL over doing file I/O. We should write to the file in append-only mode and keep the file always open to ensure that there is only a single read during application startup.
        * Importance of fsync in durability to ensure that the writes are always flushed to disk.
        * Potential issues due to partial and corrupted write and how to solve for them by following writes in the format: [len][payload][checksum]  
        * Effective file reads by reading byte-by-byte in above format.
        * Todo: What to do in case of a corrupted or partial write and how to test for that?  
    * Reference: [./journal/part_1.md]
**Step 2:** 

