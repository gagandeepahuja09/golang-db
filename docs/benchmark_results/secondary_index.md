go test -bench=. -tags=secondaryindex -run=^$ -benchmem -p=1
goos: darwin
goarch: arm64
pkg: github.com/golang-db/db
cpu: Apple M1 Pro
BenchmarkSecondaryIndex_NotUsedLowCardinality100Rows-8    	    7602	    153532 ns/op	  246742 B/op	    2296 allocs/op
BenchmarkSecondaryIndex_UsedLowCardinality100Rows-8       	   20708	     53451 ns/op	   49075 B/op	     984 allocs/op
BenchmarkSecondaryIndex_NotUsedHighCardinality100Rows-8   	    1453	    820046 ns/op	 1221214 B/op	   15508 allocs/op
BenchmarkSecondaryIndex_UsedHighCardinality100Rows-8      	   14221	     80750 ns/op	   65225 B/op	    1751 allocs/op
BenchmarkSecondaryIndex_NotUsedLowCardinality10kRows-8    	      52	  22126575 ns/op	30654487 B/op	  201180 allocs/op
BenchmarkSecondaryIndex_UsedLowCardinality10kRows-8       	     136	   8549737 ns/op	 5974242 B/op	   70768 allocs/op
BenchmarkSecondaryIndex_NotUsedHighCardinality10kRows-8   	       1	10927318625 ns/op	15402915488 B/op	150480054 allocs/op
BenchmarkSecondaryIndex_UsedHighCardinality10kRows-8      	      97	  12100610 ns/op	 6693958 B/op	  182814 allocs/op
PASS
ok  	github.com/golang-db/db	196.224s