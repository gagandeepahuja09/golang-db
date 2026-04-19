## Motivation
- As a software engineer, I have always been interested in going 1 level deeper in understanding systems. Databases have been at the pinnacle of this. There is literally no end to what all we can learn in databases. It is kind of an engineering marvel. There are so many areas: ACID guarantees, SQL capabilities (building a query parser), JOINs, Indexes (primary, secondary and composite), Query planning, how data is stored for efficient reads or writes: B-trees vs LSM trees.  
- There are great books like DDIA and Database internals to understand those in greater detail. YouTube content creators like Hussein Nasser, Arpit Bhayani and Kaivalya Apte have also played a part in sparking that curosity around how to go in depth to understand these systems.
- As Arpit Bhayani says quite a lot, you learn the most when you are hands-on.
- If you go through this line from Richard Feynman multiple times, you will see that this still applies even in the LLM world: "What I cannot create, I do not understand."
- Learning is a slow and time consuming process and the best learnings happen when you don't set any deadlines.

## Learning Strategy
- I took an approach of AI assisted learning and not AI assisted development. CLAUDE.md takes care of ensuring that it is not spoon feeding me the implementation and just giving the required hints to solve a particular problem.

## Part 1: REPL With WAL
- When I started, my initial thoughts were to start with 

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