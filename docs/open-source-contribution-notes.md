# Open Source Database Contribution Notes

## Current Project Assessment (golang-db)

### What's Built
- Full write path: WAL (CRC32) -> Memtable (B-tree) -> SSTable (with index blocks)
- Full read path: Memtable -> SSTable search with binary search on index blocks
- Background compaction (merge when >= 4 files)
- SQL parser with CREATE TABLE, INSERT, SELECT
- 2-Phase Locking transactions
- Secondary/composite index support
- Manifest-based metadata tracking
- Good test coverage across all layers

### Gaps
- No bloom filters
- No UPDATE/DELETE
- No MVCC (have 2PL but no multi-version concurrency)
- No deadlock detection
- No JOINs or aggregates
- Single-level compaction (no leveled/tiered compaction strategies)

### Readiness
Strong on: storage engine internals, file format work, index operations, WAL/recovery, basic query execution.
Need ramp-up on: query optimization, MVCC, distributed consensus, leveled compaction, buffer pool management.

---

## Pebble vs Badger

| Factor | Pebble | Badger |
|--------|--------|--------|
| Active development | Very active (CockroachDB depends on it) | Slowed down significantly |
| Community | Strong, backed by Cockroach Labs | Dgraph has had layoffs/pivots |
| Code quality | Extremely well-engineered, heavily tested | Good but less rigorous |
| Architecture | Classic LSM (closest to what I built) | WiscKey (key-value separation) |
| Complexity | Higher — production-grade with many edge cases | Simpler to read |
| Review speed | Active maintainers reviewing PRs | PRs can sit for weeks/months |
| Career signal | Stronger (Cockroach Labs is known DB company) | Weaker given reduced activity |

### Concern
Both Pebble and Badger are small, mature storage engines maintained by a core team. Not much room for outside contributors. Better to look at the **application layer** — databases built on top of storage engines.

---

## Factors to Evaluate Before Deep-Diving on a Project

1. **PR merge velocity** — Check recent merged PRs. If they take months, walk away.
2. **"Good first issue" quality** — Real tasks or abandoned stubs? Do maintainers respond?
3. **Architecture alignment** — Does it map to what I already know?
4. **Upward path** — Where does contributing here lead? (e.g., Pebble -> CockroachDB)
5. **Documentation/comments** — Well-commented codebases are learnable.
6. **Test infrastructure** — Can I run tests locally without complex setup?

---

## Recommended Projects (Application Layer, Go)

### CockroachDB (`cockroachdb/cockroach`)
- Huge codebase = more surface area, more open issues
- 500+ open issues at any time
- Storage engine knowledge helps understand what's underneath
- Contribute to SQL execution, schema changes, testing

### DoltDB (`dolthub/dolt`)
- SQL database with Git-like versioning (branch, merge, diff for data)
- Built in Go with custom storage engine (prolly trees)
- Team at DoltHub is **very contributor-friendly**, actively blogs about internals
- Less intimidating than CockroachDB but still technically interesting
- Read their blog: dolthub.com/blog

### Vitess (`vitessio/vitess`)
- MySQL-compatible sharding middleware in Go
- Powers YouTube's database layer
- CNCF project, lots of SQL planning/execution work

### rqlite (`rqlite/rqlite`)
- Distributed SQLite in Go using Raft consensus
- Much smaller codebase
- Good for learning distributed systems without drowning in complexity

---

## Approach for Understanding a Codebase

### Phase 1: Map the territory (1-2 weeks)
- Read top-level README and docs
- Trace write path end-to-end
- Trace read path end-to-end
- Draw diagram of package/module structure
- Don't try to understand everything — map boundaries first

### Phase 2: Build and break it (1 week)
- Build the project, run full test suite
- Write a small program that uses it as a library
- Read test files for components I understand — tests reveal intent better than source
- Add tracing and step through a write+read cycle

### Phase 3: Pick first contribution (strategic)
- **Start with what I know** — find the component closest to what I built
- **Fix a test or add a test** — lowest risk, gets through PR process
- **Fix a real bug, not cosmetic** — typo fixes don't teach anything
- **Read open issues for 30 min** — look for issues tagged with components I understand

### Phase 4: Build credibility
- First 2-3 PRs: small, correct, well-tested
- Then tackle something meatier
- Engage in issue discussions

---

## Key Insight

> You don't need to understand everything. Nobody understands all of CockroachDB. You understand a **vertical slice**: storage engine internals → how the query layer uses them → one specific component.

> The right project is the one where you read an issue and think "I could figure this out" — not "I understand this already."

## TidesDB (High Priority)

**Why high priority:**
- Already in Discord community — warm start
- Maintainer is very approachable — fast reviews, real mentorship
- Well-documented architecture — faster ramp-up
- Small/1-person project — contributions have outsized impact and visibility
- Have watched it evolve — already have context on design decisions

**Concerns:**
- Written in C — learning curve for memory management
- 1-person project risk (can stall or get abandoned)
- Less career signal than CockroachDB/DoltDB

**Why the C barrier is manageable:**
Already understand the concepts (LSM, WAL, SSTable, compaction) from building golang-db. Not learning two things at once — know *what* the code should do, just learning *how* C expresses it.

**First step:** Message maintainer on Discord, ask what he needs help with. First contribution could be tests, documentation, or a small feature.

---

## Priority Order

1. **TidesDB** — Start here. Door is already open. Community access + approachable maintainer.
2. **DoltDB or CockroachDB** — Parallel track in Go for contributions without C learning curve.

---

## Next Steps

- TidesDB: Reach out on Discord, express interest in contributing
- Build bloom filters in own project, then read a production implementation
- Spend an evening each on DoltDB and CockroachDB, look at open issues
