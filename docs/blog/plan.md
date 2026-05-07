# Blog Series Plan: Building a Database from First Principles

## Context

You've built a solid embedded database in Go covering LSM storage, WAL, compaction, 2PL transactions, SQL parsing, secondary/composite indexes, and a partially-complete cost-based query planner. Your journal docs (parts 1-9) already capture the raw thinking. The goal is to convert this journey into a blog series that teaches intuition, not just steps — and benefits you (revision + gaps surfaced) and readers (building genuine understanding).

---

## Validation of the Idea

**The idea is strong.** Writing a "build-along" series is one of the most effective teaching formats in systems programming. Alex Petrov (Database Internals), Phil Eaton (Simple Databases), and the Crafting Interpreters author all use this approach. Showing the journey — including missteps — is a feature, not a bug.

**Your specific advantage**: you already have raw journal notes (parts 1-9) as a skeleton, benchmark results, real bugs caught (the `:` prefix scan bug), and documented decision points. This is far better than starting from scratch.

---

## On Imperfect Code / Failing Tests

**Short answer: it's fine, with one caveat.**

Publishing with known imperfections is honest and educational. BUT:
1. **Be explicit about it upfront** in each post — "this implementation has known limitations: X, Y" earns credibility
2. **Bugs that illustrate a concept** (like the lexicographic ordering issue with int keys) should be *featured*, not hidden — they're your best teaching moments
3. **Fix before writing** only if the bug undermines the core concept of that specific post

**Recommended pre-blog fixes (small list, high impact):**
- Complete the cost-based query planner (you're 70% there in select.go) — this makes Part 9/10 publishable
- The lexicographic int ordering bug is actually a perfect blog topic ("why storing ints as strings breaks range queries")
- Everything else: document as known limitation or future work

---

## Blog Series Structure

### Series Title: *"Building a Database from Scratch in Go"*
Subtitle: *A learning journal — decisions, mistakes, and real code*

---

### Part 1: Why Build a Database? (Motivation + Architecture Overview)
**Source**: `docs/blog/part1.md` (already drafted), README
- Why this project: the Feynman quote, AI-assisted learning not AI-assisted development
- High-level architecture diagram: WAL → Memtable → SSTable → Query Layer
- What we will NOT cover (no B-trees, no MVCC yet) and why
- **Teaching goal**: Set expectations. Make readers feel the scope without overwhelming them.

### Part 2: WAL — Your First Durability Guarantee
**Source**: `docs/journal/part_1.md`, `docs/journal/part_2.md`
- Start with: what happens if your process crashes mid-write?
- Build up: append-only writes, fsync, length-prefix + CRC32 format
- Key intuition: why sequential writes beat random writes
- Bug to feature: partial write truncation problem — still not fully solved (be honest)
- **Teaching goal**: fsync, checksums, length-prefix framing

### Part 3: Memtable + SSTables — Why WAL Alone Isn't Enough
**Source**: `docs/journal/part_1.md` (WAL limits section), LSM design
- The problem: WAL grows forever, RAM is finite
- Solution: memtable flush → immutable SSTable
- Key design decision: why sorted order matters for reads
- **Teaching goal**: LSM tree fundamentals, why immutability helps

### Part 4: Compaction — Keeping Read Performance Alive
**Source**: `docs/journal/part_4_compaction.md`, `docs/benchmark_results/compaction_v1.md`
- The problem: too many SSTables = slow reads
- Compaction strategies: size-tiered vs leveled
- Manifest files for consistency
- Show benchmark results: before/after compaction
- **Teaching goal**: read amplification, write amplification tradeoffs

### Part 5: Transactions and 2PL — ACID Without Magic
**Source**: `docs/journal/part_7_transactions.md`, `transaction.go`
- What ACID actually means (not just the acronym)
- 2-Phase Locking: grow phase, shrink phase
- Read locks vs write locks, lock upgrades
- What we punted on: no wait queue, errors instead (be honest about the tradeoff)
- **Teaching goal**: serializability, 2PL mechanics

### Part 6: SQL Parsing — Building a Mini Parser
**Source**: `docs/journal/part_6_sql.md`, `create.go`
- Why not use an existing parser? Learning is the goal
- CREATE TABLE: tokenizing, schema serialization to binary
- Known limitation: CREATE TABLE is not atomic (multiple Puts, not one transaction)
- **Teaching goal**: parsing basics, schema persistence in KV store

### Part 7: SELECT — Three Execution Strategies
**Source**: `docs/journal/part_8_sql_v2.md`, `select.go`
- Strategy 1: Primary key pointed query (O(log n))
- Strategy 2: Full table scan (O(n))
- Strategy 3: Secondary index prefix scan
- How the decision tree works (current code walkthrough)
- **Teaching goal**: query execution fundamentals

### Part 8: Secondary Indexes — The Prefix Scan Design
**Source**: `docs/journal/part_8_sql_v2.md`, `secondary_index_test.go`
- Index key format: `index:tablename:indexname:col1:col2:pk`
- Why this structure enables prefix scans
- The `:` bug: how `city:5` matched `city:50` and how we caught it with tests
- Composite indexes and prefix matching logic
- Benchmark results: low vs high cardinality, 100 vs 10k rows
- **Teaching goal**: index design, prefix key structures, importance of delimiters

### Part 9: The Query Planner — Choosing the Right Index
**Source**: `docs/journal/part_9_sql_v3.md`, `select.go` (WIP)
- The problem: multiple applicable indexes, which one do you pick?
- Reservoir sampling for cardinality estimation
- Cost formula: `(count_in_range / sample_size) * total_rows`
- Current state: in progress — show the code and the reasoning
- What's next: range queries, histogram approaches
- **Teaching goal**: statistics-based cost estimation, reservoir sampling intuition

---

## Format Recommendations

**Per-post structure:**
1. **The Problem** (1-2 paragraphs) — what breaks without this feature
2. **Intuition Building** — explain the concept before showing code; use analogies
3. **Our Approach** — design decisions and trade-offs we made
4. **The Code** — key snippets only, link to GitHub for full context
5. **What We Punted / Known Issues** — 2-3 bullets, honest about gaps
6. **What's Next** — bridge to next post

**Length**: 1500-2500 words per post. Long enough for depth, short enough to finish.

**Platform**: Medium or personal blog/Substack. GitHub repo link in every post.

**Tone**: First-person, conversational. "I tried X, it broke because Y, so I switched to Z."

---

## Potential Challenges

1. **Perfectionism trap**: Don't wait until the codebase is "done". Start with Part 1 now.
2. **Going too deep too fast**: Each post should be readable without the previous ones.
3. **Code snippets vs walls of text**: Show only the 10-20 most important lines; link to GitHub for rest.
4. **The lexicographic bug**: This is currently in the code. Either fix it or feature it. Don't silently have broken range queries.
5. **Query planner WIP**: Finish the cost-based selection before writing Part 9.

---

## Recommended Starting Point

1. Finish the query planner (select.go is ~70% done — complete cost estimation logic)
2. Write Part 1 (motivation/architecture) — already partially drafted in `docs/blog/part1.md`
3. Write Part 8 (secondary indexes + the `:` bug story) — this is your most compelling concrete story
4. Fill in the rest sequentially

---

## Files to Reference

- `docs/blog/part1.md` — existing draft
- `docs/journal/part_1.md` through `part_9_sql_v3.md` — raw material
- `docs/benchmark_results/secondary_index.md` — use in Part 8
- `db/select.go` — query planner WIP
- `db/transaction.go`, `db/create.go`, `db/secondary_index_test.go` — code for Parts 5-8