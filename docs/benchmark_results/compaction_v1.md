### Results With Compaction
goos: darwin
goarch: arm64
pkg: github.com/golang-db
cpu: Apple M1 Pro
=== RUN   BenchmarkSSTableWithCompaction
BenchmarkSSTableWithCompaction
2026/01/04 11:32:57 INFO COMPACTION_STARTED files_to_be_compacted_count=4
2026/01/04 11:32:57 INFO COMPACTED_FILE_WRITE_SUCCESSFUL file_name=data_files_sstable/4.log
2026/01/04 11:32:57 INFO COMPACTED_FILE_ATOMIC_SWAP_SUCCESSFUL files_to_compact_count=4
2026/01/04 11:32:57 INFO COMPACTION_STARTED files_to_be_compacted_count=4
2026/01/04 11:32:57 INFO COMPACTED_FILE_WRITE_SUCCESSFUL file_name=data_files_sstable/8.log
2026/01/04 11:32:57 INFO COMPACTED_FILE_ATOMIC_SWAP_SUCCESSFUL files_to_compact_count=4
BenchmarkSSTableWithCompaction-8            3104            386022 ns/op          203957 B/op       3459 allocs/op
PASS
ok      github.com/golang-db    4.757s

### Results Without Compaction
(Code ref)[https://github.com/gagandeepahuja09/golang-db/compare/gagandeepAhuja/woCompactionBench?expand=1]
goos: darwin
goarch: arm64
pkg: github.com/golang-db
cpu: Apple M1 Pro
=== RUN   BenchmarkSSTableWithoutCompaction
BenchmarkSSTableWithoutCompaction
BenchmarkSSTableWithoutCompaction-8          656           1573801 ns/op          848196 B/op      13429 allocs/op
PASS
ok      github.com/golang-db    3.662s

Summary Table
Metric	With Compaction	Without Compaction	Improvement
Iterations	3,104	656	~4.7x more
Time/op	386,022 ns	1,573,801 ns	~4.1x faster
Memory/op	203,957 B	848,196 B	~4.2x less memory
Allocations/op	3,459	13,429	~3.9x fewer allocs

### Summary Table

| Metric | With Compaction | Without Compaction | Improvement |
|--------|-----------------|-------------------|-------------|
| **Iterations** | 3,104 | 656 | ~4.7x more |
| **Time/op** | 386,022 ns | 1,573,801 ns | **~4.1x faster** |
| **Memory/op** | 203,957 B | 848,196 B | **~4.2x less memory** |
| **Allocations/op** | 3,459 | 13,429 | **~3.9x fewer allocs** |