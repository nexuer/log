# Benchmarks

This package separates encoding, call-site construction, and writer-path costs.
Do not compare results from different layers as if they measured the same work.

## Run

Compile the benchmark package without running benchmarks:

```sh
go test -run '^$'
```

Run the full suite:

```sh
go test -run '^$' -bench=. -benchmem -count=5
```

Run the cross-library JSON comparison:

```sh
go test -run '^$' -bench='^BenchmarkComparison(EncodeOnly|WritePath)$' -benchmem -count=5
```

Run the Nexuer JSON-focused scenarios:

```sh
go test -run '^$' -bench='^BenchmarkNexuer(FieldScale|FieldForms|AnyValues)/JSON' -benchmem -count=5
```

Run the direct Nexuer versus standard `slog` matrix:

```sh
go test -run '^$' -bench='^BenchmarkNexuerVsSlogEncodeOnly' -benchmem -count=5
```

Run the fair handler replacement comparison, with both implementations behind
the same `slog.Logger` and producing records without time or PC/source fields:

```sh
go test -run '^$' -bench='^BenchmarkSlogHandlers' -benchmem -count=5
```

## Measurement Layers

### EncodeOnly

`BenchmarkComparisonEncodeOnly`, `BenchmarkComparisonTextEncodeOnly`, and all
`BenchmarkNexuer*` emission benchmarks write to `io.Discard`.

For Nexuer, `io.Discard` is a special fast path that skips the handler's writer
mutex and `Write` call. These benchmarks measure level checks, message handling,
field conversion, and encoding. They do not measure output synchronization or
device I/O.

Third-party libraries use their native `io.Discard` behavior. No lock wrapper is
added around zap, zerolog, phuslu, slog, or logrus.

### WritePath

`BenchmarkComparisonWritePath` uses a distinct no-op writer. It forces every
library to call `Write` while keeping the sink itself inexpensive.

No external mutex is added. Each library keeps its native synchronization
policy. Nexuer, slog, and logrus therefore include their internal writer lock;
zap, zerolog, and phuslu are measured without an additional benchmark lock.
The no-op writer is stateless and can safely be called concurrently.

This benchmark measures logger-side writer-path overhead. It is not a disk,
terminal, network, or file-rotation benchmark.

### Serial And Parallel

Every emission scenario has both variants:

- `Serial` measures single-goroutine latency.
- `Parallel` uses `testing.B.RunParallel` and measures aggregate throughput under
  the current `GOMAXPROCS` setting.

Run with explicit CPU counts when studying scaling:

```sh
go test -run '^$' -bench='BenchmarkComparisonWritePath' -benchmem -cpu=1,2,4,10
```

## Scenario Coverage

Cross-library comparisons:

- disabled level;
- message-only JSON encoding;
- accumulated structured fields;
- call-site structured fields with prebuilt fixtures;
- native text/console formatters;
- native writer path, with and without short fields;
- `slog.Attr` and `WithGroup` through each library's closest public API.

Nexuer-specific scenarios also measure `DefaultFields` (`DefaultTimestamp` and
`DefaultCaller`) separately from static accumulated fields.

`BenchmarkNexuerVsSlogEncodeOnly` additionally pairs Nexuer and standard `slog`
for message sizes, primitive field scaling, generic values, accumulated fields,
group depth, and context-aware logging in both JSON and text formats.

Nexuer-specific coverage:

- `InfoS`, `Info`, and `Infof` message APIs;
- short, 4 KiB, escaped, and Unicode messages;
- disabled calls with prebuilt and call-site arguments;
- primitive field counts of 1, 2, 5, 10, 25, and 50;
- key-value, native `Field`, and `slog.Attr` forms;
- prebuilt versus call-site field construction;
- `Any` values for slices, times, structs, struct slices, and errors;
- preformatted fields, timestamp valuer, caller valuer, and default valuers;
- `WithContext` and `Log` with context;
- one-level and three-level groups, plus grouped fields;
- `Replacer` callbacks;
- `New`, `With`, `WithFields`, `WithGroup`, and `WithContext` construction cost;
- JSON and text handlers, in both serial and parallel modes.

## Fixture Rules

Cases named `Prebuilt` construct slices before the timer starts. They isolate
logger and encoder cost.

Cases named `Constructed` create variadic fields in the timed call. They measure
the API's end-to-end call-site cost.

Do not move helper allocation into or out of a benchmark loop without renaming
the case. That changes what the benchmark measures.

## Comparison Limits

The libraries do not produce byte-for-byte identical output. In particular,
timestamp defaults and console formatting differ. Compare relative behavior
within a named scenario and inspect allocations alongside `ns/op`.

`io.Discard` results are useful for optimizing encoding. They should not be used
to estimate production log throughput when the destination blocks or performs
real I/O.

## Performance Comparison

The following results were measured on an Apple M1 Pro. Lower latency and fewer
allocations are better. EncodeOnly and WritePath are separate measurement
layers and should not be compared directly.

Serial JSON message encoding:

| Library | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| phuslu | 130.8 | 0 | 0 |
| zerolog | 163.2 | 0 | 0 |
| Nexuer | 224.4 | 0 | 0 |
| zap | 260.0 | 0 | 0 |
| slog | 429.8 | 0 | 0 |
| logrus | 1110 | 938 | 20 |

Nexuer is faster than zap, slog, and logrus in this JSON case, while zerolog
and phuslu are faster. In the parallel EncodeOnly case Nexuer records 42.52
ns/op, ahead of zap at 52.13 ns/op and behind zerolog at 28.53 ns/op and phuslu
at 20.76 ns/op.

Serial native text/console message encoding:

| Library | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| zap | 274.8 | 24 | 2 |
| Nexuer | 437.4 | 0 | 0 |
| slog | 665.2 | 0 | 0 |
| phuslu | 685.0 | 897 | 5 |
| logrus | 1061 | 472 | 14 |
| zerolog | 2305 | 1786 | 31 |

These text and console encoders do not produce byte-for-byte identical output,
so the table compares each library's native formatter rather than identical
encoding work.

Using Nexuer as a handler behind the same `slog.Logger` keeps the common paths
close to the standard handlers:

| Scenario | Standard handler | Nexuer handler | Allocation result |
| --- | ---: | ---: | --- |
| JSON message | 380.3 ns | 427.7 ns | both 0 B, 0 allocs |
| JSON 5 fields | 600.2 ns | 644.1 ns | both 0 B, 0 allocs |
| JSON 50 fields | 2840 ns | 2677 ns | both 2049 B, 1 alloc |
| JSON `[]time.Time` | 2623 ns | 1443 ns | 817 B/12 vs 0 B/0 |
| Text message | 612.3 ns | 643.2 ns | both 0 B, 0 allocs |
| Text 5 fields | 854.4 ns | 859.2 ns | both 0 B, 0 allocs |
| Text 50 fields | 3181 ns | 2998 ns | both 2049 B, 1 alloc |
| Text `[]int` | 1324 ns | 912.0 ns | 104 B/11 vs 0 B/0 |

The standard slog handlers are faster for small serial messages and a few
small-field cases. Nexuer becomes faster in the larger-field cases shown above
and has a larger advantage for the listed slice values. Struct and generic
fallback cases are closer because both implementations use reflection-based
encoding or formatting.

WritePath forces a real `Write` call to a no-op writer. Nexuer includes its
writer lock, while benchmarks do not add locks to third-party libraries:

| Scenario | Nexuer | zap | zerolog | phuslu | slog | logrus |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Serial message | 225.2 ns | 311.1 ns | 164.0 ns | 131.4 ns | 436.4 ns | 1114 ns |
| Parallel message | 207.2 ns | 43.18 ns | 29.64 ns | 24.02 ns | 222.0 ns | 1445 ns |
| Serial short fields | 408.1 ns | 441.3 ns | 212.8 ns | 157.4 ns | 604.0 ns | 1968 ns |
| Parallel short fields | 254.6 ns | 145.8 ns | 35.73 ns | 21.27 ns | 265.6 ns | 2395 ns |

Nexuer is faster than standard slog in all four WritePath cases and faster than
zap in both serial cases. Its parallel results remain close to slog and trail
the libraries that do not serialize writes in this benchmark. This difference
reflects writer synchronization policy rather than encoder performance.

## Full Result

Command:

```sh
go test -run '^$' -bench=. -benchmem -benchtime=200ms -count=1
```

The complete unedited output follows.

```text
goos: darwin
goarch: arm64
pkg: github.com/nexuer/log/benchmarks
cpu: Apple M1 Pro
BenchmarkComparisonTextEncodeOnly/Message/Serial/Nexuer-10         	  551416	       437.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Zap-10            	  873375	       274.8 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Zerolog-10        	  101122	      2305 ns/op	    1786 B/op	      31 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Phuslu-10         	  413920	       685.0 ns/op	     897 B/op	       5 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Slog-10           	  355447	       665.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Logrus-10         	  223234	      1061 ns/op	     472 B/op	      14 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Nexuer-10       	 3339784	        74.39 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Zap-10          	 2465101	       107.2 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Zerolog-10      	  246819	       992.5 ns/op	    1788 B/op	      31 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Phuslu-10       	  636878	       405.4 ns/op	     898 B/op	       5 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Slog-10         	  959919	       236.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Logrus-10       	  166327	      1390 ns/op	     472 B/op	      14 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Nexuer-10     	  408634	       584.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Zap-10        	  642620	       357.1 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Zerolog-10    	   60945	      3937 ns/op	    2292 B/op	      62 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Phuslu-10     	  241531	       955.7 ns/op	     993 B/op	      11 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Slog-10       	  283027	       850.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Logrus-10     	   19654	     12037 ns/op	    5185 B/op	      95 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Nexuer-10   	 2356561	        99.53 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Zap-10      	 2118634	       118.3 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Zerolog-10  	  140620	      1695 ns/op	    2295 B/op	      62 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Phuslu-10   	  499692	       495.6 ns/op	     995 B/op	      11 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Slog-10     	  833466	       256.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Logrus-10   	   17479	     13677 ns/op	    5196 B/op	      95 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/SlogJSON-10       	  430598	       558.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/PhusluSlogJSON-10 	  727897	       332.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/NexuerJSON-10     	  691268	       341.0 ns/op	      64 B/op	       1 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/SlogJSON-10     	  984901	       247.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/PhusluSlogJSON-10         	 5466326	        46.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/NexuerJSON-10             	 2161082	       104.0 ns/op	      64 B/op	       1 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/SlogJSON/LogAttrs-10            	  427492	       548.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/PhusluSlogJSON/LogAttrs-10      	  757904	       316.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/NexuerJSON/InfoS-10             	  666859	       374.2 ns/op	     128 B/op	       3 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/SlogJSON/LogAttrs-10          	 1000000	       244.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/PhusluSlogJSON/LogAttrs-10    	 5151038	        42.45 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/NexuerJSON/InfoS-10           	 1826320	       128.3 ns/op	     128 B/op	       3 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Short-10                           	 1000000	       219.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Long4K-10                          	   60074	      3987 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Escaped-10                         	 1000000	       216.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Info/SingleString-10                     	 1000000	       240.0 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Infof/NoArgs-10                          	 1000000	       219.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Infof/Formatting-10                      	   42860	      5572 ns/op	    1778 B/op	      52 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Short-10                         	 4853689	        44.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Long4K-10                        	  460688	       509.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Escaped-10                       	 5131114	        41.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Info/SingleString-10                   	 4367156	        63.36 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Infof/NoArgs-10                        	 4198590	        56.18 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Infof/Formatting-10                    	  141834	      1658 ns/op	    1783 B/op	      52 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Short-10                           	  548869	       436.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Long4K-10                          	   13470	     17718 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Escaped-10                         	  652653	       372.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/Info/SingleString-10                     	  528735	       455.1 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/Text/Serial/Infof/NoArgs-10                          	  553180	       433.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/Infof/Formatting-10                      	   30442	      7820 ns/op	    1779 B/op	      52 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Short-10                         	 3196291	        78.63 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Long4K-10                        	  106555	      2214 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Escaped-10                       	 3519897	        67.42 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Info/SingleString-10                   	 2926519	        91.92 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Infof/NoArgs-10                        	 3010473	        81.29 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Infof/Formatting-10                    	  121311	      2071 ns/op	    1784 B/op	      52 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS-10                                 	66506570	         3.516 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/Info-10                                  	12911845	        17.97 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS/PrebuiltFields10-10                	75936342	         3.188 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS/ConstructedFields-10               	 7467784	        32.82 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS-10                               	544959646	         0.4314 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/Info-10                                	35747308	         7.157 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS/PrebuiltFields10-10              	587756890	         0.4074 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS/ConstructedFields-10             	 6391435	        38.93 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS-10                                 	67783857	         3.546 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Serial/Info-10                                  	13412160	        17.82 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS/PrebuiltFields10-10                	73789392	         3.205 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS/ConstructedFields-10               	 7601210	        33.89 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS-10                               	547661796	         0.4346 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/Info-10                                	29882959	         7.087 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS/PrebuiltFields10-10              	593253594	         0.3965 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS/ConstructedFields-10             	 6045770	        38.25 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields1-10                             	  862507	       270.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields2-10                             	  753393	       316.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields5-10                             	  560403	       428.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields10-10                            	  391471	       614.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields25-10                            	  204046	      1187 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields50-10                            	  113848	      2109 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields1-10                           	 4211966	        59.38 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields2-10                           	 3421461	        65.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields5-10                           	 3121552	        92.85 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields10-10                          	 2338124	       127.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields25-10                          	 1332434	       209.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields50-10                          	  571983	       385.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields1-10                             	  495781	       494.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields2-10                             	  451764	       529.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields5-10                             	  367316	       642.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields10-10                            	  287715	       839.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields25-10                            	  167120	      1433 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields50-10                            	  100255	      2397 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields1-10                           	 2881645	        81.90 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields2-10                           	 2609101	        94.79 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields5-10                           	 2202188	       106.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields10-10                          	 1632446	       154.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields25-10                          	  824281	       253.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields50-10                          	  454297	       489.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/KeyValues-10                           	  628093	       371.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/Fields-10                              	  645204	       363.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/SlogAttrs-10                           	  707668	       342.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/ConstructedKeyValues-10                	  585044	       406.1 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/ConstructedFields-10                   	  520692	       471.4 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/KeyValues-10                         	 2956918	        78.25 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/Fields-10                            	 3343561	        80.57 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/SlogAttrs-10                         	 3076428	        75.35 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/ConstructedKeyValues-10              	 1969920	       130.8 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/ConstructedFields-10                 	 1458626	       164.3 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/KeyValues-10                           	  410238	       580.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/Fields-10                              	  417254	       571.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/SlogAttrs-10                           	  425072	       567.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/ConstructedKeyValues-10                	  373930	       627.5 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/ConstructedFields-10                   	  353811	       680.8 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/KeyValues-10                         	 2376168	       106.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/Fields-10                            	 2062065	       110.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/SlogAttrs-10                         	 2494646	        95.81 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/ConstructedKeyValues-10              	 1522262	       156.2 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/ConstructedFields-10                 	 1281508	       195.7 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/IntSlice-10                             	  716470	       326.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/StringSlice-10                          	  654456	       364.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Time-10                                 	  718410	       335.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/TimeSlice-10                            	  214402	      1119 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Struct-10                               	  425305	       569.4 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/StructSlice-10                          	   86977	      2745 ns/op	     480 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Error-10                                	  852096	       275.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/IntSlice-10                           	 2801438	        94.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/StringSlice-10                        	 2508759	        80.06 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Time-10                               	 3133472	        86.49 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/TimeSlice-10                          	 1352458	       176.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Struct-10                             	 1786968	       158.0 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/StructSlice-10                        	  351871	       639.2 ns/op	     481 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Error-10                              	 4087213	        63.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/IntSlice-10                             	  356684	       676.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/StringSlice-10                          	  353379	       663.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Time-10                                 	  435792	       557.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/TimeSlice-10                            	   69390	      3452 ns/op	     320 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Struct-10                               	  169034	      1405 ns/op	     152 B/op	       5 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/StructSlice-10                          	  157519	      1496 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Error-10                                	  487429	       489.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/IntSlice-10                           	 1709236	       130.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/StringSlice-10                        	 1643784	       137.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Time-10                               	 2573272	        98.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/TimeSlice-10                          	  385686	       613.3 ns/op	     320 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Struct-10                             	 1000000	       261.1 ns/op	     152 B/op	       5 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/StructSlice-10                        	  936128	       231.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Error-10                              	 2790310	        86.15 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithKeyValues-10                	 1000000	       227.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithFields-10                   	 1000000	       229.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/TimestampValuer-10              	  688286	       342.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/CallerValuer-10                 	  455486	       535.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/DefaultFields-10                	  188882	      1275 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithContext-10                  	 1000000	       220.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/LogContext-10                   	  786584	       304.4 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithKeyValues-10              	 4941291	        49.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithFields-10                 	 4876677	        51.15 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/TimestampValuer-10            	 3584456	        59.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/CallerValuer-10               	 2909660	        88.98 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/DefaultFields-10              	 1477473	       173.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithContext-10                	 5542521	        52.79 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/LogContext-10                 	 3055982	        84.22 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithKeyValues-10                	  536956	       446.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithFields-10                   	  534579	       442.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/TimestampValuer-10              	  434448	       557.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/CallerValuer-10                 	  373005	       640.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/DefaultFields-10                	  180445	      1326 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithContext-10                  	  549795	       438.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/LogContext-10                   	  465883	       525.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithKeyValues-10              	 3363856	        72.99 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithFields-10                 	 3109813	        73.41 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/TimestampValuer-10            	 2783968	       100.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/CallerValuer-10               	 2307136	       108.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/DefaultFields-10              	 1363358	       172.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithContext-10                	 3397720	        70.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/LogContext-10                 	 2153455	       111.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/WithGroup/Depth1-10                        	  784944	       306.5 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/WithGroup/Depth3-10                        	  775736	       308.8 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/GroupField-10                              	  615325	       392.6 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/WithGroup/Depth1-10                      	 2707266	        95.32 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/WithGroup/Depth3-10                      	 2735574	        95.71 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/GroupField-10                            	 2189011	       118.0 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/Text/Serial/WithGroup/Depth1-10                        	  442117	       544.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Serial/WithGroup/Depth3-10                        	  430090	       556.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Serial/GroupField-10                              	  370458	       644.3 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/Text/Parallel/WithGroup/Depth1-10                      	 2211667	       116.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Parallel/WithGroup/Depth3-10                      	 2128758	       116.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Parallel/GroupField-10                            	 1631749	       155.0 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerReplacer/JSON/Serial-10                                       	  600831	       403.1 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/JSON/Parallel-10                                     	 1904146	       126.6 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/Text/Serial-10                                       	  400363	       611.4 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/Text/Parallel-10                                     	 1671841	       141.0 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerConstruction/NewJSON-10                                       	 2450402	        97.38 ns/op	     208 B/op	       5 allocs/op
BenchmarkNexuerConstruction/NewText-10                                       	 2490956	        98.27 ns/op	     208 B/op	       5 allocs/op
BenchmarkNexuerConstruction/WithKeyValues-10                                 	  535708	       418.3 ns/op	     504 B/op	      11 allocs/op
BenchmarkNexuerConstruction/WithFields-10                                    	  702129	       323.4 ns/op	     360 B/op	       9 allocs/op
BenchmarkNexuerConstruction/WithGroup-10                                     	 2691344	        89.07 ns/op	     200 B/op	       4 allocs/op
BenchmarkNexuerConstruction/WithGroupDepth3-10                               	  861024	       286.5 ns/op	     664 B/op	      12 allocs/op
BenchmarkNexuerConstruction/WithContext-10                                   	10217547	        23.97 ns/op	      64 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Message/Standard-10                        	  634150	       380.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Message/Nexuer-10                          	  551337	       427.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/1/Standard-10                       	  557005	       434.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/1/Nexuer-10                         	  484746	       485.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/5/Standard-10                       	  395391	       600.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/5/Nexuer-10                         	  379186	       644.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/10/Standard-10                      	  267613	       900.0 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/10/Nexuer-10                        	  256426	       915.0 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/25/Standard-10                      	  146937	      1634 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/25/Nexuer-10                        	  150825	      1604 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/50/Standard-10                      	   85867	      2840 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/50/Nexuer-10                        	   88214	      2677 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/IntSlice/Standard-10                   	  367075	       665.6 ns/op	     112 B/op	       2 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/IntSlice/Nexuer-10                     	  426199	       561.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/TimeSlice/Standard-10                  	   89322	      2623 ns/op	     817 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/TimeSlice/Nexuer-10                    	  170920	      1443 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/StructSlice/Standard-10                	   76194	      3170 ns/op	    1426 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/StructSlice/Nexuer-10                  	   80010	      3002 ns/op	     480 B/op	      10 allocs/op
BenchmarkSlogHandlers/JSON/Serial/WithAttrs/5/Standard-10                    	  626611	       383.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/WithAttrs/5/Nexuer-10                      	  550694	       437.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Group/Depth3/Standard-10                   	  480195	       495.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Group/Depth3/Nexuer-10                     	  432087	       553.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Message/Standard-10                      	 1000000	       205.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Message/Nexuer-10                        	 1000000	       213.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/1/Standard-10                     	 1000000	       233.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/1/Nexuer-10                       	  935900	       216.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/5/Standard-10                     	  916015	       237.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/5/Nexuer-10                       	  873573	       252.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/10/Standard-10                    	  782554	       331.0 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/10/Nexuer-10                      	  903373	       316.5 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/25/Standard-10                    	  415830	       648.3 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/25/Nexuer-10                      	  371184	       594.5 ns/op	     898 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/50/Standard-10                    	  225240	      1172 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/50/Nexuer-10                      	  221577	      1093 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/IntSlice/Standard-10                 	  944634	       283.0 ns/op	     112 B/op	       2 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/IntSlice/Nexuer-10                   	 1000000	       224.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/TimeSlice/Standard-10                	  343412	       713.6 ns/op	     819 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/TimeSlice/Nexuer-10                  	  855830	       282.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/StructSlice/Standard-10              	  273542	       949.5 ns/op	    1429 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/StructSlice/Nexuer-10                	  389583	       690.9 ns/op	     482 B/op	      10 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/WithAttrs/5/Standard-10                  	 1000000	       216.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/WithAttrs/5/Nexuer-10                    	 1000000	       220.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Group/Depth3/Standard-10                 	 1000000	       218.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Group/Depth3/Nexuer-10                   	  959168	       225.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Message/Standard-10                        	  391327	       612.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Message/Nexuer-10                          	  372962	       643.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/1/Standard-10                       	  354524	       671.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/1/Nexuer-10                         	  343695	       697.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/5/Standard-10                       	  284668	       854.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/5/Nexuer-10                         	  279907	       859.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/10/Standard-10                      	  210972	      1146 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/10/Nexuer-10                        	  211314	      1131 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/25/Standard-10                      	  125415	      1907 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/25/Nexuer-10                        	  135002	      1794 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/50/Standard-10                      	   73562	      3181 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/50/Nexuer-10                        	   79945	      2998 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/IntSlice/Standard-10                   	  180595	      1324 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/IntSlice/Nexuer-10                     	  263479	       912.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/TimeSlice/Standard-10                  	   55010	      4417 ns/op	     881 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/TimeSlice/Nexuer-10                    	   64390	      3732 ns/op	     320 B/op	      10 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/StructSlice/Standard-10                	  140949	      1730 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/StructSlice/Nexuer-10                  	  138366	      1730 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/WithAttrs/5/Standard-10                    	  371492	       617.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/WithAttrs/5/Nexuer-10                      	  371996	       655.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Group/Depth3/Standard-10                   	  330276	       733.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Group/Depth3/Nexuer-10                     	  310851	       768.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Message/Standard-10                      	 1000000	       214.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Message/Nexuer-10                        	  995470	       224.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/1/Standard-10                     	  981930	       235.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/1/Nexuer-10                       	  983253	       239.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/5/Standard-10                     	  947289	       247.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/5/Nexuer-10                       	  978309	       241.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/10/Standard-10                    	  872515	       347.0 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/10/Nexuer-10                      	  816244	       339.6 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/25/Standard-10                    	  330872	       674.3 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/25/Nexuer-10                      	  384609	       661.8 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/50/Standard-10                    	  207294	      1227 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/50/Nexuer-10                      	  196641	      1247 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/IntSlice/Standard-10                 	  825261	       319.0 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/IntSlice/Nexuer-10                   	  883922	       269.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/TimeSlice/Standard-10                	  272038	       916.4 ns/op	     883 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/TimeSlice/Nexuer-10                  	  358246	       656.9 ns/op	     320 B/op	      10 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/StructSlice/Standard-10              	  798490	       351.8 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/StructSlice/Nexuer-10                	  678556	       327.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/WithAttrs/5/Standard-10                  	 1000000	       235.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/WithAttrs/5/Nexuer-10                    	 1000000	       217.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Group/Depth3/Standard-10                 	  953404	       261.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Group/Depth3/Nexuer-10                   	  938691	       246.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/DefaultFields-10                	  150625	      1572 ns/op	       8 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/LoggerDepth1-10                 	  144938	      1658 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/CallDepth1-10                   	  158548	      1522 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/MergedDepth2-10                 	  137514	      1755 ns/op	      48 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/DefaultFields-10              	  798535	       295.2 ns/op	       8 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/LoggerDepth1-10               	  946677	       249.1 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/CallDepth1-10                 	  874024	       265.6 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/MergedDepth2-10               	  943597	       262.6 ns/op	      48 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/DefaultFields-10                	  149156	      1659 ns/op	       8 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/LoggerDepth1-10                 	  135452	      1730 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/CallDepth1-10                   	  151218	      1583 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/MergedDepth2-10                 	  133636	      1797 ns/op	      48 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/DefaultFields-10              	  742918	       281.6 ns/op	       8 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/LoggerDepth1-10               	  789938	       272.7 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/CallDepth1-10                 	  815366	       266.5 ns/op	      32 B/op	       2 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/MergedDepth2-10               	  832730	       277.9 ns/op	      48 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Short/Nexuer-10          	 1000000	       219.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Short/Slog-10            	  539344	       446.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Long4K/Nexuer-10         	   59244	      4047 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Long4K/Slog-10           	   55280	      4365 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Escaped/Nexuer-10        	 1000000	       220.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Escaped/Slog-10          	  540063	       444.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Disabled/Nexuer-10               	75729678	         3.202 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Disabled/Slog-10                 	46399603	         5.201 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/1/Nexuer-10               	  894034	       271.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/1/Slog-10                 	  486589	       491.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/2/Nexuer-10               	  781992	       312.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/2/Slog-10                 	  444896	       536.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/5/Nexuer-10               	  564019	       428.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/5/Slog-10                 	  364038	       662.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/10/Nexuer-10              	  389422	       618.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/10/Slog-10                	  250993	       969.2 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/25/Nexuer-10              	  199479	      1190 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/25/Slog-10                	  141366	      1674 ns/op	     896 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/50/Nexuer-10              	  113346	      2115 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/50/Slog-10                	   82681	      2945 ns/op	    2049 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/IntSlice/Nexuer-10           	  743714	       324.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/IntSlice/Slog-10             	  333771	       724.0 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StringSlice/Nexuer-10        	  667670	       365.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StringSlice/Slog-10          	  308958	       760.4 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Time/Nexuer-10               	  717577	       334.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Time/Slog-10                 	  442063	       546.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/TimeSlice/Nexuer-10          	  212247	      1122 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/TimeSlice/Slog-10            	   88698	      2692 ns/op	     817 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Struct/Nexuer-10             	  416880	       572.4 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Struct/Slog-10               	  261962	       882.7 ns/op	     176 B/op	       3 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StructSlice/Nexuer-10        	   86169	      2755 ns/op	     480 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StructSlice/Slog-10          	   75711	      3220 ns/op	    1426 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Error/Nexuer-10              	  879966	       273.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Error/Slog-10                	  442497	       510.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/AccumulatedFields/5/Nexuer-10    	 1000000	       227.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/AccumulatedFields/5/Slog-10      	  543795	       442.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth1/Nexuer-10           	  759532	       319.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth1/Slog-10             	  432424	       542.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth3/Nexuer-10           	  675913	       359.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth3/Slog-10             	  414033	       593.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Context/Nexuer-10                	  816198	       301.8 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Context/Slog-10                  	  462939	       520.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Short/Nexuer-10        	 5604126	        47.59 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Short/Slog-10          	 1000000	       233.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Long4K/Nexuer-10       	  464958	       510.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Long4K/Slog-10         	  421437	       573.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Escaped/Nexuer-10      	 5271445	        47.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Escaped/Slog-10        	 1000000	       239.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Disabled/Nexuer-10             	609230480	         0.3957 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Disabled/Slog-10               	371621211	         1.305 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/1/Nexuer-10             	 4390413	        58.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/1/Slog-10               	 1000000	       236.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/2/Nexuer-10             	 4098696	        59.39 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/2/Slog-10               	 1000000	       242.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/5/Nexuer-10             	 2673991	        89.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/5/Slog-10               	  969859	       243.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/10/Nexuer-10            	 2002597	       132.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/10/Slog-10              	  752695	       335.3 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/25/Nexuer-10            	 1000000	       235.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/25/Slog-10              	  366640	       673.0 ns/op	     897 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/50/Nexuer-10            	  593802	       360.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/50/Slog-10              	  187040	      1249 ns/op	    2052 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/IntSlice/Nexuer-10         	 3481464	        74.50 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/IntSlice/Slog-10           	  956031	       278.0 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StringSlice/Nexuer-10      	 3522162	        78.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StringSlice/Slog-10        	  873070	       284.9 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Time/Nexuer-10             	 3109940	        72.84 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Time/Slog-10               	 1000000	       233.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/TimeSlice/Nexuer-10        	 1421872	       162.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/TimeSlice/Slog-10          	  380815	       740.0 ns/op	     819 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Struct/Nexuer-10           	 1671354	       150.2 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Struct/Slog-10             	  825936	       306.3 ns/op	     176 B/op	       3 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StructSlice/Nexuer-10      	  417620	       618.2 ns/op	     481 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StructSlice/Slog-10        	  268312	       957.9 ns/op	    1429 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Error/Nexuer-10            	 4566759	        52.99 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Error/Slog-10              	 1000000	       223.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/AccumulatedFields/5/Nexuer-10  	 5050159	        49.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/AccumulatedFields/5/Slog-10    	 1000000	       212.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth1/Nexuer-10         	 2905513	        91.57 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth1/Slog-10           	  907786	       249.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth3/Nexuer-10         	 2468522	       102.0 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth3/Slog-10           	  869433	       258.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Context/Nexuer-10              	 2919469	        82.78 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Context/Slog-10                	  923209	       240.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Short/Nexuer-10          	  554698	       435.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Short/Slog-10            	  345216	       684.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Long4K/Nexuer-10         	   13400	     17925 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Long4K/Slog-10           	   12885	     18229 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Escaped/Nexuer-10        	  645283	       374.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Escaped/Slog-10          	  377330	       633.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Disabled/Nexuer-10               	75216458	         3.221 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Disabled/Slog-10                 	44929095	         5.191 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/1/Nexuer-10               	  498256	       482.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/1/Slog-10                 	  318349	       740.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/2/Nexuer-10               	  460008	       525.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/2/Slog-10                 	  304672	       783.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/5/Nexuer-10               	  373322	       642.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/5/Slog-10                 	  261548	       918.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/10/Nexuer-10              	  287584	       836.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/10/Slog-10                	  196280	      1226 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/25/Nexuer-10              	  168597	      1427 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/25/Slog-10                	  117656	      2008 ns/op	     896 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/50/Nexuer-10              	   99104	      2400 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/50/Slog-10                	   72870	      3281 ns/op	    2049 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/IntSlice/Nexuer-10           	  355171	       677.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/IntSlice/Slog-10             	  171796	      1409 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StringSlice/Nexuer-10        	  364704	       665.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StringSlice/Slog-10          	  170569	      1401 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Time/Nexuer-10               	  429222	       556.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Time/Slog-10                 	  289106	       805.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/TimeSlice/Nexuer-10          	   69471	      3469 ns/op	     320 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/TimeSlice/Slog-10            	   53424	      4549 ns/op	     881 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Struct/Nexuer-10             	  173340	      1410 ns/op	     152 B/op	       5 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Struct/Slog-10               	  141236	      1690 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StructSlice/Nexuer-10        	  160856	      1483 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StructSlice/Slog-10          	  134788	      1791 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Error/Nexuer-10              	  498622	       488.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Error/Slog-10                	  289447	       822.0 ns/op	       4 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/AccumulatedFields/5/Nexuer-10    	  541399	       444.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/AccumulatedFields/5/Slog-10      	  350078	       682.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth1/Nexuer-10           	  440845	       545.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth1/Slog-10             	  303409	       790.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth3/Nexuer-10           	  417514	       581.6 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth3/Slog-10             	  291039	       826.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Context/Nexuer-10                	  465886	       520.1 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Context/Slog-10                  	  312182	       771.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Short/Nexuer-10        	 3235482	        69.45 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Short/Slog-10          	 1000000	       224.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Long4K/Nexuer-10       	  105646	      2213 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Long4K/Slog-10         	  106098	      2282 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Escaped/Nexuer-10      	 3770156	        66.83 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Escaped/Slog-10        	 1000000	       246.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Disabled/Nexuer-10             	608631782	         0.7893 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Disabled/Slog-10               	179567858	         1.324 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/1/Nexuer-10             	 2970192	        80.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/1/Slog-10               	  997782	       245.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/2/Nexuer-10             	 2588043	       100.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/2/Slog-10               	  927864	       238.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/5/Nexuer-10             	 2153113	       110.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/5/Slog-10               	  843906	       255.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/10/Nexuer-10            	 1862552	       131.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/10/Slog-10              	  812307	       351.7 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/25/Nexuer-10            	  783417	       257.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/25/Slog-10              	  384562	       672.5 ns/op	     898 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/50/Nexuer-10            	  565526	       567.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/50/Slog-10              	  197034	      1257 ns/op	    2052 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/IntSlice/Nexuer-10         	 1693546	       130.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/IntSlice/Slog-10           	  736836	       332.1 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StringSlice/Nexuer-10      	 1788394	       137.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StringSlice/Slog-10        	  758642	       379.6 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Time/Nexuer-10             	 2489692	       110.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Time/Slog-10               	  922958	       248.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/TimeSlice/Nexuer-10        	  396066	       622.2 ns/op	     320 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/TimeSlice/Slog-10          	  269595	       939.2 ns/op	     882 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Struct/Nexuer-10           	 1000000	       274.3 ns/op	     152 B/op	       5 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Struct/Slog-10             	  633558	       363.3 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StructSlice/Nexuer-10      	  946978	       242.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StructSlice/Slog-10        	  706522	       381.8 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Error/Nexuer-10            	 2797371	        90.22 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Error/Slog-10              	  951342	       277.2 ns/op	       4 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/AccumulatedFields/5/Nexuer-10  	 3157117	        72.56 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/AccumulatedFields/5/Slog-10    	  929226	       241.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth1/Nexuer-10         	 2274340	       119.1 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth1/Slog-10           	  974799	       260.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth3/Nexuer-10         	 2063085	       119.9 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth3/Slog-10           	  885308	       279.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Context/Nexuer-10              	 2140468	       104.9 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Context/Slog-10                	  917109	       256.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Nexuer-10                      	66572650	         3.546 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Zap-10                         	47023094	         5.102 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Zerolog-10                     	75906320	         3.198 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Phuslu-10                      	84176058	         2.874 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Slog-10                        	43640989	         5.431 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Logrus-10                      	13809372	        17.18 ns/op	      16 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Nexuer-10                    	547845654	         0.4381 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Zap-10                       	369173263	         0.7391 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Zerolog-10                   	448262629	         0.5201 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Phuslu-10                    	529581580	         0.3915 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Slog-10                      	319763196	         0.7553 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Logrus-10                    	39734829	         7.482 ns/op	      16 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Nexuer-10                       	 1000000	       224.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Zap-10                          	  918396	       260.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Zerolog-10                      	 1472230	       163.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Phuslu-10                       	 1834969	       130.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Slog-10                         	  552416	       429.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Logrus-10                       	  212074	      1110 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Nexuer-10                     	 5724075	        42.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Zap-10                        	 4656674	        52.13 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Zerolog-10                    	10081262	        28.53 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Phuslu-10                     	12418342	        20.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Slog-10                       	 1000000	       228.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Logrus-10                     	  163490	      1439 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Nexuer-10             	  941606	       245.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Zap-10                	  835714	       283.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Zerolog-10            	 1313890	       182.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Phuslu-10             	 1524880	       153.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Slog-10               	  499786	       480.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Logrus-10             	   25670	      9156 ns/op	    3744 B/op	      69 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Nexuer-10           	 4506621	        53.81 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Zap-10              	 5920930	        54.43 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Zerolog-10          	10138399	        31.69 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Phuslu-10           	10716298	        23.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Slog-10             	 1000000	       216.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Logrus-10           	   22831	     10421 ns/op	    3763 B/op	      69 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Nexuer-10                	   48870	      4942 ns/op	     577 B/op	      12 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Zap-10                   	  122661	      1956 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Zerolog-10               	  120526	      2023 ns/op	     184 B/op	      11 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Phuslu-10                	  148309	      1596 ns/op	      24 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Slog-10                  	   32410	      7429 ns/op	    3031 B/op	      35 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Logrus-10                	   24228	      9905 ns/op	    4576 B/op	      75 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Nexuer-10              	  254049	       966.4 ns/op	     578 B/op	      12 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Zap-10                 	  800644	       260.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Zerolog-10             	  766038	       419.6 ns/op	     185 B/op	      11 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Phuslu-10              	 1000000	       218.6 ns/op	      24 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Slog-10                	  112538	      2233 ns/op	    3040 B/op	      35 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Logrus-10              	   21019	     11171 ns/op	    4595 B/op	      75 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Nexuer-10                        	 1000000	       225.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Zap-10                           	  764412	       311.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Zerolog-10                       	 1458908	       164.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Phuslu-10                        	 1811871	       131.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Slog-10                          	  541261	       436.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Logrus-10                        	  213465	      1114 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Nexuer-10                      	 1000000	       207.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Zap-10                         	 4961098	        43.18 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Zerolog-10                     	 9466242	        29.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Phuslu-10                      	10705682	        24.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Slog-10                        	 1000000	       222.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Logrus-10                      	  163712	      1445 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Nexuer-10                    	  589540	       408.1 ns/op	      96 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Zap-10                       	  541714	       441.3 ns/op	     192 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Zerolog-10                   	 1000000	       212.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Phuslu-10                    	 1525304	       157.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Slog-10                      	  401091	       604.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Logrus-10                    	  121593	      1968 ns/op	    1884 B/op	      30 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Nexuer-10                  	  998110	       254.6 ns/op	      96 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Zap-10                     	 1648094	       145.8 ns/op	     192 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Zerolog-10                 	 8659309	        35.73 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Phuslu-10                  	10937906	        21.27 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Slog-10                    	  848980	       265.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Logrus-10                  	  100764	      2395 ns/op	    1887 B/op	      30 allocs/op
PASS
ok  	github.com/nexuer/log/benchmarks	150.573s
```
