## Motivation And Learning Philosophy
- As a software engineer, I have always been interested in going 1 level deeper in understanding systems. Databases have been at the pinnacle of this. There is literally no end to what all we can learn in databases. It is kind of an engineering marvel. There are so many areas: ACID guarantees, SQL capabilities (building a query parser), JOINs, Indexes (primary, secondary and composite), Query planning, how data is stored for efficient reads or writes: B-trees vs LSM trees.  
- There are great books like DDIA and Database internals to understand those in greater detail. YouTube content creators like Hussein Nasser, Arpit Bhayani and Kaivalya Apte have also played a part in sparking that curosity around how to go in depth to understand these systems.
- As Arpit Bhayani says quite a lot, you learn the most when you are hands-on.
- If you think about this line from Richard Feynman you will see that this still applies even in the LLM world: "What I cannot create, I do not understand."
- My personal opinion is that, learning is a slow and time consuming process and the best learnings happen when you don't set any deadlines. I have also heard a good line from a friend and resonate will with it: "Time does not factor in to good work. Even though the deadline has passed, even if the person that was supposed to be impressed by it is not impressed by it, if something is the right thing to do, it still is the right thing to do".
- The blog series serves two major purposes: 
    1. Be a guide for building databases for anyone not having much knowledge on databases. Even a college undergraduate should be able to understand, follow along get an understanding of the core concepts behind a database and if interested, build their own database.
    2. Important tool for me to revise the concepts as one learns the best by first building stuff and then explaining it to others.

## Learning Strategy
- I took an approach of AI assisted learning and not AI assisted development. An internal CLAUDE.md takes care of ensuring that it is not spoon feeding me the implementation and just giving the required hints to solve a particular problem. I also alway provide the approach that I am thinking or try and implement a basic approach rather than asking claude

## What is not covered
- The blog series is meant for learning purpose and not for writing production quality database. There would be many todos, few failing UTs, few edge cases missed sometimes unintentionally and sometime intentionally to just focus on the core implementation.

## Golang Code References
- 

## Where to start
- When I started, my initial thoughts were to start with focusing on the query layer first. This meant focusing on query planner, how will CREATE TABLE support different data types, how will the code be structured to focus on  etc. Then within my claude code session, claude recommended to focus on the storage layer first. 
- I didn't understand the rationale then but after having built early versions of the storage layer and the query layer, I realised that a key-value store can also be extended to support any operations required for a relational database at scale. We will see how the design is extensible in upcoming blogs.
- Here, I feel I should give more clearer reason on why and how is the design extensible?

## In-memory key-value store with a simple REPL application
- Whatever problem we are solving, we start with the brute force or the simplest change we can do.
- When building databases, correctness > performance and scale handling.
- So, we start with an in-memory key-value store support GET and PUT commands. Shortly after that, we build some sort of persistence layer.
- Even before thinking about persistence and durability, we start with just the in-memory layer which is relatively straightforward and can be built with a simple hashmap.
- The only new challenge which I encountered, which I was not much aware of, was around how to build the CLI via REPL.

### Building REPL CLI
- CLI tools are mostly REPL (Read-Eval-Print-Loop) which just means that you infinitely (via a for loop), keep on accepting input, processing / evaluating / executing it and then output the result on CLI till you encounter an EXIT based instruction or SIGTERM / SIGKILL like OS termination signals.
- In order to read STDIN line by line, Golang provides a package called bufio for various I/O capabilities.  
- The bufio package provides a data type or struct called bufio.Scanner for reading data through various sources like a file or STDIN line by line.
- Below code provides a high-level overview of taking input for the CLI.

```go
    func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		breakLoop := false
		switch cmd {
		case "GET":
			cmdGet(db, args)
		case "PUT":
            cmdPut(db, args)
        case "EXIT":
			breakLoop = true
		default:
			fmt.Println(CommandNotSupported)
		}
        if breakLoop {
			break
		}
    }
```

## Achieving Durability by Adding WAL

> What does durability mean?

Durability simply means that when we are firing PUT command, we are guaranteeing that the data is permanently saved. 


> How do we achieve durability?

In order to achieve durability, we need to write to disk also along with writing in-memory.

> What should be the order of write? Disk then in-memory or In-memory then disk?

If we write in-memory first and then to disk, it is possible that the application crashes immediately after the in-memory write. In such cases, during the application restart, we loose out on the data as it was never successfully written to disk. 
On the other hand, if we had written to disk first, the data can be recovered during application restart. This way we are maximising chances of recovering data during application restart.

Apart from crashes as well, writing to disk involves file I/O and that has much larger potential of failing compared to writing in-process. So, the common pattern is to do the harder or riskier or external writes first before internal writes. If we write in-memory first and then the file I/O breaks, we also need to rollback the in-memory write done. In-memory write can anyway only fail in case of a process crash. Hence, writing to disk first brings much more simplicity in our design.

> How should we be writing to database for optimal read and write performance?

There is always a tradeoff between read and write performance. We can either write data in a simple way that would help with write performance but instead impact read performance because of the simple way in which it is written.
Alternately we can write in some way optimised for read but it might impact write performance. We will come to a real example of this shortly in this blog.


> Optimising write performance to disk

There is a golden rule while reading from and writing to databases which is that sequential scans are much faster than random scans. 
*What are sequential and random writes*
- Sequential writes involve saving data continuously in adjacent blocks on a storage drive. In contrast, random writes involve small pieces of data in scattered, non-adjacent or non-contiguous locations across the drive. 

> Sequential writes via Append-only writes

Append-only writes are the fastest possible writes because they enable sequential writes. Append-only means that we keep on adding data towards the end of the file rather than continuously modifying data. 
This means that if we have updates like 1) PUT key1 value1   2) PUT key1 value2, we store these 2 operations as 2 separate commands separately in a sequential append-only way instead of either editing the first write or appending the new write and then deleting the older write. Both of the alternate solutions increase random writes.

> Atomicity

One single command. WAL also brings a level of simplicity. Easier to maintain atomicity that way.

> Importance of write-ahead log as a bin-log for replication

*details to be added*


We will now be shifting focus on how the read and write path look like for implementing the WAL layer and what additional changes would be required during application init.

## End-to-end flow
- During write, write to WAL first by appending the key value pair that needs to be written towards the end of the file. 
- During application bootup, utilise WAL to build the in-memory key value store. Read from the oldest to the newest key value pair. The order is critical
- No change in the 

We will now shift focus on detailing the write path and application init flows.

## Write Path
While introducing WAL code

- Significance of append only writes in performance for WAL over doing file I/O. We should write to the file in append-only mode and keep the file always open to ensure that there is only a single read during application startup.
- Importance of fsync in durability to ensure that the writes are always flushed to disk.
- Potential issues due to partial and corrupted write and how to solve for them by following writes in the format: [len][payload][checksum]  
- Effective file reads by reading byte-by-byte in above format.
- Todo: What to do in case of a corrupted or partial write and how to test for that?  
    * Reference: [./journal/part_1.md]
**Step 2:** 

## Application Init