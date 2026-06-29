# Benchmarks

This package compares `github.com/nexuer/log` with several Go logging libraries.

## Run

```sh
go test -run '^$' -bench=. -benchmem
```

Run a smaller core set:

```sh
go test -run '^$' -bench='BenchmarkDisabledWithoutFields|BenchmarkWithoutFields|BenchmarkAccumulatedContext|BenchmarkAddingFields|BenchmarkNexuerTextHandler' -benchmem
```

## Scenarios

- `BenchmarkDisabledWithoutFields`: log call is below the enabled level and has no accumulated context.
- `BenchmarkDisabledAccumulatedContext`: log call is disabled after logger-level context has been attached.
- `BenchmarkDisabledAddingFields`: log call is disabled while adding fields at the call site.
- `BenchmarkWithoutFields`: enabled log call with only a message.
- `BenchmarkAccumulatedContext`: enabled log call after logger-level context has been attached.
- `BenchmarkAddingFields`: enabled log call while adding fields at the call site.
- `BenchmarkNexuerTextHandler`: text handler scenarios for NexuerLog. Cross-library benchmarks use the JSON handler for NexuerLog.

## NexuerLog APIs

- `NexuerLog.Info`: print-style message API. It accepts variadic arguments and may allocate even when the level is disabled.
- `NexuerLog.Infof`: format-style message API.
- `NexuerLog.InfoS`: structured message API. This is the preferred hot-path API.
- `NexuerLog.Formatting`: format-style API with many formatting arguments.
- `hasValuer`: includes deferred fields such as timestamp and caller.

## Notes

Most cross-library scenarios write to `io.Discard` and measure encoding plus logger overhead, not disk or terminal I/O. Results should be compared within the same run and machine.

## Results

```text
goos: darwin
goarch: arm64
pkg: github.com/nexuer/log/benchmarks
cpu: Apple M1 Pro
BenchmarkDisabledWithoutFields/NexuerLog.Info-10         	177800803	         7.472 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.Infof-10        	1000000000	         0.4033 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.InfoS-10        	1000000000	         0.3763 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/NexuerLog.Formatting-10   	20795004	        58.42 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/Zap-10                    	1000000000	         0.5888 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/Zap.Check-10              	1000000000	         0.4706 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/Zap.Sugar-10              	143740726	         7.121 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/Zap.SugarFormatting-10    	21391168	        58.94 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/apex/log-10               	1000000000	         0.4958 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/sirupsen/logrus-10        	177142830	         7.537 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledWithoutFields/rs/zerolog-10             	1000000000	         0.4125 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/rs/zerolog.Formatting-10  	14323611	       100.2 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledWithoutFields/slog-10                   	1000000000	         0.8715 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledWithoutFields/slog.LogAttrs-10          	1000000000	         0.6847 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Info-10    	150470832	         7.902 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Info.hasValuer-10         	160227576	         8.157 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Infof-10                  	1000000000	         0.3785 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.InfoS-10                  	1000000000	         0.3910 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/NexuerLog.Formatting-10             	18725763	        61.74 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/Zap-10                              	1000000000	         0.6381 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.Check-10                        	1000000000	         0.4996 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.Sugar-10                        	141224146	         8.532 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/Zap.SugarFormatting-10              	19602904	        67.02 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/apex/log-10                         	1000000000	         0.3274 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/sirupsen/logrus-10                  	149331693	         7.248 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisabledAccumulatedContext/rs/zerolog-10                       	1000000000	         0.3456 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/rs/zerolog.Formatting-10            	18971844	        62.30 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAccumulatedContext/slog-10                             	1000000000	         0.6795 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAccumulatedContext/slog.LogAttrs-10                    	1000000000	         0.6940 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAddingFields/NexuerLog-10                              	 7493265	       159.5 ns/op	     456 B/op	       7 allocs/op
BenchmarkDisabledAddingFields/NexuerLog.hasValuer-10                    	 6209768	       190.0 ns/op	     520 B/op	       7 allocs/op
BenchmarkDisabledAddingFields/Zap-10                                    	 4783671	       256.5 ns/op	     800 B/op	       5 allocs/op
BenchmarkDisabledAddingFields/Zap.Check-10                              	1000000000	         0.5102 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisabledAddingFields/Zap.Sugar-10                              	19496436	        64.29 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAddingFields/apex/log-10                               	 3363190	       347.4 ns/op	     920 B/op	      12 allocs/op
BenchmarkDisabledAddingFields/sirupsen/logrus-10                        	 1731272	       661.0 ns/op	    1593 B/op	      16 allocs/op
BenchmarkDisabledAddingFields/rs/zerolog-10                             	100000000	        12.33 ns/op	      24 B/op	       1 allocs/op
BenchmarkDisabledAddingFields/slog-10                                   	19141706	        62.70 ns/op	     136 B/op	       6 allocs/op
BenchmarkDisabledAddingFields/slog.LogAttrs-10                          	 6903660	       176.6 ns/op	     512 B/op	       5 allocs/op
BenchmarkWithoutFields/NexuerLog.Info-10                                	25795380	        55.07 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/NexuerLog.Infof-10                               	36712029	        32.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/NexuerLog.InfoS-10                               	31848224	        40.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/NexuerLog.Formatting-10                          	  673819	      1791 ns/op	    1920 B/op	      58 allocs/op
BenchmarkWithoutFields/Zap-10                                           	22879513	        49.35 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.Check-10                                     	28837719	        42.26 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.CheckSampled-10                              	32358278	        35.48 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/Zap.Sugar-10                                     	17935231	        74.83 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/Zap.SugarFormatting-10                           	  679406	      1748 ns/op	    1922 B/op	      58 allocs/op
BenchmarkWithoutFields/apex/log-10                                      	 1454930	       829.4 ns/op	     216 B/op	       4 allocs/op
BenchmarkWithoutFields/go-kit/kit/log-10                                	 4643096	       241.6 ns/op	     480 B/op	       8 allocs/op
BenchmarkWithoutFields/inconshreveable/log15-10                         	  511062	      2167 ns/op	    1265 B/op	      17 allocs/op
BenchmarkWithoutFields/sirupsen/logrus-10                               	  809158	      1510 ns/op	     938 B/op	      20 allocs/op
BenchmarkWithoutFields/stdlib.Println-10                                	 5873685	       197.4 ns/op	      16 B/op	       1 allocs/op
BenchmarkWithoutFields/stdlib.Printf-10                                 	  915756	      1667 ns/op	    1274 B/op	      57 allocs/op
BenchmarkWithoutFields/rs/zerolog-10                                    	37218342	        31.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/rs/zerolog.Formatting-10                         	  657616	      1843 ns/op	    1915 B/op	      58 allocs/op
BenchmarkWithoutFields/rs/zerolog.Check-10                              	45591694	        32.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/slog-10                                          	 5586784	       214.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkWithoutFields/slog.LogAttrs-10                                 	 5183334	       228.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Info-10                           	18873228	        69.47 ns/op	      16 B/op	       1 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Info.hasValuer-10                 	 3586242	       386.3 ns/op	     344 B/op	       5 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Infof-10                          	31214538	        40.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Infof.hasValuer-10                	 3420676	       338.8 ns/op	     328 B/op	       4 allocs/op
BenchmarkAccumulatedContext/NexuerLog.InfoS-10                          	27595959	        45.86 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/NexuerLog.Formatting-10                     	  645694	      1896 ns/op	    1932 B/op	      58 allocs/op
BenchmarkAccumulatedContext/Zap-10                                      	21352170	        54.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.Check-10                                	32049748	        59.57 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.CheckSampled-10                         	31975875	        43.13 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/Zap.Sugar-10                                	 9737703	       119.9 ns/op	      16 B/op	       1 allocs/op
BenchmarkAccumulatedContext/Zap.SugarFormatting-10                      	  603943	      2045 ns/op	    1926 B/op	      58 allocs/op
BenchmarkAccumulatedContext/apex/log-10                                 	  114045	     10120 ns/op	    2974 B/op	      52 allocs/op
BenchmarkAccumulatedContext/go-kit/kit/log-10                           	  442197	      2831 ns/op	    3368 B/op	      55 allocs/op
BenchmarkAccumulatedContext/inconshreveable/log15-10                    	  130410	      9053 ns/op	    3094 B/op	      67 allocs/op
BenchmarkAccumulatedContext/sirupsen/logrus-10                          	  105645	     11095 ns/op	    3762 B/op	      69 allocs/op
BenchmarkAccumulatedContext/rs/zerolog-10                               	35649961	        33.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/rs/zerolog.Check-10                         	32658009	        33.36 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/rs/zerolog.Formatting-10                    	  693806	      1868 ns/op	    1915 B/op	      58 allocs/op
BenchmarkAccumulatedContext/slog-10                                     	 5912776	       215.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkAccumulatedContext/slog.LogAttrs-10                            	 4871962	       231.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkAddingFields/NexuerLog-10                                      	  460513	      2650 ns/op	    3286 B/op	      41 allocs/op
BenchmarkAddingFields/NexuerLog.hasValuer-10                            	  342529	      3535 ns/op	    3749 B/op	      46 allocs/op
BenchmarkAddingFields/Zap-10                                            	 1573056	       766.2 ns/op	     804 B/op	       5 allocs/op
BenchmarkAddingFields/Zap.Check-10                                      	 1592985	       677.9 ns/op	     805 B/op	       5 allocs/op
BenchmarkAddingFields/Zap.CheckSampled-10                               	10810039	       112.2 ns/op	      88 B/op	       0 allocs/op
BenchmarkAddingFields/Zap.Sugar-10                                      	 1239872	      1022 ns/op	    1627 B/op	      10 allocs/op
BenchmarkAddingFields/apex/log-10                                       	  110815	     10853 ns/op	    3900 B/op	      64 allocs/op
BenchmarkAddingFields/go-kit/kit/log-10                                 	  487099	      2424 ns/op	    2990 B/op	      56 allocs/op
BenchmarkAddingFields/inconshreveable/log15-10                          	  102744	     11991 ns/op	    6233 B/op	      73 allocs/op
BenchmarkAddingFields/sirupsen/logrus-10                                	   97532	     12239 ns/op	    5360 B/op	      84 allocs/op
BenchmarkAddingFields/rs/zerolog-10                                     	 4705833	       327.5 ns/op	      24 B/op	       1 allocs/op
BenchmarkAddingFields/rs/zerolog.Check-10                               	 3859047	       396.9 ns/op	      24 B/op	       1 allocs/op
BenchmarkAddingFields/slog-10                                           	  410817	      3148 ns/op	    3175 B/op	      41 allocs/op
BenchmarkAddingFields/slog.LogAttrs-10                                  	  412651	      2859 ns/op	    3554 B/op	      40 allocs/op
BenchmarkNexuerTextHandler/DisabledWithoutFields/InfoS-10               	1000000000	         0.5744 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerTextHandler/WithoutFields/InfoS-10                       	14125893	        89.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerTextHandler/AccumulatedContext/InfoS-10                  	14441910	       128.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNexuerTextHandler/AddingFields/InfoS-10                        	  355348	      4936 ns/op	    2220 B/op	      63 allocs/op
PASS
ok  	github.com/nexuer/log/benchmarks	128.597s
```
