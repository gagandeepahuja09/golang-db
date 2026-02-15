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