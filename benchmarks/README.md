# Benchmarks

This package compares `github.com/nexuer/log` with several Go logging libraries.

## Run

```sh
go test -run '^$' -bench=. -benchmem
```

Run a smaller core set:

```sh
go test -run '^$' -bench='BenchmarkDisabledWithoutFields|BenchmarkWithoutFields|BenchmarkAccumulatedContext|BenchmarkAddingFields|BenchmarkNexuerHandlerFormats' -benchmem
```

## Scenarios

- `BenchmarkDisabledWithoutFields`: log call is below the enabled level and has no accumulated context.
- `BenchmarkDisabledAccumulatedContext`: log call is disabled after logger-level context has been attached.
- `BenchmarkDisabledAddingFields`: log call is disabled while adding fields at the call site.
- `BenchmarkWithoutFields`: enabled log call with only a message.
- `BenchmarkAccumulatedContext`: enabled log call after logger-level context has been attached.
- `BenchmarkAddingFields`: enabled log call while adding fields at the call site.
- `BenchmarkNexuerHandlerFormats`: matched NexuerLog JSON and text handler scenarios, including flat fields and grouped fields. Cross-library benchmarks use the JSON handler for NexuerLog.

## NexuerLog APIs

- `NexuerLog.Info`: print-style message API. It accepts variadic arguments and may allocate even when the level is disabled.
- `NexuerLog.Infof`: format-style message API.
- `NexuerLog.InfoS`: structured message API. This is the preferred hot-path API.
- `NexuerLog.Formatting`: format-style API with many formatting arguments.
- `hasValuer`: includes deferred fields such as timestamp and caller.

## Notes

Most cross-library scenarios write to `io.Discard` and measure encoding plus logger overhead, not disk or terminal I/O. Results should be compared within the same run and machine.

## WithGroup Notes

`BenchmarkNexuerHandlerFormats` includes matching `FlatFields` and `WithGroup` cases. Both use one pre-attached field and two call-site fields; `WithGroup` nests them under `request` while `FlatFields` keeps them flat.

In this run:

- JSON `WithGroup` is effectively flat-field cost: `97.72 ns/op` vs `97.32 ns/op`, with the same `64 B/op` and `1 allocs/op`.
- Text `WithGroup` adds about `19.6 ns/op` over flat fields: `140.7 ns/op` vs `121.1 ns/op`, with the same `64 B/op` and `1 allocs/op`.
- The extra Text cost is from composing prefixed keys such as `request.method`; JSON mostly pays the same cost as writing the nested object structure.

## Results

```text
goos: darwin
goarch: arm64
pkg: github.com/nexuer/log/benchmarks
cpu: Apple M1 Pro
BenchmarkNexuerHandlerFormats/JSON/DisabledWithoutFields-10         	1000000000	         0.3852 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/JSON/WithoutFields-10                 	31248643	        41.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/JSON/AccumulatedContext-10            	32554621	        37.90 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/JSON/AccumulatedContextWithValuer-10  	 4111640	       323.3 ns/op	     328 B/op	       4 allocs/op
BenchmarkNexuerHandlerFormats/JSON/AddingFields-10                  	  502958	      2305 ns/op	    3288 B/op	      41 allocs/op
BenchmarkNexuerHandlerFormats/JSON/FlatFields-10                    	13667905	        97.32 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerHandlerFormats/JSON/WithGroup-10                     	11461308	        97.72 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerHandlerFormats/Text/DisabledWithoutFields-10         	1000000000	         0.3900 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/Text/WithoutFields-10                 	17869096	        67.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/Text/AccumulatedContext-10            	15429427	        71.75 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerHandlerFormats/Text/AccumulatedContextWithValuer-10  	 3497624	       323.9 ns/op	     328 B/op	       4 allocs/op
BenchmarkNexuerHandlerFormats/Text/AddingFields-10                  	  520869	      2542 ns/op	    2223 B/op	      63 allocs/op
BenchmarkNexuerHandlerFormats/Text/FlatFields-10                    	 9459324	       121.1 ns/op	      64 B/op	       1 allocs/op
BenchmarkNexuerHandlerFormats/Text/WithGroup-10                     	 9081646	       140.7 ns/op	      64 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.Info-10                    	156234835	         8.641 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.Infof-10                   	1000000000	         0.4040 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.InfoS-10                   	1000000000	         0.4098 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.Formatting-10              	18175540	        61.11 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/Zap-10                               	1000000000	         0.6454 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/Zap.Check-10                         	1000000000	         0.5315 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/Zap.Sugar-10                         	157203297	         8.156 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/Zap.SugarFormatting-10               	19269033	        67.40 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/apex/log-10                          	1000000000	         0.5463 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/sirupsen/logrus-10                   	160444958	         8.481 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/rs/zerolog-10                        	1000000000	         0.3734 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/rs/zerolog.Formatting-10             	20008600	        61.43 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/slog-10                              	1000000000	         0.6810 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/slog.LogAttrs-10                     	1000000000	         0.7156 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Info-10               	157875140	         7.187 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Info.hasValuer-10     	156739504	         8.242 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Infof-10              	1000000000	         0.4179 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.InfoS-10              	1000000000	         0.4281 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Formatting-10         	20119093	        61.12 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/Zap-10                          	1000000000	         0.6661 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.Check-10                    	1000000000	         0.5532 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.Sugar-10                    	152728615	         8.164 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.SugarFormatting-10          	18146029	        65.09 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/apex/log-10                     	1000000000	         0.2915 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/sirupsen/logrus-10              	158625849	         7.209 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/rs/zerolog-10                   	1000000000	         0.3692 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/rs/zerolog.Formatting-10        	18846379	        60.37 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/slog-10                         	1000000000	         0.6974 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/slog.LogAttrs-10                	1000000000	         0.7213 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAddingFields/NexuerLog-10                          	 7245691	       159.2 ns/op	     456 B/op	       7 allocs/op
BenchmarkDisabledAddingFields/NexuerLog.hasValuer-10                	 6142708	       193.3 ns/op	     520 B/op	       7 allocs/op
BenchmarkDisabledAddingFields/Zap-10                                	 4775793	       253.1 ns/op	     800 B/op	       5 allocs/op
BenchmarkDisabledAddingFields/Zap.Check-10                          	1000000000	         0.4995 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAddingFields/Zap.Sugar-10                          	20090476	        60.94 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAddingFields/apex/log-10                           	 3624392	       338.2 ns/op	     920 B/op	      12 allocs/op
BenchmarkDisabledAddingFields/sirupsen/logrus-10                    	 1858335	       671.6 ns/op	    1593 B/op	      16 allocs/op
BenchmarkDisabledAddingFields/rs/zerolog-10                         	93807405	        12.47 ns/op	      24 B/op	       1 allocs/op
BenchmarkDisabledAddingFields/slog-10                               	19768909	        62.45 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAddingFields/slog.LogAttrs-10                      	 6917385	       170.7 ns/op	     512 B/op	       5 allocs/op
BenchmarkWithoutFields/NexuerLog.Info-10                            	23377818	        49.12 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/NexuerLog.Infof-10                           	26908971	        41.42 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/NexuerLog.InfoS-10                           	31720542	        33.81 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/NexuerLog.Formatting-10                      	  588837	      1729 ns/op	    1920 B/op	      58 allocs/op
BenchmarkWithoutFields/Zap-10                                       	28693460	        40.18 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.Check-10                                 	25348297	        40.67 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.CheckSampled-10                          	31941616	        35.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.Sugar-10                                 	16152868	        69.97 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/Zap.SugarFormatting-10                       	  743902	      1639 ns/op	    1922 B/op	      58 allocs/op
BenchmarkWithoutFields/apex/log-10                                  	 1438642	       838.4 ns/op	     216 B/op	       4 allocs/op
BenchmarkWithoutFields/go-kit/kit/log-10                            	 4750106	       228.0 ns/op	     480 B/op	       8 allocs/op
BenchmarkWithoutFields/inconshreveable/log15-10                     	  580881	      2043 ns/op	    1265 B/op	      17 allocs/op
BenchmarkWithoutFields/sirupsen/logrus-10                           	  735432	      1526 ns/op	     938 B/op	      20 allocs/op
BenchmarkWithoutFields/stdlib.Println-10                            	 5929909	       206.5 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/stdlib.Printf-10                             	 1000000	      1331 ns/op	    1275 B/op	      57 allocs/op
BenchmarkWithoutFields/rs/zerolog-10                                	44341050	        28.14 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/rs/zerolog.Formatting-10                     	  743160	      1602 ns/op	    1916 B/op	      58 allocs/op
BenchmarkWithoutFields/rs/zerolog.Check-10                          	45453326	        29.34 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/slog-10                                      	 5420626	       226.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/slog.LogAttrs-10                             	 4998319	       228.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Info-10                       	19794970	        55.93 ns/op	      16 B/op	       1 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Info.hasValuer-10             	 3609471	       332.1 ns/op	     344 B/op	       5 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Infof-10                      	28212142	        37.17 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Infof.hasValuer-10            	 3786306	       308.5 ns/op	     328 B/op	       4 allocs/op
BenchmarkAccumulatedContext/NexuerLog.InfoS-10                      	26437249	        47.63 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Formatting-10                 	  759442	      1698 ns/op	    1928 B/op	      58 allocs/op
BenchmarkAccumulatedContext/Zap-10                                  	26793314	        49.65 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.Check-10                            	27079172	        51.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.CheckSampled-10                     	34583626	        35.26 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.Sugar-10                            	15328328	        79.30 ns/op	      16 B/op	       1 allocs/op
BenchmarkAccumulatedContext/Zap.SugarFormatting-10                  	  643300	      1651 ns/op	    1927 B/op	      58 allocs/op
BenchmarkAccumulatedContext/apex/log-10                             	  123187	      9549 ns/op	    2973 B/op	      52 allocs/op
BenchmarkAccumulatedContext/go-kit/kit/log-10                       	  489981	      2394 ns/op	    3370 B/op	      55 allocs/op
BenchmarkAccumulatedContext/inconshreveable/log15-10                	  141991	      8467 ns/op	    3093 B/op	      67 allocs/op
BenchmarkAccumulatedContext/sirupsen/logrus-10                      	  113023	     10724 ns/op	    3762 B/op	      69 allocs/op
BenchmarkAccumulatedContext/rs/zerolog-10                           	42492415	        27.46 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/rs/zerolog.Check-10                     	38595499	        26.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/rs/zerolog.Formatting-10                	  801734	      1632 ns/op	    1916 B/op	      58 allocs/op
BenchmarkAccumulatedContext/slog-10                                 	 5425828	       208.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/slog.LogAttrs-10                        	 5100032	       225.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkAddingFields/NexuerLog-10                                  	  541110	      2258 ns/op	    3290 B/op	      41 allocs/op
BenchmarkAddingFields/NexuerLog.hasValuer-10                        	  412890	      2775 ns/op	    3752 B/op	      46 allocs/op
BenchmarkAddingFields/Zap-10                                        	 1873030	       632.7 ns/op	     805 B/op	       5 allocs/op
BenchmarkAddingFields/Zap.Check-10                                  	 1832221	       640.8 ns/op	     805 B/op	       5 allocs/op
BenchmarkAddingFields/Zap.CheckSampled-10                           	10942194	       104.6 ns/op	      87 B/op	       0 allocs/op
BenchmarkAddingFields/Zap.Sugar-10                                  	 1293979	       921.6 ns/op	    1628 B/op	      10 allocs/op
BenchmarkAddingFields/apex/log-10                                   	  115848	     10236 ns/op	    3897 B/op	      64 allocs/op
BenchmarkAddingFields/go-kit/kit/log-10                             	  573664	      2270 ns/op	    2990 B/op	      56 allocs/op
BenchmarkAddingFields/inconshreveable/log15-10                      	   99925	     11965 ns/op	    6232 B/op	      73 allocs/op
BenchmarkAddingFields/sirupsen/logrus-10                            	   98665	     12342 ns/op	    5357 B/op	      84 allocs/op
BenchmarkAddingFields/rs/zerolog-10                                 	 4097528	       288.2 ns/op	      24 B/op	       1 allocs/op
BenchmarkAddingFields/rs/zerolog.Check-10                           	 4327336	       282.2 ns/op	      24 B/op	       1 allocs/op
BenchmarkAddingFields/slog-10                                       	  487516	      2399 ns/op	    3178 B/op	      41 allocs/op
BenchmarkAddingFields/slog.LogAttrs-10                              	  474123	      2573 ns/op	    3557 B/op	      40 allocs/op
PASS
ok  	github.com/nexuer/log/benchmarks	134.083s
```
