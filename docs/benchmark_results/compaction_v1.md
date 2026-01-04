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