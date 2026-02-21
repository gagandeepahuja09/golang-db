## How do databases support full-table scan?

======================================= NOT NEEDED ====================================================
- If INSERT INTO is supported via [<table_name>:<primary_key_id>] ==> we can easily find a row using GET.
- Problems:
    - How to get all the rows of a table?
        - We will have to add key = [`primary_keys`:<table_name>]
        - value = ['primary_key_1', 'primary_key_2', ] ... ==> this is the list of sorted primary keys. 
        - How will this list be serialised in DB?
            - If string: [length_of_list][length_of_pk1][pk1][length_of_pk2][pk2]...
            - If int: [length_of_list][pk1][pk2]...
        - So, INSERT INTO will do PUT on both <table_name>:<primary_key_id> and `primary_keys`:<table_name>.
        - Why sorted? This will assist in range based queries on primary key as well.
        - How will we insert in sorted array? Finding the index for insertion is a straightforward binary search problem.
        - But update in array is O(N). C++ Vectors make it ammortized O(1)?
- Query example: 
    - SELECT * FROM payments WHERE id >= 'abc123' AND id <= 'efg456'
    - GET primary_keys:payments ==> what if there are millions of keys?
    - Then we will have to create an index for this primary_keys key also.
======================================= NOT NEEDED ====================================================

- Above can instead be solved by prefix scan.
- Todo: see how these commands: CREATE TABLE, INSERT INTO and SELECT perform at scale and what needs to be done differently for supporting them.
    - How does full-table scan work with millions of rows?
    - How to support constraints like UNIQUE?
    - Adding INDEX support
    - ALTER Table command support
    - NOT NULL constraint support
    - Support 2 kinds of INSERT INTO syntaxes and the implications of it on how much space is consumed. 
- Supporting NULL values in database.
    - NULL values are different from zero values.
    - One key feature of NULL value should be that it doesn't take up space in the database.
    - How do we implement it?
    - Currently how we serialise data is such that we reserve 4 bytes for integer type while reading.
    - Alternately we can store an additional byte telling whether it is NULL or NOT NULL.
    - If it is null, we don't read the 4 bytes.