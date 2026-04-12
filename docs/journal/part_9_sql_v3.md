### Benchmarking Performance Of Secondary Indexes
- Write multiple benchmarks: 100, 1k, 10k, 100k, 1 million rows inserted.
- Only 2 columns: c1: PK, c2
- What is the cardinality for c2? Test both cases 
    - Low cardinality case: only 5-10 unique values.
    - High cardinality case: Some random id set. 
- Case 1: add index on c2.
- Case 2: don't add index on c2.