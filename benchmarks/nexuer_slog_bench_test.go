package benchmarks

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	nlog "github.com/nexuer/log"
)

type nexuerSlogFormat struct {
	name          string
	nexuerHandler func() nlog.Handler
	slogHandler   func(io.Writer, *slog.HandlerOptions) slog.Handler
}

type nexuerSlogPair struct {
	nexuer func()
	slog   func()
}

type nexuerSlogCase struct {
	name string
	new  func(nexuerSlogFormat) nexuerSlogPair
}

type metadataStrippingHandler struct{ slog.Handler }

func (h metadataStrippingHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Time = time.Time{}
	record.PC = 0
	return h.Handler.Handle(ctx, record)
}

func (h metadataStrippingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return metadataStrippingHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h metadataStrippingHandler) WithGroup(name string) slog.Handler {
	return metadataStrippingHandler{Handler: h.Handler.WithGroup(name)}
}

type slogHandlerBenchmarkCase struct {
	name string
	new  func(newHandler func(io.Writer) slog.Handler) func()
}

// BenchmarkSlogHandlers compares handlers behind the same slog.Logger API.
// Both handlers omit Record time and PC and write through the same no-op writer.
func BenchmarkSlogHandlers(b *testing.B) {
	for _, format := range []struct {
		name     string
		standard func(io.Writer) slog.Handler
		nexuer   func(io.Writer) slog.Handler
	}{
		{
			name: "JSON",
			standard: func(w io.Writer) slog.Handler {
				return metadataStrippingHandler{Handler: slog.NewJSONHandler(w, nil)}
			},
			nexuer: func(w io.Writer) slog.Handler { return nlog.NewSlogHandler(nlog.New(w, nlog.Json())) },
		},
		{
			name: "Text",
			standard: func(w io.Writer) slog.Handler {
				return metadataStrippingHandler{Handler: slog.NewTextHandler(w, nil)}
			},
			nexuer: func(w io.Writer) slog.Handler { return nlog.NewSlogHandler(nlog.New(w, nlog.Text())) },
		},
	} {
		b.Run(format.name, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					for _, bc := range slogHandlerBenchmarkCases() {
						b.Run(bc.name, func(b *testing.B) {
							for _, impl := range []struct {
								name string
								new  func(io.Writer) slog.Handler
							}{
								{"Standard", format.standard},
								{"Nexuer", format.nexuer},
							} {
								b.Run(impl.name, func(b *testing.B) {
									runBenchmark(b, mode, bc.new(impl.new))
								})
							}
						})
					}
				})
			}
		})
	}
}

// BenchmarkNexuerSlogDefaultFields measures the timestamp/caller path after a
// configured Logger is exposed as a slog.Handler, including depth composition.
func BenchmarkNexuerSlogDefaultFields(b *testing.B) {
	for _, format := range []struct {
		name    string
		handler func() nlog.Handler
	}{
		{"JSON", func() nlog.Handler { return nlog.Json() }},
		{"Text", func() nlog.Handler { return nlog.Text() }},
	} {
		b.Run(format.name, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					for _, bc := range []struct {
						name string
						new  func() func()
					}{
						{"DefaultFields", func() func() {
							native := nlog.New(benchmarkDiscardWriter{}, format.handler()).WithFields(nlog.DefaultFields...)
							logger := slog.New(nlog.NewSlogHandler(native))
							return func() { logger.Info(getMessage(0)) }
						}},
						{"LoggerDepth1", func() func() {
							ctx := nlog.AddCallerDepth(context.Background(), 1)
							native := nlog.New(benchmarkDiscardWriter{}, format.handler()).
								WithContext(ctx).
								WithFields(nlog.DefaultFields...)
							logger := slog.New(nlog.NewSlogHandler(native))
							return func() { logger.Info(getMessage(0)) }
						}},
						{"CallDepth1", func() func() {
							ctx := nlog.AddCallerDepth(context.Background(), 1)
							native := nlog.New(benchmarkDiscardWriter{}, format.handler()).WithFields(nlog.DefaultFields...)
							logger := slog.New(nlog.NewSlogHandler(native))
							return func() { logger.LogAttrs(ctx, slog.LevelInfo, getMessage(0)) }
						}},
						{"MergedDepth2", func() func() {
							loggerCtx := nlog.AddCallerDepth(context.Background(), 1)
							callCtx := nlog.AddCallerDepth(context.Background(), 1)
							native := nlog.New(benchmarkDiscardWriter{}, format.handler()).
								WithContext(loggerCtx).
								WithFields(nlog.DefaultFields...)
							logger := slog.New(nlog.NewSlogHandler(native))
							return func() { logger.LogAttrs(callCtx, slog.LevelInfo, getMessage(0)) }
						}},
					} {
						b.Run(bc.name, func(b *testing.B) {
							runBenchmark(b, mode, bc.new())
						})
					}
				})
			}
		})
	}
}

func slogHandlerBenchmarkCases() []slogHandlerBenchmarkCase {
	message := getMessage(0)
	cases := []slogHandlerBenchmarkCase{
		{
			name: "Message",
			new: func(newHandler func(io.Writer) slog.Handler) func() {
				logger := slog.New(newHandler(benchmarkDiscardWriter{}))
				return func() { logger.Info(message) }
			},
		},
	}
	for _, count := range []int{1, 5, 10, 25, 50} {
		attrs := makeSlogIntAttrs(count)
		cases = append(cases, slogHandlerBenchmarkCase{
			name: "Fields/" + strconv.Itoa(count),
			new: func(newHandler func(io.Writer) slog.Handler) func() {
				logger := slog.New(newHandler(benchmarkDiscardWriter{}))
				return func() { logger.LogAttrs(context.Background(), slog.LevelInfo, message, attrs...) }
			},
		})
	}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"Any/IntSlice", _tenInts},
		{"Any/TimeSlice", _tenTimes},
		{"Any/StructSlice", _tenUsers},
	} {
		attr := slog.Any("value", tc.value)
		cases = append(cases, slogHandlerBenchmarkCase{
			name: tc.name,
			new: func(newHandler func(io.Writer) slog.Handler) func() {
				logger := slog.New(newHandler(benchmarkDiscardWriter{}))
				return func() { logger.LogAttrs(context.Background(), slog.LevelInfo, message, attr) }
			},
		})
	}
	withAttrs := makeSlogIntAttrs(5)
	withArgs := make([]any, len(withAttrs))
	for i := range withAttrs {
		withArgs[i] = withAttrs[i]
	}
	return append(cases,
		slogHandlerBenchmarkCase{
			name: "WithAttrs/5",
			new: func(newHandler func(io.Writer) slog.Handler) func() {
				logger := slog.New(newHandler(benchmarkDiscardWriter{})).With(withArgs...)
				return func() { logger.Info(message) }
			},
		},
		slogHandlerBenchmarkCase{
			name: "Group/Depth3",
			new: func(newHandler func(io.Writer) slog.Handler) func() {
				logger := slog.New(newHandler(benchmarkDiscardWriter{})).
					WithGroup("http").WithGroup("request").WithGroup("result")
				return func() { logger.LogAttrs(context.Background(), slog.LevelInfo, message, slog.Int("status", 200)) }
			},
		},
	)
}

// BenchmarkNexuerVsSlogEncodeOnly provides a direct, discoverable comparison
// for the detailed Nexuer scenarios. Both implementations receive equivalent
// user messages and fields and write to io.Discard. Standard slog still incurs
// its native record timestamp and handler synchronization behavior.
func BenchmarkNexuerVsSlogEncodeOnly(b *testing.B) {
	formats := []nexuerSlogFormat{
		{
			name:          "JSON",
			nexuerHandler: func() nlog.Handler { return nlog.Json() },
			slogHandler: func(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
				return slog.NewJSONHandler(w, opts)
			},
		},
		{
			name:          "Text",
			nexuerHandler: func() nlog.Handler { return nlog.Text() },
			slogHandler: func(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
				return slog.NewTextHandler(w, opts)
			},
		},
	}
	cases := nexuerSlogCases()
	for _, format := range formats {
		b.Run(format.name, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					for _, bc := range cases {
						b.Run(bc.name, func(b *testing.B) {
							b.Run("Nexuer", func(b *testing.B) {
								runBenchmark(b, mode, bc.new(format).nexuer)
							})
							b.Run("Slog", func(b *testing.B) {
								runBenchmark(b, mode, bc.new(format).slog)
							})
						})
					}
				})
			}
		})
	}
}

func nexuerSlogCases() []nexuerSlogCase {
	short := getMessage(0)
	long := strings.Repeat("request completed successfully ", 128)
	escaped := "line one\nline two\t\"quoted\" \\ path 世界"
	cases := []nexuerSlogCase{
		messageNexuerSlogCase("Message/Short", short),
		messageNexuerSlogCase("Message/Long4K", long),
		messageNexuerSlogCase("Message/Escaped", escaped),
		{
			name: "Disabled",
			new: func(format nexuerSlogFormat) nexuerSlogPair {
				nexuer := nlog.New(io.Discard, format.nexuerHandler()).SetLevel(nlog.LevelError)
				standard := slog.New(format.slogHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
				return nexuerSlogPair{
					nexuer: func() { nexuer.InfoS(short) },
					slog:   func() { standard.Info(short) },
				}
			},
		},
	}

	for _, count := range []int{1, 2, 5, 10, 25, 50} {
		count := count
		kvs := makePrimitiveKVs(count)
		attrs := makeSlogIntAttrs(count)
		cases = append(cases, nexuerSlogCase{
			name: "Fields/" + strconv.Itoa(count),
			new: func(format nexuerSlogFormat) nexuerSlogPair {
				nexuer := nlog.New(io.Discard, format.nexuerHandler())
				standard := slog.New(format.slogHandler(io.Discard, nil))
				return nexuerSlogPair{
					nexuer: func() { nexuer.InfoS(short, kvs...) },
					slog: func() {
						standard.LogAttrs(context.Background(), slog.LevelInfo, short, attrs...)
					},
				}
			},
		})
	}

	for _, tc := range []struct {
		name  string
		value any
	}{
		{"Any/IntSlice", _tenInts},
		{"Any/StringSlice", _tenStrings},
		{"Any/Time", _tenTimes[0]},
		{"Any/TimeSlice", _tenTimes},
		{"Any/Struct", _oneUser},
		{"Any/StructSlice", _tenUsers},
		{"Any/Error", errExample},
	} {
		tc := tc
		kvs := []any{"value", tc.value}
		attr := slog.Any("value", tc.value)
		cases = append(cases, nexuerSlogCase{
			name: tc.name,
			new: func(format nexuerSlogFormat) nexuerSlogPair {
				nexuer := nlog.New(io.Discard, format.nexuerHandler())
				standard := slog.New(format.slogHandler(io.Discard, nil))
				return nexuerSlogPair{
					nexuer: func() { nexuer.InfoS(short, kvs...) },
					slog: func() {
						standard.LogAttrs(context.Background(), slog.LevelInfo, short, attr)
					},
				}
			},
		})
	}

	return append(cases,
		accumulatedNexuerSlogCase(),
		groupNexuerSlogCase("Group/Depth1", 1),
		groupNexuerSlogCase("Group/Depth3", 3),
		contextNexuerSlogCase(),
	)
}

func messageNexuerSlogCase(name, message string) nexuerSlogCase {
	return nexuerSlogCase{
		name: name,
		new: func(format nexuerSlogFormat) nexuerSlogPair {
			nexuer := nlog.New(io.Discard, format.nexuerHandler())
			standard := slog.New(format.slogHandler(io.Discard, nil))
			return nexuerSlogPair{
				nexuer: func() { nexuer.InfoS(message) },
				slog:   func() { standard.Info(message) },
			}
		},
	}
}

func makeSlogIntAttrs(count int) []slog.Attr {
	attrs := make([]slog.Attr, count)
	for i := range count {
		attrs[i] = slog.Int("key_"+strconv.Itoa(i), i)
	}
	return attrs
}

func accumulatedNexuerSlogCase() nexuerSlogCase {
	nexuerFields := makePrimitiveKVs(5)
	slogFields := makeSlogIntAttrs(5)
	return nexuerSlogCase{
		name: "AccumulatedFields/5",
		new: func(format nexuerSlogFormat) nexuerSlogPair {
			nexuer := nlog.New(io.Discard, format.nexuerHandler()).With(nexuerFields...)
			args := make([]any, len(slogFields))
			for i := range slogFields {
				args[i] = slogFields[i]
			}
			standard := slog.New(format.slogHandler(io.Discard, nil)).With(args...)
			return nexuerSlogPair{
				nexuer: func() { nexuer.InfoS(getMessage(0)) },
				slog:   func() { standard.Info(getMessage(0)) },
			}
		},
	}
}

func groupNexuerSlogCase(name string, depth int) nexuerSlogCase {
	return nexuerSlogCase{
		name: name,
		new: func(format nexuerSlogFormat) nexuerSlogPair {
			nexuer := nlog.New(io.Discard, format.nexuerHandler())
			standard := slog.New(format.slogHandler(io.Discard, nil))
			for i := range depth {
				group := "group_" + strconv.Itoa(i)
				nexuer = nexuer.WithGroup(group)
				standard = standard.WithGroup(group)
			}
			return nexuerSlogPair{
				nexuer: func() { nexuer.InfoS(getMessage(0), "status", 200) },
				slog: func() {
					standard.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0), slog.Int("status", 200))
				},
			}
		},
	}
}

func contextNexuerSlogCase() nexuerSlogCase {
	ctx := context.Background()
	return nexuerSlogCase{
		name: "Context",
		new: func(format nexuerSlogFormat) nexuerSlogPair {
			nexuer := nlog.New(io.Discard, format.nexuerHandler())
			standard := slog.New(format.slogHandler(io.Discard, nil))
			return nexuerSlogPair{
				nexuer: func() { _ = nexuer.Log(ctx, nlog.LevelInfo, getMessage(0), "status", 200) },
				slog: func() {
					standard.LogAttrs(ctx, slog.LevelInfo, getMessage(0), slog.Int("status", 200))
				},
			}
		},
	}
}
