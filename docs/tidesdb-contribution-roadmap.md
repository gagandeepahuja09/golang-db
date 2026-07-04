# TidesDB Contribution Roadmap

## Context

You've built a working LSM-tree storage engine in Go (WAL, memtable, SSTable, compaction, 2PL transactions). You want to contribute to TidesDB — a C-based LSM storage engine with advanced features (MVCC, lock-free skip list, bloom filters, WiscKey-style key-value separation, S3 object store). Your C experience is limited to DSA in C++. This is a weekend project alongside your day job at Razorpay.

**Core principle: You already know *what* the code does. You're only learning *how* C expresses it.** This cuts the ramp-up from months to weeks.

**Time budget:** 6-8 hours per weekend (one full day or two shorter sessions).

**Answer to your main concern:** No, you won't be reading for weeks before writing code. By weekend 2, you'll be building the project and running tests. By weekend 3-4, you'll be writing your first test or benchmark. The trick is to enter the codebase through components you already understand.

---

## Phase 1: C Fundamentals + Build TidesDB (Weekends 1-2)

**Goal:** Get comfortable enough with C to read TidesDB source code, and get the project building and running on your machine.

### Weekend 1: Targeted C Ramp-up

You don't need to "learn C" from scratch. You need to learn the delta between C++ (which you've used) and C systems programming. Focus on:

**Must-know C concepts (prioritized for TidesDB):**
1. **Memory management** — `malloc`/`free`/`calloc`/`realloc`. No constructors/destructors. Manual lifetime tracking.
2. **Pointers and pointer arithmetic** — You know this from C++ DSA. Refresh: `void*` casting, function pointers, double pointers (`**`).
3. **Structs (no classes)** — No methods. Functions take struct pointers as first argument (`tidesdb_t *tdb`). This is TidesDB's entire API pattern.
4. **Header files and compilation model** — `.h` for declarations, `.c` for definitions. `#include` guards. Linking.
5. **pthreads** — `pthread_mutex_t`, `pthread_rwlock_t`, `_Atomic` qualifiers. TidesDB is heavily concurrent.
6. **File I/O** — `pread`/`pwrite` (TidesDB's block manager uses these for lock-free I/O). You understand the concepts from your Go `file.ReadAt()`.
7. **Error handling** — No exceptions. Return codes + errno. Every TidesDB function returns `tidesdb_err_t*`.

**Skip for now:** Preprocessor macros (beyond basics), signal handling, setjmp/longjmp, complex build toolchains.

**Learning resources (pick ONE, don't go deep):**
- "Beej's Guide to C Programming" — free, practical, covers exactly what you need
- Or just read TidesDB's `src/buffer.c` (small, self-contained) as your C tutorial — it's a byte buffer with malloc/realloc/free patterns

**Milestone:** Write a small C program that opens a file, writes length-prefixed key-value pairs (like your Go WAL), reads them back. Build it with `gcc`. This bridges your Go knowledge to C syntax in ~2-3 hours.

### Weekend 2: Build and Run TidesDB

**Steps:**
```bash
# Clone
git clone https://github.com/tidesdb/tidesdb.git
cd tidesdb

# Install dependencies (macOS)
brew install cmake lz4 zstd snappy

# Build
cmake -B build -DTIDESDB_BUILD_TESTS=ON
cmake --build build

# Run tests
cd build && ctest

# Run benchmarks
./tidesdb_bench
```

**While building, read these (in order):**
1. `README.md` — feature overview
2. `CONTRIBUTING.md` — CLA/DCO requirements, code style rules
3. Blog: [How Does TidesDB Work?](https://tidesdb.com/getting-started/how-does-tidesdb-work/) — architecture overview
4. Blog: [What I Learned Building a Storage Engine That Outperforms RocksDB](https://tidesdb.com/articles/what-i-learned-building-a-storage-engine-that-outperforms-rocksdb/) — design decisions

**Also run with sanitizers (this is how Alex finds bugs):**
```bash
cmake -B build-san -DTIDESDB_WITH_SANITIZER=ON -DTIDESDB_BUILD_TESTS=ON
cmake --build build-san
cd build-san && ctest
```

**Milestone:** Project builds, all tests pass, you've run benchmarks and have baseline numbers on your machine.

---

## Phase 2: Map the Codebase Through What You Know (Weekends 3-4)

**Goal:** Read the code through the lens of your golang-db. You already built these components — now see how a production C implementation handles them.

### Reading order (map to your knowledge):

| Your golang-db component | TidesDB equivalent | Files to read |
|---|---|---|
| WAL (append-only, CRC32) | Write-ahead log | `src/tidesdb.c` (search for `wal`) |
| Memtable (B-tree) | Skip list (lock-free, arena allocator) | `src/skip_list.h`, `src/skip_list.c` |
| SSTable (index blocks, footer) | Block-based SSTables | `src/block_manager.h`, `src/block_manager.c` |
| Binary search on index | Index lookups | `src/tidesdb.c` (search for `get` / `seek`) |
| Compaction (merge when >= 4 files) | 3-mode compaction (full/dividing/partitioned) | `src/tidesdb.c` (search for `compact` / `merge`) |
| 2PL transactions | MVCC + SSI (5 isolation levels) | `src/tidesdb.c` (search for `txn`) |
| Not built yet | Bloom filters | `src/bloom_filter.h`, `src/bloom_filter.c` |
| Not built yet | Compression (LZ4/Zstd/Snappy) | `src/compress.h`, `src/compress.c` |
| Not built yet | Clock cache (NUMA-aware) | `src/clock_cache.h`, `src/clock_cache.c` |

**How to read:** Don't read linearly. For each component:
1. Read the `.h` file first — it's the API contract (like a Go interface)
2. Read the corresponding test file in `test/` — tests reveal intent better than source
3. Then read the `.c` implementation

**Milestone:** You can explain (to yourself or in notes) how TidesDB's skip list differs from your B-tree memtable, and how the block manager differs from your SSTable reader. Write these notes down — they'll become blog material later.

---

## Phase 3: First Contribution — Tests or Benchmarks (Weekends 5-6)

**Goal:** Get your first PR merged. Start with something low-risk that gets you through the contribution process (CLA, DCO sign-off, code formatting, review cycle).

### Option A: Expand benchmark suite (recommended first PR)

External contributor `oryankibandi` already got a benchmark PR merged (#572 — Zipfian distribution). This is a proven path.

**Ideas:**
- Add concurrent read/write benchmarks (TidesDB's lock-free design should shine here)
- Add benchmarks for different compression algorithms (LZ4 vs Zstd vs Snappy)
- Add benchmarks for different value sizes (small values vs large values — relevant to WiscKey separation)
- Add iterator performance benchmarks

**Why this is ideal first PR:**
- Benchmarks don't change production code — low risk for the maintainer to merge
- You'll deeply understand the API by writing benchmarks
- Running benchmarks on your hardware gives you real data to discuss with Alex
- It's satisfying — you see numbers, you can compare, you can optimize

### Option B: Add or expand test coverage

**Test files to study:** `test/bloom_filter__tests.c` (simple, self-contained), `test/skip_list__tests.c` (more complex, concurrency)

**Ideas based on known gaps:**
- Concurrent column family operations under load (recent bug patterns in PRs #625, #624)
- Iterator seek edge cases (Alex has been fixing these repeatedly — more test coverage prevents regressions)
- Compaction correctness tests (deleted keys reappearing was a recent fix)
- WAL recovery after partial writes

### Before submitting:
```bash
# Format code (required)
./code_formatter.sh

# Run full test suite
cd build && ctest

# Sign DCO (required for all contributions after Jan 31, 2025)
git commit -s -m "your message"   # -s adds Signed-off-by
```

**Milestone:** First PR submitted. Even if it takes a review cycle or two, you've entered the contributor pipeline.

---

## Phase 4: Deeper Contributions (Weekends 7-12)

**Goal:** Move beyond tests into areas that build real understanding and credibility.

### Track A: Concurrency and correctness (highest learning value)

Alex spends most of his time fixing concurrency bugs found by ThreadSanitizer. This is the most technically interesting area and the one where help is most valuable.

**Approach:**
1. Run the full test suite with ThreadSanitizer enabled on your machine
2. Run tests repeatedly (flaky failures = race conditions)
3. When you find a race, understand it, write a minimal reproducer, propose a fix
4. Study how Alex fixes races (recent PRs #625, #624, #621 are excellent case studies)

**Why this is valuable:** Debugging concurrency in C is a rare and highly valued skill. This is exactly the kind of work that signals staff-level systems thinking.

### Track B: Platform and build improvements (easier, still valued)

Successful external contributors (`bkmgit`, `balagrivine`) focused here:
- Fix CMake linking issues (#618 — zstd/snappy/lz4 hardcoded flags)
- Improve cross-platform support
- Code refactoring for modularity (extracting duplicated code, naming consistency)

### Track C: Areas Alex may have overlooked

**Things a fresh pair of eyes catches that a solo maintainer misses:**
1. **Error path testing** — What happens when malloc fails? When disk is full? When a file is corrupted mid-write?
2. **Documentation gaps** — The C API reference exists but inline code comments for complex functions may be sparse.
3. **Fuzz testing** — Issue #112 requested this but was closed. AFL or libFuzzer on the public API would find edge cases.
4. **Memory profiling** — Issue #605 flagged x86 tests exceeding memory. Running Valgrind/heaptrack on the test suite and identifying memory hotspots would be valuable.
5. **Configuration edge cases** — What happens with extreme config values? Zero block size? Bloom filter with 0% false positive rate? 1000 column families?

**Milestone:** 2-3 merged PRs, established relationship with Alex, comfortable reading and modifying the codebase.

---

## Phase 5: Meaningful Feature Work (Month 3+)

By this point you'll have enough context to tackle real feature work. Possible directions:

- **Implement fuzz testing infrastructure** — High impact, Alex specifically wanted this
- **Improve compaction strategies** — You understand compaction from your project, now contribute to a production implementation
- **Performance optimization** — Use your benchmark data to identify and fix bottlenecks
- **New test infrastructure** — Deterministic simulation testing (issue #175)

---

## Parallel Learning Track (Ongoing)

These concepts will come up as you read the code. Learn them as needed, not upfront:

| Concept | When you'll need it | Resource |
|---|---|---|
| pthreads deep dive | Phase 3 (concurrency work) | "Programming with POSIX Threads" by Butenhof (chapters 1-3 only) |
| Lock-free programming | Phase 3-4 | Read TidesDB's skip_list.c atomics, then Jeff Preshing's blog posts |
| CMake | Phase 3 (if doing build PRs) | "Professional CMake" or just read TidesDB's CMakeLists.txt |
| AddressSanitizer / ThreadSanitizer | Phase 3 | Google's sanitizer docs (short, practical) |
| Valgrind / heaptrack | Phase 4 (memory profiling) | Valgrind quick start guide |
| WiscKey paper | Phase 2 (understanding value separation) | "WiscKey: Separating Keys from Values in SSD-Conscious Storage" |
| Bloom filter math | Phase 2 (reading bloom_filter.c) | Your DDIA chapter on bloom filters is sufficient |

---

## Discord Outreach Template

When you're ready to message Alex (after Weekend 2, once you've built and run the project):

> Hey Alex, I've been building my own LSM-tree storage engine in Go (WAL, memtable, SSTable, compaction, transactions) and I've been following TidesDB's development. Got the project building on my Mac and ran the test suite + benchmarks. Really interested in contributing — I was thinking of starting with expanding the benchmark suite (e.g., concurrent read/write benchmarks, different value sizes for the WiscKey separation). Would that be useful, or is there something else you'd find more helpful right now?

This works because: you've done the homework, you have a specific proposal, and you're asking what *he* needs.

---

## Weekly Satisfaction Checkpoints

| Weekend | Tangible Output |
|---|---|
| 1 | Small C program that does file I/O with length-prefixed encoding |
| 2 | TidesDB builds and runs on your machine, benchmark baseline numbers |
| 3 | Notes comparing your golang-db components to TidesDB equivalents |
| 4 | Deep-read of one component (bloom filter or skip list), can explain it |
| 5 | Draft of first PR (benchmark or test) |
| 6 | First PR submitted |
| 7-8 | PR merged, second PR in progress |
| 9-10 | Concurrency bug investigation or fuzz testing setup |
| 11-12 | Meaningful contribution (feature or infrastructure) |

---

## Blog Material (Free Byproduct)

Everything you learn becomes blog material in your existing style:
- "What I Learned Reading a Production Skip List After Building a B-tree Memtable"
- "C Memory Management for Go Developers: What I Wish I Knew"
- "How TidesDB's Block Manager Differs From My SSTable Reader"
- "Finding Concurrency Bugs with ThreadSanitizer: A Weekend Experiment"

---

## Key Resources

**TidesDB docs:**
- [How Does TidesDB Work?](https://tidesdb.com/getting-started/how-does-tidesdb-work/)
- [What I Learned Building a Storage Engine That Outperforms RocksDB](https://tidesdb.com/articles/what-i-learned-building-a-storage-engine-that-outperforms-rocksdb/)
- [Why We Benchmark on Modest Hardware](https://tidesdb.com/articles/why-bench-on-modest-hardware/)
- [From Building Houses to Storage Engines](https://tidesdb.com/articles/from-building-houses-to-storage-engines/)
- [Plugging into MariaDB](https://tidesdb.com/articles/plugging-into-mariadb/)
- [C API Reference](https://tidesdb.com/reference/c/)

**Key source files:**
- `src/tidesdb.h` — Public API (start here)
- `src/skip_list.h/c` — Memtable (compare with your B-tree)
- `src/block_manager.h/c` — File I/O layer (compare with your SSTable reader)
- `src/bloom_filter.h/c` — Feature you haven't built yet
- `src/compress.h/c` — Compression abstraction
- `src/clock_cache.h/c` — Caching layer
- `test/` — 13 test suites
- `bench/tidesdb__bench.c` — Benchmark suite

**C learning (minimal, targeted):**
- Beej's Guide to C Programming (free)
- Jeff Preshing's blog on lock-free programming (for Phase 3+)
