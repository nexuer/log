package benchmarks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/nexuer/log"
)

type nexuerBenchmarkCase struct {
	name string
	new  func(handler log.Handler) func()
}

var nexuerLoggerSink *log.Logger

func runNexuerCases(b *testing.B, cases []nexuerBenchmarkCase) {
	b.Helper()
	for _, format := range []struct {
		name    string
		handler func() log.Handler
	}{
		{"JSON", func() log.Handler { return log.Json() }},
		{"Text", func() log.Handler { return log.Text() }},
	} {
		b.Run(format.name, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					for _, bc := range cases {
						b.Run(bc.name, func(b *testing.B) {
							runBenchmark(b, mode, bc.new(format.handler()))
						})
					}
				})
			}
		})
	}
}

// BenchmarkNexuerMessages separates message API and escaping costs from field
// encoding. io.Discard makes these encode-only measurements.
func BenchmarkNexuerMessages(b *testing.B) {
	short := getMessage(0)
	long := strings.Repeat("request completed successfully ", 128)
	escaped := "line one\nline two\t\"quoted\" \\ path 世界"
	formatArgs := fakeFmtArgs()
	cases := []nexuerBenchmarkCase{
		{"InfoS/Short", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.InfoS(short) }
		}},
		{"InfoS/Long4K", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.InfoS(long) }
		}},
		{"InfoS/Escaped", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.InfoS(escaped) }
		}},
		{"Info/SingleString", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.Info(short) }
		}},
		{"Infof/NoArgs", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.Infof(short) }
		}},
		{"Infof/Formatting", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.Infof("%v %v %v %s %v %v %v %v %v %s", formatArgs...) }
		}},
	}
	runNexuerCases(b, cases)
}

// BenchmarkNexuerDisabled keeps argument construction outside the timed loop
// except for the explicitly named Constructed cases.
func BenchmarkNexuerDisabled(b *testing.B) {
	fields := makePrimitiveKVs(10)
	cases := []nexuerBenchmarkCase{
		{"InfoS", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).SetLevel(log.LevelError)
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"Info", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).SetLevel(log.LevelError)
			return func() { logger.Info(getMessage(0)) }
		}},
		{"InfoS/PrebuiltFields10", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).SetLevel(log.LevelError)
			return func() { logger.InfoS(getMessage(0), fields...) }
		}},
		{"InfoS/ConstructedFields", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).SetLevel(log.LevelError)
			return func() {
				logger.InfoS(getMessage(0), "request_id", "req-1", "method", "GET", "status", 200, "ok", true)
			}
		}},
	}
	runNexuerCases(b, cases)
}

// BenchmarkNexuerFieldScale measures primitive key-value parsing and encoding
// without allocating the field fixture inside the timed loop.
func BenchmarkNexuerFieldScale(b *testing.B) {
	var cases []nexuerBenchmarkCase
	for _, count := range []int{1, 2, 5, 10, 25, 50} {
		count := count
		fields := makePrimitiveKVs(count)
		cases = append(cases, nexuerBenchmarkCase{
			name: fmt.Sprintf("Fields%d", count),
			new: func(h log.Handler) func() {
				logger := log.New(io.Discard, h)
				return func() { logger.InfoS(getMessage(0), fields...) }
			},
		})
	}
	runNexuerCases(b, cases)
}

func makePrimitiveKVs(count int) []any {
	kvs := make([]any, 0, count*2)
	for i := range count {
		kvs = append(kvs, "key_"+strconv.Itoa(i), i)
	}
	return kvs
}

func fieldsAsAny(fields []log.Field) []any {
	args := make([]any, len(fields))
	for i := range fields {
		args[i] = fields[i]
	}
	return args
}

func attrsAsAny(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i := range attrs {
		args[i] = attrs[i]
	}
	return args
}

// BenchmarkNexuerFieldForms compares equivalent short fields through key-value,
// native Field, and slog.Attr input. All fixtures are prebuilt.
func BenchmarkNexuerFieldForms(b *testing.B) {
	kvs := []any{"request_id", "req-1", "status", 200, "ok", true}
	fields := fieldsAsAny([]log.Field{
		log.String("request_id", "req-1"),
		log.Int("status", 200),
		log.Bool("ok", true),
	})
	attrs := attrsAsAny([]slog.Attr{
		slog.String("request_id", "req-1"),
		slog.Int("status", 200),
		slog.Bool("ok", true),
	})
	cases := []nexuerBenchmarkCase{
		{"KeyValues", newNexuerFieldsCall(kvs)},
		{"Fields", newNexuerFieldsCall(fields)},
		{"SlogAttrs", newNexuerFieldsCall(attrs)},
		{"ConstructedKeyValues", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { logger.InfoS(getMessage(0), "request_id", "req-1", "status", 200, "ok", true) }
		}},
		{"ConstructedFields", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() {
				logger.InfoS(getMessage(0), log.String("request_id", "req-1"), log.Int("status", 200), log.Bool("ok", true))
			}
		}},
	}
	runNexuerCases(b, cases)
}

func newNexuerFieldsCall(fields []any) func(log.Handler) func() {
	return func(h log.Handler) func() {
		logger := log.New(io.Discard, h)
		return func() { logger.InfoS(getMessage(0), fields...) }
	}
}

// BenchmarkNexuerAnyValues isolates generic encoding by concrete value type.
func BenchmarkNexuerAnyValues(b *testing.B) {
	cases := []struct {
		name  string
		value any
	}{
		{"IntSlice", _tenInts},
		{"StringSlice", _tenStrings},
		{"Time", _tenTimes[0]},
		{"TimeSlice", _tenTimes},
		{"Struct", _oneUser},
		{"StructSlice", _tenUsers},
		{"Error", errExample},
	}
	benchCases := make([]nexuerBenchmarkCase, 0, len(cases))
	for _, tc := range cases {
		tc := tc
		fields := []any{"value", tc.value}
		benchCases = append(benchCases, nexuerBenchmarkCase{
			name: tc.name,
			new:  newNexuerFieldsCall(fields),
		})
	}
	runNexuerCases(b, benchCases)
}

// BenchmarkNexuerAccumulatedFields measures preformatted logger context and
// lazy values separately.
func BenchmarkNexuerAccumulatedFields(b *testing.B) {
	ctx := context.Background()
	cases := []nexuerBenchmarkCase{
		{"WithKeyValues", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).With("request_id", "req-1", "service", "api")
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"WithFields", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).WithFields(log.String("request_id", "req-1"), log.String("service", "api"))
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"TimestampValuer", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).With("ts", log.Timestamp("2006-01-02T15:04:05Z07:00"))
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"CallerValuer", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).With("caller", log.Caller(1))
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"DefaultFields", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).With(log.DefaultFields...)
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"WithContext", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).WithContext(ctx)
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"LogContext", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			return func() { _ = logger.Log(ctx, log.LevelInfo, getMessage(0), "request_id", "req-1") }
		}},
	}
	runNexuerCases(b, cases)
}

// BenchmarkNexuerGroups covers group depth and grouped Fields.
func BenchmarkNexuerGroups(b *testing.B) {
	cases := []nexuerBenchmarkCase{
		{"WithGroup/Depth1", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).WithGroup("request").With("id", "req-1")
			return func() { logger.InfoS(getMessage(0), "status", 200) }
		}},
		{"WithGroup/Depth3", func(h log.Handler) func() {
			logger := log.New(io.Discard, h).WithGroup("http").WithGroup("request").WithGroup("result").With("id", "req-1")
			return func() { logger.InfoS(getMessage(0), "status", 200) }
		}},
		{"GroupField", func(h log.Handler) func() {
			logger := log.New(io.Discard, h)
			group := log.Group("request", "id", "req-1", "status", 200)
			return func() { logger.InfoS(getMessage(0), group) }
		}},
	}
	runNexuerCases(b, cases)
}

// BenchmarkNexuerReplacer measures the optional field callback separately so
// each format receives a handler configured with the same callback.
func BenchmarkNexuerReplacer(b *testing.B) {
	redact := func(_ context.Context, _ []string, field log.Field) log.Field {
		if field.Key == "secret" {
			return log.String(field.Key, "[redacted]")
		}
		return field
	}
	for _, format := range []struct {
		name    string
		handler log.Handler
	}{
		{"JSON", log.Json(&log.HandlerOptions{Replacer: redact})},
		{"Text", log.Text(&log.HandlerOptions{Replacer: redact})},
	} {
		b.Run(format.name, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					logger := log.New(io.Discard, format.handler)
					runBenchmark(b, mode, func() {
						logger.InfoS(getMessage(0), "secret", "token", "status", 200)
					})
				})
			}
		})
	}
}

// BenchmarkNexuerConstruction measures logger derivation separately from log
// emission. These operations are intentionally excluded from all other cases.
func BenchmarkNexuerConstruction(b *testing.B) {
	ctx := context.Background()
	base := log.New(io.Discard, log.Json())
	fields := []log.Field{log.String("request_id", "req-1"), log.Int("status", 200)}
	for _, bc := range []struct {
		name string
		fn   func()
	}{
		{"NewJSON", func() { nexuerLoggerSink = log.New(io.Discard, log.Json()) }},
		{"NewText", func() { nexuerLoggerSink = log.New(io.Discard, log.Text()) }},
		{"WithKeyValues", func() { nexuerLoggerSink = base.With("request_id", "req-1", "status", 200) }},
		{"WithFields", func() { nexuerLoggerSink = base.WithFields(fields...) }},
		{"WithGroup", func() { nexuerLoggerSink = base.WithGroup("request") }},
		{"WithGroupDepth3", func() { nexuerLoggerSink = base.WithGroup("http").WithGroup("request").WithGroup("result") }},
		{"WithContext", func() { nexuerLoggerSink = base.WithContext(ctx) }},
	} {
		b.Run(bc.name, func(b *testing.B) {
			runBenchmark(b, benchmarkSerial, bc.fn)
		})
	}
}
