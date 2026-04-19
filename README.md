# golang-db

### Building an embedded database from first principles to maximize learning.
I'm building this to understand the "magic" under the hood of a database. No shortcuts, no pre-built engines. This is just Go and a lot of curiosity. I use LLMs as reviewers and guides, providing them with my detailed thought process and where I'm stuck, rather than asking for code to copy-paste. This is the power of learning without deadlines.

### Getting Started

Since I'm writing everything myself, you just need Go installed to spin it up.

```Bash

# Clone the repo  
git clone https://github.com/gagandeepahuja09/golang-db

# Run the database  
go run main.go
```

### Usage Examples

The query layer currently handles basic SQL-like syntax.

```SQL

-- Datatypes supported: INT, STRING, BOOL

-- Create a new table  
CREATE TABLE users (id INT, name STRING);

-- Insert data (goes through WAL and Memtable)  
INSERT INTO users VALUES (1, 'Gagan');

-- Retrieve data (searches Memtable then SSTables)  
SELECT * FROM users WHERE id = 1;

```

### Feature Checklist

I'm progressively ticking these off as I dive deeper into the internals.

* **Storage Layer (LSM Engine)**  
  * \[x\] Write-Ahead Log (WAL) for crash recovery 
  * \[x\] In-memory Memtable (Sorted ingestion) 
  * \[x\] Periodic flush to immutable SSTables
  * \[x\] Background Compaction with Manifests
  * \[ \] Coming up with appropriate memtable flush size
  * \[ \] Benchmarking for performance improvements
  * \[ \] Bloom Filters (Read optimization)  
* **Transaction Layer (ACID)**  
  * \[x\] 2-Phase Locking (2PL) for serializability 
  * \[ \] MVCC (Multi-Version Concurrency Control) 
  * \[ \] Support for multiple isolation levels  
* **Query Layer (Relational Interface)**  
  * \[x\] CREATE, INSERT, SELECT support  
  * \[x\] Secondary Indexing  
  * \[ \] Query Planner
  * \[ \] Aggregate Functions and GROUP BY capability  
  * \[ \] JOINS  
  * \[ \] UPDATE, DELETE
  * \[ \] DROP, and ALTER

