<p align="center">
  <img src="docs/assets/brand/saardb-logo-on-white.png" alt="SaarDB logo" width="140" />
</p>

# SaarDB

**Database internals, explained by building them.**

SaarDB is a first-principles database project in Go, built to make storage, indexing, transactions, and SQL execution feel intuitive.

## Why This Exists

SaarDB is built to make the "magic" under the hood of databases easier to understand by implementing the core pieces from first principles: write-ahead logging, in-memory indexing, SSTables, compaction, transactions, and a small SQL layer.

This is a learning project, not a production database. The goal is to make the internals understandable by building them one layer at a time.

## Architecture

The current system is centered around an LSM-style storage engine:

```text
Writes:
REPL / SQL -> WAL -> in-memory map -> SSTable flush -> compaction

Reads:
in-memory map -> newest SSTable -> older SSTables
```

At a high level:

- **WAL** gives crash recovery through append-only writes.
- **In-memory map** keeps recent writes fast to read.
- **SSTables** persist sorted data to disk.
- **Compaction** merges SSTables and removes stale versions.
- **Transactions** use 2-phase locking for serializable writes.
- **SQL layer** parses and executes a growing subset of relational operations.

## Getting Started

You only need Go installed.

```bash
git clone https://github.com/gagandeepahuja09/saardb
cd saardb
go run main.go
```

## Usage Examples

These examples use commands that are runnable from the current CLI.

```text
PUT user:1 Gagan
GET user:1
```

The relational layer also supports basic table creation and inserts:

```sql
CREATE TABLE users (age INT, id STRING, active BOOL, PRIMARY KEY (id));
INSERT INTO users VALUES (25, user1, 1)
```

`SELECT` parsing and internal execution paths exist, including secondary-index based reads, but CLI `SELECT` wiring is still in progress.

## Feature Checklist

### Storage Layer

- [x] Write-Ahead Log (WAL) with binary command serialization
- [x] WAL checksums for corruption detection
- [x] In-memory sorted write buffer
- [x] Periodic flush to immutable SSTables
- [x] SSTable index blocks for faster lookup
- [x] Background compaction with manifests
- [ ] Tuned flush sizing
- [ ] Bloom filters
- [ ] More systematic benchmarks

### Transaction Layer

- [x] 2-phase locking (2PL)
- [x] Atomic multi-key transaction payloads in WAL
- [ ] MVCC
- [ ] Multiple isolation levels

### Query Layer

- [x] `CREATE TABLE`
- [x] `INSERT INTO`
- [x] `SELECT` parser
- [x] Internal SELECT execution
- [x] Secondary and composite indexes
- [ ] CLI SELECT wiring
- [ ] Query planner
- [ ] Aggregate functions and `GROUP BY`
- [ ] Joins
- [ ] `UPDATE`, `DELETE`, `DROP`, and `ALTER`

## Learning Series

The project is also documented as a learning series focused on intuition, not just implementation steps:

- [Part 1: WAL](docs/blog/part1.md)
