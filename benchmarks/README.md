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

Run Nexuer JSON-focused scenarios before and after an encoder change:

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

Use `benchstat` on saved before and after output for optimization decisions.

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

## Performance Assessment

The 2026-07-10 Apple M1 Pro snapshot supports treating Nexuer as a mature
performance-oriented logger for structured JSON, native text records, and
replacement of the standard `slog` handler:

- JSON message and accumulated-field encode-only paths are allocation-free and
  competitive with zerolog, while substantially ahead of zap and standard slog
  in this fixture.
- Behind the same `slog.Logger`, Nexuer is 1-10% faster than the standard JSON
  and text handlers for messages, primitive fields, pre-attached attributes,
  and groups. The JSON `[]time.Time` case is about 2.1x faster and removes all
  12 standard-handler allocations.
- Native `DefaultTimestamp`, `DefaultCaller`, and the combined `DefaultFields`
  are allocation-free. The repeated eight-sample optimization comparison
  reduced their geometric-mean latency by 15.41% and removed 32-328 B/op.
- Disabled structured calls are 3-4 ns/op with no allocations. Primitive JSON
  fields, common slices, times, errors, and accumulated fields are also
  allocation-free in the measured encode-only paths.
- Common text slices now bypass `fmt` reflection without changing their `%+v`
  representation. `[]int` and `[]string` are allocation-free; `[]time.Time`
  retains only the allocations required by the standard `Time.String` method.
- The writer path is synchronized by Nexuer itself. Serial performance remains
  competitive, but parallel no-op-writer throughput is intentionally below
  libraries that do not serialize writes.

The remaining performance limitations are explicit rather than hidden:

- generic call-site values that fall through `encoding/json` still pay encoder
  reflection and allocation costs;
- arbitrary named slices and nested values that miss the internal text fast
  paths still use `fmt.Appendf` and reflection;
- composing non-background slog caller-depth contexts costs 24 B and one
  allocation; Logger-only and call-only depth paths remain allocation-free;
- parallel WritePath results measure the internal mutex policy. Removing that
  mutex would change the writer concurrency contract and is not a valid
  benchmark-only optimization.

Useful follow-up work is to reduce generic call-site field conversion. A custom
reflection encoder is intentionally out of scope because output compatibility
is more important than eliminating the remaining fallback costs.

## Text Any Optimization: 2026-07-11

The text fast paths preserve the exact previous output, including nil and empty
slices, quoting and escaping, NaN/Inf, time zones, and monotonic clock suffixes.
Representative Apple M1 Pro serial results are:

| Value | Before | After | Result |
| --- | ---: | ---: | --- |
| `[]int` | 1.101 us, 104 B, 11 allocs | 681 ns, 0 B, 0 allocs | about 38% faster |
| `[]string` | 1.087 us, 184 B, 11 allocs | 643 ns, 0 B, 0 allocs | about 41% faster |
| `[]time.Time` | 4.279 us, 881 B, 21 allocs | 3.576 us, 320 B, 10 allocs | about 16% faster |
| struct fallback | 1.375 us, 232 B, 6 allocs | about 1.4 us, 152 B, 5 allocs | latency neutral |
| struct-slice fallback | 1.500 us, 128 B, 1 alloc | about 1.5 us, 0 B, 0 allocs | latency neutral |

Post-change allocation profiling attributes 99.92% of the remaining
`[]time.Time` allocations to `time.Time.Format`, called by the standard
`Time.String` implementation. Replacing it with RFC3339 would remove those
allocations but change observable text output, especially for monotonic times,
so the compatible implementation is retained.

## Full Result: 2026-07-10

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
BenchmarkComparisonTextEncodeOnly/Message/Serial/Nexuer-10         	  604026	       391.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Zap-10            	  810776	       303.0 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Zerolog-10        	   88978	      2681 ns/op	    1786 B/op	      31 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Phuslu-10         	  316094	       760.8 ns/op	     897 B/op	       5 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Slog-10           	  332736	       707.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Serial/Logrus-10         	  210102	      1133 ns/op	     472 B/op	      14 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Nexuer-10       	 3402668	        75.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Zap-10          	 2031652	       124.9 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Zerolog-10      	  241894	      1022 ns/op	    1788 B/op	      31 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Phuslu-10       	  594586	       417.3 ns/op	     898 B/op	       5 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Slog-10         	  931495	       220.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/Message/Parallel/Logrus-10       	  165169	      1496 ns/op	     473 B/op	      14 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Nexuer-10     	  441612	       545.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Zap-10        	  625929	       382.6 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Zerolog-10    	   59170	      4136 ns/op	    2292 B/op	      62 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Phuslu-10     	  228542	      1034 ns/op	     993 B/op	      11 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Slog-10       	  266685	       900.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Serial/Logrus-10     	   19156	     12439 ns/op	    5185 B/op	      95 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Nexuer-10   	 2363874	       107.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Zap-10      	 1481779	       138.9 ns/op	      24 B/op	       2 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Zerolog-10  	  138348	      1706 ns/op	    2295 B/op	      62 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Phuslu-10   	  462478	       541.8 ns/op	     994 B/op	      11 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Slog-10     	 1000000	       257.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonTextEncodeOnly/ShortFields/Parallel/Logrus-10   	   16434	     14466 ns/op	    5198 B/op	      95 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/SlogJSON-10       	  409514	       594.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/PhusluSlogJSON-10 	  673644	       355.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Serial/NexuerJSON-10     	  820208	       293.8 ns/op	      64 B/op	       1 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/SlogJSON-10     	  981460	       241.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/PhusluSlogJSON-10         	 4612693	        51.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/WithGroup/Parallel/NexuerJSON-10             	 2446893	       113.0 ns/op	      64 B/op	       1 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/SlogJSON/LogAttrs-10            	  413385	       593.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/PhusluSlogJSON/LogAttrs-10      	  688599	       340.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Serial/NexuerJSON/InfoS-10             	  736120	       325.7 ns/op	     128 B/op	       3 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/SlogJSON/LogAttrs-10          	  882920	       254.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/PhusluSlogJSON/LogAttrs-10    	 4511083	        51.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonSlogFeatures/Attrs/Parallel/NexuerJSON/InfoS-10           	 1759111	       128.0 ns/op	     128 B/op	       3 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Short-10                           	 1436078	       165.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Long4K-10                          	   57958	      4153 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/InfoS/Escaped-10                         	 1479550	       163.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Info/SingleString-10                     	 1272404	       189.7 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Infof/NoArgs-10                          	 1448284	       166.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Serial/Infof/Formatting-10                      	   41524	      5739 ns/op	    1779 B/op	      52 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Short-10                         	 6585370	        43.05 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Long4K-10                        	  393300	       614.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/InfoS/Escaped-10                       	 5758807	        42.49 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Info/SingleString-10                   	 4012801	        60.88 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Infof/NoArgs-10                        	 6061198	        52.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/JSON/Parallel/Infof/Formatting-10                    	  144788	      1791 ns/op	    1782 B/op	      52 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Short-10                           	  597045	       390.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Long4K-10                          	   12934	     18819 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/InfoS/Escaped-10                         	  745041	       321.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/Info/SingleString-10                     	  589958	       408.5 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/Text/Serial/Infof/NoArgs-10                          	  619827	       390.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Serial/Infof/Formatting-10                      	   29876	      8028 ns/op	    1779 B/op	      52 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Short-10                         	 3570564	        64.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Long4K-10                        	  101901	      2577 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/InfoS/Escaped-10                       	 3961190	        57.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Info/SingleString-10                   	 2985108	        89.76 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Infof/NoArgs-10                        	 3010402	        70.68 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerMessages/Text/Parallel/Infof/Formatting-10                    	  111154	      2222 ns/op	    1784 B/op	      52 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS-10                                 	65623076	         3.645 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/Info-10                                  	12932718	        18.41 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS/PrebuiltFields10-10                	70950814	         3.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Serial/InfoS/ConstructedFields-10               	 6452754	        40.62 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS-10                               	407207283	         0.6507 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/Info-10                                	30101562	         7.929 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS/PrebuiltFields10-10              	523991725	         0.4780 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/JSON/Parallel/InfoS/ConstructedFields-10             	 5771200	        42.13 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS-10                                 	65986945	         3.650 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Serial/Info-10                                  	12920911	        18.39 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS/PrebuiltFields10-10                	72311851	         3.316 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Serial/InfoS/ConstructedFields-10               	 6404283	        37.68 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS-10                               	477262497	         0.5272 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/Info-10                                	29837756	         8.444 ns/op	      16 B/op	       1 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS/PrebuiltFields10-10              	508242625	         0.5162 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerDisabled/Text/Parallel/InfoS/ConstructedFields-10             	 5062538	        44.39 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields1-10                             	 1000000	       218.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields2-10                             	  932702	       256.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields5-10                             	  643064	       376.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields10-10                            	  432266	       558.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields25-10                            	  213872	      1122 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Serial/Fields50-10                            	  117204	      2042 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields1-10                           	 4319330	        57.11 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields2-10                           	 3444808	        67.78 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields5-10                           	 2976625	        88.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields10-10                          	 1841397	       135.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields25-10                          	 1231245	       230.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/JSON/Parallel/Fields50-10                          	  559179	       389.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields1-10                             	  541083	       441.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields2-10                             	  497997	       485.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields5-10                             	  400759	       604.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields10-10                            	  303050	       792.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields25-10                            	  173397	      1385 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Serial/Fields50-10                            	  101449	      2365 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields1-10                           	 2711595	        89.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields2-10                           	 2348736	       114.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields5-10                           	 2091368	       122.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields10-10                          	 1555005	       155.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields25-10                          	  833416	       302.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldScale/Text/Parallel/Fields50-10                          	  470061	       512.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/KeyValues-10                           	  721514	       340.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/Fields-10                              	  742334	       320.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/SlogAttrs-10                           	  781398	       301.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/ConstructedKeyValues-10                	  684711	       361.7 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/JSON/Serial/ConstructedFields-10                   	  583834	       421.2 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/KeyValues-10                         	 3436336	        79.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/Fields-10                            	 3021736	        86.38 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/SlogAttrs-10                         	 3444279	        67.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/ConstructedKeyValues-10              	 1956933	       138.0 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/JSON/Parallel/ConstructedFields-10                 	 1419061	       172.1 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/KeyValues-10                           	  433036	       547.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/Fields-10                              	  445599	       540.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/SlogAttrs-10                           	  453289	       532.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/ConstructedKeyValues-10                	  412117	       582.0 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/Text/Serial/ConstructedFields-10                   	  390738	       640.6 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/KeyValues-10                         	 2063425	       125.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/Fields-10                            	 2357930	       104.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/SlogAttrs-10                         	 2466162	       102.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/ConstructedKeyValues-10              	 1442746	       156.3 ns/op	      96 B/op	       1 allocs/op
BenchmarkNexuerFieldForms/Text/Parallel/ConstructedFields-10                 	 1208043	       195.7 ns/op	     192 B/op	       4 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/IntSlice-10                             	  867364	       276.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/StringSlice-10                          	  749833	       320.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Time-10                                 	  858586	       280.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/TimeSlice-10                            	  236204	      1026 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Struct-10                               	  463446	       564.2 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/StructSlice-10                          	   85813	      2796 ns/op	     480 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/JSON/Serial/Error-10                                	 1000000	       224.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/IntSlice-10                           	 3510576	        72.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/StringSlice-10                        	 2770802	        82.20 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Time-10                               	 3347425	        71.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/TimeSlice-10                          	 1389482	       182.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Struct-10                             	 1747910	       142.2 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/StructSlice-10                        	  328765	       686.8 ns/op	     481 B/op	      10 allocs/op
BenchmarkNexuerAnyValues/JSON/Parallel/Error-10                              	 3869404	        56.56 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/IntSlice-10                             	  217712	      1104 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/StringSlice-10                          	  222721	      1078 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Time-10                                 	  461426	       521.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/TimeSlice-10                            	   56906	      4232 ns/op	     881 B/op	      21 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Struct-10                               	  174494	      1375 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/StructSlice-10                          	  161431	      1500 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/Text/Serial/Error-10                                	  528129	       448.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/IntSlice-10                           	 1000000	       285.2 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/StringSlice-10                        	 1000000	       309.2 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Time-10                               	 2466398	        98.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/TimeSlice-10                          	  259482	       978.7 ns/op	     882 B/op	      21 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Struct-10                             	  945470	       336.8 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/StructSlice-10                        	  816140	       345.1 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerAnyValues/Text/Parallel/Error-10                              	 3051100	        86.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithKeyValues-10                	 1347590	       174.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithFields-10                   	 1373487	       175.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/TimestampValuer-10              	  817879	       293.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/CallerValuer-10                 	  476635	       507.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/DefaultFields-10                	  313597	       753.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/WithContext-10                  	 1442784	       166.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Serial/LogContext-10                   	  914314	       257.6 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithKeyValues-10              	 5519788	        44.38 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithFields-10                 	 5275188	        48.93 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/TimestampValuer-10            	 3453612	        66.35 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/CallerValuer-10               	 2561136	        96.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/DefaultFields-10              	 1951224	       140.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/WithContext-10                	 6391974	        41.19 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/JSON/Parallel/LogContext-10                 	 2999017	        83.86 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithKeyValues-10                	  609265	       395.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithFields-10                   	  608802	       397.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/TimestampValuer-10              	  457581	       522.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/CallerValuer-10                 	  398516	       598.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/DefaultFields-10                	  273540	       872.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/WithContext-10                  	  615232	       390.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Serial/LogContext-10                   	  499786	       478.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithKeyValues-10              	 3271645	        72.04 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithFields-10                 	 3309586	        71.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/TimestampValuer-10            	 2363793	        98.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/CallerValuer-10               	 2240647	       108.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/DefaultFields-10              	 1633720	       149.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/WithContext-10                	 3529071	        71.97 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerAccumulatedFields/Text/Parallel/LogContext-10                 	 2284136	       120.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/WithGroup/Depth1-10                        	  942639	       257.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/WithGroup/Depth3-10                        	  933987	       260.5 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Serial/GroupField-10                              	  714595	       343.7 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/WithGroup/Depth1-10                      	 3033394	        83.76 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/WithGroup/Depth3-10                      	 3006913	        87.96 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/JSON/Parallel/GroupField-10                            	 2005488	       124.0 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/Text/Serial/WithGroup/Depth1-10                        	  474097	       502.6 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Serial/WithGroup/Depth3-10                        	  470895	       516.3 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Serial/GroupField-10                              	  401270	       602.7 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerGroups/Text/Parallel/WithGroup/Depth1-10                      	 2070768	       119.8 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Parallel/WithGroup/Depth3-10                      	 2043286	       117.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerGroups/Text/Parallel/GroupField-10                            	 1539091	       159.6 ns/op	      64 B/op	       2 allocs/op
BenchmarkNexuerReplacer/JSON/Serial-10                                       	  705622	       344.5 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/JSON/Parallel-10                                     	 1865448	       128.2 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/Text/Serial-10                                       	  426132	       565.8 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerReplacer/Text/Parallel-10                                     	 1646074	       157.3 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerConstruction/NewJSON-10                                       	 2254860	       108.3 ns/op	     208 B/op	       5 allocs/op
BenchmarkNexuerConstruction/NewText-10                                       	 2179093	       108.3 ns/op	     208 B/op	       5 allocs/op
BenchmarkNexuerConstruction/WithKeyValues-10                                 	  542295	       454.9 ns/op	     504 B/op	      11 allocs/op
BenchmarkNexuerConstruction/WithFields-10                                    	  669447	       367.6 ns/op	     360 B/op	       9 allocs/op
BenchmarkNexuerConstruction/WithGroup-10                                     	 2144619	        96.68 ns/op	     200 B/op	       4 allocs/op
BenchmarkNexuerConstruction/WithGroupDepth3-10                               	  805064	       299.0 ns/op	     664 B/op	      12 allocs/op
BenchmarkNexuerConstruction/WithContext-10                                   	 9235252	        26.17 ns/op	      64 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Message/Standard-10                        	  608011	       393.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Message/Nexuer-10                          	  621592	       387.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/1/Standard-10                       	  536187	       451.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/1/Nexuer-10                         	  546831	       438.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/5/Standard-10                       	  383616	       624.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/5/Nexuer-10                         	  404079	       589.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/10/Standard-10                      	  260600	       932.0 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/10/Nexuer-10                        	  276303	       875.7 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/25/Standard-10                      	  144967	      1677 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/25/Nexuer-10                        	  156728	      1577 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/50/Standard-10                      	   80032	      2932 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Fields/50/Nexuer-10                        	   89306	      2709 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/IntSlice/Standard-10                   	  358498	       683.7 ns/op	     112 B/op	       2 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/IntSlice/Nexuer-10                     	  469735	       504.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/TimeSlice/Standard-10                  	   89184	      2707 ns/op	     817 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/TimeSlice/Nexuer-10                    	  190923	      1277 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/StructSlice/Standard-10                	   73084	      3279 ns/op	    1426 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Any/StructSlice/Nexuer-10                  	   78885	      3026 ns/op	     481 B/op	      10 allocs/op
BenchmarkSlogHandlers/JSON/Serial/WithAttrs/5/Standard-10                    	  606998	       402.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/WithAttrs/5/Nexuer-10                      	  617302	       395.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Group/Depth3/Standard-10                   	  463921	       522.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Serial/Group/Depth3/Nexuer-10                     	  477278	       504.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Message/Standard-10                      	  989095	       215.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Message/Nexuer-10                        	 1000000	       231.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/1/Standard-10                     	 1000000	       231.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/1/Nexuer-10                       	  949209	       244.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/5/Standard-10                     	  931570	       260.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/5/Nexuer-10                       	  901718	       252.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/10/Standard-10                    	  857780	       339.4 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/10/Nexuer-10                      	  928268	       336.8 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/25/Standard-10                    	  347119	       679.9 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/25/Nexuer-10                      	  384442	       624.3 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/50/Standard-10                    	  189314	      1275 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Fields/50/Nexuer-10                      	  223911	      1158 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/IntSlice/Standard-10                 	  916248	       305.7 ns/op	     112 B/op	       2 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/IntSlice/Nexuer-10                   	 1000000	       252.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/TimeSlice/Standard-10                	  299704	       756.2 ns/op	     818 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/TimeSlice/Nexuer-10                  	  796350	       283.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/StructSlice/Standard-10              	  248636	      1013 ns/op	    1429 B/op	      12 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Any/StructSlice/Nexuer-10                	  349843	       727.6 ns/op	     482 B/op	      10 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/WithAttrs/5/Standard-10                  	  961939	       230.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/WithAttrs/5/Nexuer-10                    	 1000000	       231.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Group/Depth3/Standard-10                 	 1000000	       244.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/JSON/Parallel/Group/Depth3/Nexuer-10                   	  963838	       262.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Message/Standard-10                        	  376224	       645.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Message/Nexuer-10                          	  394155	       617.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/1/Standard-10                       	  343334	       694.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/1/Nexuer-10                         	  363012	       668.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/5/Standard-10                       	  269727	       881.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/5/Nexuer-10                         	  291943	       818.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/10/Standard-10                      	  204654	      1192 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/10/Nexuer-10                        	  220870	      1107 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/25/Standard-10                      	  121214	      1998 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/25/Nexuer-10                        	  132004	      1824 ns/op	     896 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/50/Standard-10                      	   72630	      3300 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Fields/50/Nexuer-10                        	   81141	      2976 ns/op	    2049 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/IntSlice/Standard-10                   	  175927	      1367 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/IntSlice/Nexuer-10                     	  179678	      1341 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/TimeSlice/Standard-10                  	   52789	      4537 ns/op	     881 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/TimeSlice/Nexuer-10                    	   52981	      4524 ns/op	     881 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/StructSlice/Standard-10                	  136414	      1780 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/Any/StructSlice/Nexuer-10                  	  137496	      1755 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Serial/WithAttrs/5/Standard-10                    	  370003	       645.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/WithAttrs/5/Nexuer-10                      	  396674	       622.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Group/Depth3/Standard-10                   	  315254	       759.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Serial/Group/Depth3/Nexuer-10                     	  325215	       736.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Message/Standard-10                      	  923965	       233.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Message/Nexuer-10                        	  960144	       242.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/1/Standard-10                     	  940008	       242.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/1/Nexuer-10                       	  968115	       227.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/5/Standard-10                     	  919348	       274.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/5/Nexuer-10                       	  800600	       269.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/10/Standard-10                    	  791730	       435.8 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/10/Nexuer-10                      	  597267	       433.6 ns/op	     208 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/25/Standard-10                    	  379148	       691.1 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/25/Nexuer-10                      	  388734	       684.9 ns/op	     897 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/50/Standard-10                    	  184755	      1352 ns/op	    2052 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Fields/50/Nexuer-10                      	  195669	      1252 ns/op	    2051 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/IntSlice/Standard-10                 	  744127	       347.7 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/IntSlice/Nexuer-10                   	  680480	       356.5 ns/op	     104 B/op	      11 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/TimeSlice/Standard-10                	  245912	      1063 ns/op	     882 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/TimeSlice/Nexuer-10                  	  216946	      1034 ns/op	     882 B/op	      21 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/StructSlice/Standard-10              	  706773	       404.9 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Any/StructSlice/Nexuer-10                	  695089	       414.1 ns/op	     128 B/op	       1 allocs/op
BenchmarkSlogHandlers/Text/Parallel/WithAttrs/5/Standard-10                  	 1000000	       231.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/WithAttrs/5/Nexuer-10                    	 1000000	       224.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Group/Depth3/Standard-10                 	  891088	       260.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSlogHandlers/Text/Parallel/Group/Depth3/Nexuer-10                   	  915331	       262.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/DefaultFields-10                	  173371	      1394 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/LoggerDepth1-10                 	  146488	      1611 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/CallDepth1-10                   	  148648	      1621 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Serial/MergedDepth2-10                 	  137823	      1740 ns/op	      24 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/DefaultFields-10              	  727492	       302.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/LoggerDepth1-10               	  765204	       317.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/CallDepth1-10                 	  769590	       308.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/JSON/Parallel/MergedDepth2-10               	  764512	       275.4 ns/op	      24 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/DefaultFields-10                	  163201	      1488 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/LoggerDepth1-10                 	  142641	      1672 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/CallDepth1-10                   	  142848	      1674 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Serial/MergedDepth2-10                 	  127510	      1882 ns/op	      24 B/op	       1 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/DefaultFields-10              	  719774	       314.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/LoggerDepth1-10               	  715501	       316.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/CallDepth1-10                 	  736054	       302.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerSlogDefaultFields/Text/Parallel/MergedDepth2-10               	  787336	       305.7 ns/op	      24 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Short/Nexuer-10          	 1446706	       166.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Short/Slog-10            	  510307	       473.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Long4K/Nexuer-10         	   58050	      4134 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Long4K/Slog-10           	   54000	      4455 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Escaped/Nexuer-10        	 1480287	       162.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Message/Escaped/Slog-10          	  510176	       465.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Disabled/Nexuer-10               	71119876	         3.333 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Disabled/Slog-10                 	43525894	         5.402 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/1/Nexuer-10               	 1000000	       217.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/1/Slog-10                 	  467593	       513.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/2/Nexuer-10               	  919848	       256.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/2/Slog-10                 	  427330	       563.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/5/Nexuer-10               	  638920	       377.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/5/Slog-10                 	  344463	       692.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/10/Nexuer-10              	  427909	       561.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/10/Slog-10                	  246459	       992.5 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/25/Nexuer-10              	  213049	      1125 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/25/Slog-10                	  136288	      1752 ns/op	     896 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/50/Nexuer-10              	  117489	      2059 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Fields/50/Slog-10                	   80456	      3009 ns/op	    2049 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/IntSlice/Nexuer-10           	  875086	       274.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/IntSlice/Slog-10             	  326426	       752.3 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StringSlice/Nexuer-10        	  760324	       317.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StringSlice/Slog-10          	  302660	       787.5 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Time/Nexuer-10               	  865293	       280.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Time/Slog-10                 	  416901	       570.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/TimeSlice/Nexuer-10          	  236895	      1013 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/TimeSlice/Slog-10            	   86079	      2755 ns/op	     817 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Struct/Nexuer-10             	  464347	       516.0 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Struct/Slog-10               	  262453	       919.1 ns/op	     176 B/op	       3 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StructSlice/Nexuer-10        	   85974	      2759 ns/op	     480 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/StructSlice/Slog-10          	   71653	      3328 ns/op	    1427 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Error/Nexuer-10              	 1000000	       221.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Any/Error/Slog-10                	  430000	       534.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/AccumulatedFields/5/Nexuer-10    	 1366778	       175.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/AccumulatedFields/5/Slog-10      	  513327	       460.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth1/Nexuer-10           	  904394	       269.0 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth1/Slog-10             	  423117	       583.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth3/Nexuer-10           	  732590	       307.4 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Group/Depth3/Slog-10             	  392875	       610.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Context/Nexuer-10                	  991342	       243.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Serial/Context/Slog-10                  	  441284	       554.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Short/Nexuer-10        	 5964140	        38.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Short/Slog-10          	  912358	       243.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Long4K/Nexuer-10       	  294434	       681.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Long4K/Slog-10         	  337928	       870.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Escaped/Nexuer-10      	 6309908	        38.89 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Message/Escaped/Slog-10        	  996505	       239.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Disabled/Nexuer-10             	327492481	         0.7920 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Disabled/Slog-10               	181968499	         1.361 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/1/Nexuer-10             	 3993747	        61.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/1/Slog-10               	  968066	       269.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/2/Nexuer-10             	 4689259	        54.29 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/2/Slog-10               	  949882	       262.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/5/Nexuer-10             	 3158013	        79.71 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/5/Slog-10               	  981193	       261.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/10/Nexuer-10            	 1795226	       137.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/10/Slog-10              	  893868	       362.5 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/25/Nexuer-10            	 1260092	       206.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/25/Slog-10              	  325063	       740.7 ns/op	     897 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/50/Nexuer-10            	  645074	       379.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Fields/50/Slog-10              	  175750	      1347 ns/op	    2052 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/IntSlice/Nexuer-10         	 3077836	        66.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/IntSlice/Slog-10           	  916117	       308.5 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StringSlice/Nexuer-10      	 3361432	        75.65 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StringSlice/Slog-10        	  800721	       315.0 ns/op	     112 B/op	       2 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Time/Nexuer-10             	 4533894	        67.39 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Time/Slog-10               	  972544	       238.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/TimeSlice/Nexuer-10        	 1269909	       212.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/TimeSlice/Slog-10          	  239613	       851.6 ns/op	     818 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Struct/Nexuer-10           	 1700470	       146.7 ns/op	      48 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Struct/Slog-10             	  840874	       350.5 ns/op	     176 B/op	       3 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StructSlice/Nexuer-10      	  455007	       685.8 ns/op	     481 B/op	      10 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/StructSlice/Slog-10        	  211090	      1018 ns/op	    1429 B/op	      12 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Error/Nexuer-10            	 3641370	        60.70 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Any/Error/Slog-10              	  972118	       246.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/AccumulatedFields/5/Nexuer-10  	 5462697	        39.86 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/AccumulatedFields/5/Slog-10    	 1000000	       225.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth1/Nexuer-10         	 2554561	        92.13 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth1/Slog-10           	  848230	       264.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth3/Nexuer-10         	 2477553	       100.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Group/Depth3/Slog-10           	  949334	       245.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Context/Nexuer-10              	 3022737	        82.77 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/JSON/Parallel/Context/Slog-10                	  900492	       271.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Short/Nexuer-10          	  609594	       389.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Short/Slog-10            	  334288	       724.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Long4K/Nexuer-10         	   12945	     18606 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Long4K/Slog-10           	   12696	     18910 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Escaped/Nexuer-10        	  728809	       323.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Message/Escaped/Slog-10          	  366357	       655.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Disabled/Nexuer-10               	72465583	         3.316 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Disabled/Slog-10                 	44973296	         5.368 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/1/Nexuer-10               	  544917	       441.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/1/Slog-10                 	  310304	       767.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/2/Nexuer-10               	  498942	       480.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/2/Slog-10                 	  295869	       816.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/5/Nexuer-10               	  402510	       600.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/5/Slog-10                 	  248473	       957.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/10/Nexuer-10              	  304981	       787.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/10/Slog-10                	  192440	      1266 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/25/Nexuer-10              	  175994	      1378 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/25/Slog-10                	  116967	      2051 ns/op	     896 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/50/Nexuer-10              	  102596	      2345 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Fields/50/Slog-10                	   69558	      3402 ns/op	    2049 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/IntSlice/Nexuer-10           	  220633	      1091 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/IntSlice/Slog-10             	  165844	      1446 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StringSlice/Nexuer-10        	  227271	      1073 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StringSlice/Slog-10          	  168885	      1438 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Time/Nexuer-10               	  455389	       521.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Time/Slog-10                 	  287577	       839.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/TimeSlice/Nexuer-10          	   56200	      4256 ns/op	     881 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/TimeSlice/Slog-10            	   51630	      4616 ns/op	     881 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Struct/Nexuer-10             	  177790	      1376 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Struct/Slog-10               	  139105	      1734 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StructSlice/Nexuer-10        	  160299	      1505 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/StructSlice/Slog-10          	  127728	      1864 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Error/Nexuer-10              	  539415	       445.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Any/Error/Slog-10                	  276157	       860.5 ns/op	       4 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/AccumulatedFields/5/Nexuer-10    	  608486	       396.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/AccumulatedFields/5/Slog-10      	  331750	       719.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth1/Nexuer-10           	  477097	       506.0 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth1/Slog-10             	  291904	       824.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth3/Nexuer-10           	  442926	       539.4 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Group/Depth3/Slog-10             	  275655	       856.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Context/Nexuer-10                	  515154	       465.8 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Serial/Context/Slog-10                  	  301150	       798.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Short/Nexuer-10        	 2756541	        92.60 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Short/Slog-10          	  940711	       236.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Long4K/Nexuer-10       	   97969	      2520 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Long4K/Slog-10         	   87202	      2720 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Escaped/Nexuer-10      	 3662460	        57.72 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Message/Escaped/Slog-10        	 1000000	       227.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Disabled/Nexuer-10             	508269770	         0.4977 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Disabled/Slog-10               	179252684	         1.234 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/1/Nexuer-10             	 3476817	        72.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/1/Slog-10               	  964564	       244.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/2/Nexuer-10             	 2736843	        93.25 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/2/Slog-10               	  831156	       296.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/5/Nexuer-10             	 2403468	       109.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/5/Slog-10               	  902919	       245.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/10/Nexuer-10            	 1702916	       162.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/10/Slog-10              	  760848	       374.6 ns/op	     208 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/25/Nexuer-10            	  824824	       265.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/25/Slog-10              	  319884	       733.0 ns/op	     897 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/50/Nexuer-10            	  544329	       412.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Fields/50/Slog-10              	  182370	      1321 ns/op	    2052 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/IntSlice/Nexuer-10         	 1000000	       284.6 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/IntSlice/Slog-10           	  788997	       391.2 ns/op	     104 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StringSlice/Nexuer-10      	 1000000	       330.6 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StringSlice/Slog-10        	  737430	       395.3 ns/op	     184 B/op	      11 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Time/Nexuer-10             	 2455790	       102.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Time/Slog-10               	  897252	       279.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/TimeSlice/Nexuer-10        	  251881	       994.9 ns/op	     882 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/TimeSlice/Slog-10          	  221383	      1107 ns/op	     882 B/op	      21 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Struct/Nexuer-10           	  783193	       319.4 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Struct/Slog-10             	  590502	       410.0 ns/op	     232 B/op	       6 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StructSlice/Nexuer-10      	 1000000	       351.4 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/StructSlice/Slog-10        	  631807	       439.2 ns/op	     128 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Error/Nexuer-10            	 3271510	        71.67 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Any/Error/Slog-10              	  808828	       263.0 ns/op	       4 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/AccumulatedFields/5/Nexuer-10  	 3695448	        73.85 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/AccumulatedFields/5/Slog-10    	  921084	       252.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth1/Nexuer-10         	 2075962	       114.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth1/Slog-10           	  852272	       272.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth3/Nexuer-10         	 2025600	       127.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Group/Depth3/Slog-10           	  852954	       274.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Context/Nexuer-10              	 1939753	       115.1 ns/op	      32 B/op	       1 allocs/op
BenchmarkNexuerVsSlogEncodeOnly/Text/Parallel/Context/Slog-10                	 1000000	       258.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Nexuer-10                      	65816535	         3.673 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Zap-10                         	42418444	         5.656 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Zerolog-10                     	73148430	         3.327 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Phuslu-10                      	79133931	         2.990 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Slog-10                        	42499185	         5.662 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Serial/Logrus-10                      	13258417	        17.82 ns/op	      16 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Nexuer-10                    	470727072	         0.5602 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Zap-10                       	263172204	         0.8512 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Zerolog-10                   	456267622	         0.5103 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Phuslu-10                    	535952545	         0.4568 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Slog-10                      	268987303	         0.8649 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Disabled/Parallel/Logrus-10                    	30682034	         8.319 ns/op	      16 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Nexuer-10                       	 1433332	       167.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Zap-10                          	  904820	       268.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Zerolog-10                      	 1386126	       172.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Phuslu-10                       	 1772241	       135.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Slog-10                         	  524967	       465.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Serial/Logrus-10                       	  201826	      1160 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Nexuer-10                     	 6911703	        35.98 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Zap-10                        	 4610552	        49.74 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Zerolog-10                    	 8341165	        31.34 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Phuslu-10                     	11800738	        20.06 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Slog-10                       	 1000000	       241.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/Message/Parallel/Logrus-10                     	  167427	      1532 ns/op	     939 B/op	      20 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Nexuer-10             	 1259547	       189.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Zap-10                	  826575	       289.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Zerolog-10            	 1244827	       192.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Phuslu-10             	 1523342	       157.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Slog-10               	  498054	       484.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Serial/Logrus-10             	   25159	      9530 ns/op	    3746 B/op	      69 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Nexuer-10           	 5510679	        46.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Zap-10              	 6118078	        46.66 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Zerolog-10          	 8207887	        33.99 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Phuslu-10           	 9879151	        24.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Slog-10             	 1000000	       220.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/AccumulatedFields/Parallel/Logrus-10           	   22040	     10877 ns/op	    3766 B/op	      69 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Nexuer-10                	   49350	      4895 ns/op	     577 B/op	      12 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Zap-10                   	  118738	      2031 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Zerolog-10               	  113359	      2099 ns/op	     184 B/op	      11 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Phuslu-10                	  135090	      1710 ns/op	      24 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Slog-10                  	   31155	      7683 ns/op	    3032 B/op	      35 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Serial/Logrus-10                	   23802	     10022 ns/op	    4575 B/op	      75 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Nexuer-10              	  228351	      1097 ns/op	     578 B/op	      12 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Zap-10                 	  820419	       284.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Zerolog-10             	  619574	       507.6 ns/op	     185 B/op	      11 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Phuslu-10              	  928342	       294.5 ns/op	      24 B/op	       1 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Slog-10                	  103342	      2453 ns/op	    3038 B/op	      35 allocs/op
BenchmarkComparisonEncodeOnly/CallsiteFields/Parallel/Logrus-10              	   20653	     11714 ns/op	    4600 B/op	      75 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Nexuer-10                        	 1367812	       175.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Zap-10                           	  748771	       317.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Zerolog-10                       	 1396166	       171.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Phuslu-10                        	 1775095	       135.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Slog-10                          	  520974	       457.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Serial/Logrus-10                        	  206575	      1155 ns/op	     938 B/op	      20 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Nexuer-10                      	 1228191	       194.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Zap-10                         	 4638487	        48.29 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Zerolog-10                     	 7344820	        30.22 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Phuslu-10                      	10205364	        21.78 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Slog-10                        	  956334	       237.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/Message/Parallel/Logrus-10                      	  156594	      1517 ns/op	     939 B/op	      20 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Nexuer-10                    	  675081	       361.3 ns/op	      96 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Zap-10                       	  541231	       452.7 ns/op	     192 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Zerolog-10                   	 1000000	       222.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Phuslu-10                    	 1476524	       162.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Slog-10                      	  375998	       632.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Serial/Logrus-10                    	  116914	      2030 ns/op	    1884 B/op	      30 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Nexuer-10                  	  955620	       252.8 ns/op	      96 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Zap-10                     	 1498810	       159.4 ns/op	     192 B/op	       1 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Zerolog-10                 	 5716916	        43.68 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Phuslu-10                  	 8753998	        26.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Slog-10                    	  985609	       263.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkComparisonWritePath/ShortFields/Parallel/Logrus-10                  	   96964	      2515 ns/op	    1887 B/op	      30 allocs/op
PASS
ok  	github.com/nexuer/log/benchmarks	154.280s
```
