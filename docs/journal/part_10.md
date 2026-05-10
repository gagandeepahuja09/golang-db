Here's what I found, organized from practical to academic:                                                       
                                                                                                                   
  Practical / Industry                                                                                             
                                                                                                                   
  1. https://github.com/facebook/rocksdb/wiki/RocksDB-Tuning-Guide — The canonical reference. Covers               
  write_buffer_size (memtable/flush), block_size (data block), and frames everything around the three amplification
   factors: write, read, and space. RocksDB defaults to 64MB memtable and 4KB block size, with clear explanations  
  of why you'd change each.                                                                                      
  2. https://github.com/facebook/rocksdb/wiki/Setup-Options-and-Basic-Tuning — More concise starting point if the
  full tuning guide is overwhelming.                                                                               
  3. https://ceph.io/en/news/blog/2022/rocksdb-tuning-deep-dive/ — Real-world experiments showing that smaller
  memtables can increase write amplification (deletes and updates land in different flush groups), but this can be 
  compensated by tuning compaction triggers. Great for building intuition about how flush size interacts with    
  compaction.                                                                                                      
  4. https://betterprogramming.pub/navigating-the-minefield-of-rocksdb-configuration-options-246af1e1d3f9 —      
  Blog-style walkthrough of the key knobs with clear tradeoff explanations.                                        
  
  Research Papers                                                                                                  
                                                                                                                 
  5. https://nivdayan.github.io/monkey-journal.pdf (Dayan, Athanassoulis, Idreos — SIGMOD 2017 / TODS 2018) — This 
  is the one I'd most recommend for your blog's audience. It maps the full LSM design space (merge policy, buffer
  size, bloom filter allocation) into a closed-form cost model. Shows that the tradeoff between lookup cost, update
   cost, and memory is fundamental, and that most systems tune it suboptimally. The                              
  http://daslab.seas.harvard.edu/monkey/ has good visualizations.
  6. https://www.researchgate.net/publication/325376432 (Dayan, Idreos — SIGMOD 2018) — Builds on Monkey. Key
  insight: most merge operations at non-largest levels are wasteful. Introduces "Lazy Leveling" which only merges  
  aggressively at the largest level. Provides a closed-form model to pick optimal config given a workload.
  7. https://www.cidrdb.org/cidr2017/papers/p82-dong-cidr17.pdf (Dong et al. — CIDR 2017) — From the RocksDB team  
  at Facebook. Focuses specifically on the space amplification side of the tradeoff triangle.                      
  
  The Core Mental Model                                                                                            
                                                                                                                 
  The key framing that all these sources converge on: you can optimize for at most two of {read amplification,     
  write amplification, space amplification}. Flush size and block size are knobs that move you along this triangle.
   Monkey/Dostoevsky formalize this into math; the RocksDB guides give you the practical levers.                   
                                                                                                                 
  For your blog's "tuning flush size and block size" section, I'd reference the RocksDB tuning guide for the       
  practical defaults and the Monkey paper for the theoretical foundation