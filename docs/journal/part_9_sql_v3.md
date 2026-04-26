## Benchmarking Performance Of Secondary Indexes
- Write multiple benchmarks: 100, 1k, 10k, 100k, 1 million rows inserted.
- Only 2 columns: c1: PK, c2
- What is the cardinality for c2? Test both cases 
    - Low cardinality case: only 5-10 unique values.
    - High cardinality case: Some random id set. 
- Case 1: add index on c2.
- Case 2: don't add index on c2.

## Query Planner
- If there are multiple indexes to choose from, pick the path which will return the least number of rows.
- How to come to know how many rows will be returned by the query?
- Two cases: range based queries and pointed queries.

### Pointed Queries
#### Brute-force solution
- For **pointed queries**, we need to understand what is the average cardinality for a specific table. We can find that by maintaining the no. of rows and the no. of unique values in the table.
- This would give a view of **avg. no of rows returned for a table** which is equivalent to cardinality.
- **Solution for getting average no. of rows**
    - During each write, update the **no. of rows** and **no. of unique values**.
    - This needs to be persisted. 2 keys which need to be stored:
        1. number of rows key: num_rows:table_name and value => <value_of_num_rows>.
        2. number of unique values key: unique_values_table_name and value => list of unique values.
    - **Logic for updating no. of unique values**
        - Whenever we update the key unique_values_table_name, we go through the entire list of unique values. If the value is not found, we insert it into the values list.
        - What would be the optimal way to store this? Maintain a sorted list? Search can be done via binary search to check if value is present or not. Insertion would still be O(N).

#### Optimisation For Number Of Unique Values
- https://pkg.go.dev/github.com/datadog/hyperloglog
- It is unoptimal to keep a track of all possible unique values within a sorted set. With 1M possible unique values like in case of unique id based secondary index column, we would be storing 1M unique values. 
- Hence data structures like hyperloglog come in handy to keep a small memory footprint.
- **What is hyperloglog and how it works? Intuition**

#### Optimisation For Skewed Data
- The above solution suffers from **skewed data**, where only few rows contribute large % of data. To reduce the likelihood of this problem, we can store the no. of rows returned by top N rows.
- The problem is: how to determine the value of N? How do we know what are the top N rows? What if our query has a where condition has a column which returns very less no. of rows? That solution is not handled by our approach. Also, if we store only for top N, it doesn't solve the whole problem. This is an estimation problem, what is the best solution?
- To solve for skewed data, commonly a histogram bucket approach is utilised. The data is divided into buckets to better visualize how the data looks like. There are 2 common ways to structure histogram.
* **Equi-width histogram**: Divide the data into equal histograms. Example: If the values are between 10 and 100, then we store how many rows are there between 10-20, 20-30, ..., 90-100.
* **Equi-height histogram**: If the data pattern is that most rows are between 20-30, that histogram width would be lower and broken down into further buckets to ensure that the heights are similar. Example: 10-20, 20-22, 23-24, 25-27, 28-30, 30-45, 45-60, 60-80, 80-100. This one is more useful for query planner and skewed data understanding as it would capture the skewed data pattern better.
- The histogram pattern would be useful for both pointed queries and range queries.


### When And How to store the statistics data: cardinality for a table and histogram for a table?
- Rather than storing each value, which would incur a lot of memory, we only store a random sample of values. Let's say 1000 values. 
- We will come back on how to choose the value of N (sample size).
- Why do we think that choosing a random sample of N values would suffice even for a case where there are millions of rows in a table? That is the power of random numbers. A random sample would give roughly the same accuracy as the actual table. But I still don't get it, how do we ensure "uniform randomness" and what exactly is "uniform randomness"
- What is meant by "is this sample drawn uniformly?"
- How did we reach the 1 / sqrt(sample_size) formula?
- What is reservoir sampling and the k/N probability formula?

#### Uniform Randomness
- This means that every element has an equal probability of being picked.
- If the table has 1M rows and we want a sample of 1k, every row has a probability of 1k / 1M = .1% of being picked.
- **Uniform sampling mirrors the production data**: If a city column has 60%, 30%, 10% pattern for 3 cities, the same would be observed in the sampled data.

#### Problem: How to ensure uniform randomness for a stream of numbers? Solution: Reservoir Sampling
- Since table size is every-growing, ensuring random sampling requires some creative solution. This is where **reservior sampling** helps.
- k: Sample size, N: table size.
- For the first k numbers, fill them in the list directly.
- If i > k: generate a random number between 0 and i. Let's call it j. If j < k: replace random_sample[j] with arr[i]. If not, discard it.
- So, each number is being randomly check if it is a candidate for being in the random sample list.

#### Reads and Writes Path For Sample Path
##### Write Path
- For every indexed column, track the sampled data for 1000 via reservoir sampling.
- Key: col_sample:<table_name>:<column_name>
- Value: List of numbers in random sample stored in binary encoded manner.
- Keep on maintaining the list in sorted order.
- This list can be persisted to ss-table during flush. Before that, it will be read in-memory only.

### Read Path
- Once we have the sorted list for a column value, give a range query like WHERE c1 >= 100 AND c1 <= 1000.
- For greater condition, find the first occurence of 100 in the sorted list. (similar to lower bound)
- For less condition, find the last occurrence of 1000 in the sorted list. (similar to upper bound)
- Both these actions can be done via binary search.
- One we get the estimate of the range, we can apply a formula like (range_of_sample) * (table_size / sample_size) to get an estimate.
- Note: We also need to store the table_size now as stats.
- That way we don't need to really think about maintaining buckets.

- **Range queries** would be solved later.   
- There might also be cases where choosing an index is not necessary.

#### Next Steps
- We don't yet have range query support, especially with indexes.
- We need to solve for composite indexes case, what happens to query planner in that case?