package benchmarks

import (
	"io"
	"log/slog"
	"testing"

	nlog "github.com/nexuer/log"
	phuslulog "github.com/phuslu/log"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type benchmarkMode string

const (
	benchmarkSerial   benchmarkMode = "Serial"
	benchmarkParallel benchmarkMode = "Parallel"
)

func runBenchmark(b *testing.B, mode benchmarkMode, fn func()) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	if mode == benchmarkSerial {
		for range b.N {
			fn()
		}
		return
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fn()
		}
	})
}

// benchmarkDiscardWriter forces loggers through their Write path while keeping
// the sink cost negligible. Unlike io.Discard, Nexuer does not special-case it.
type benchmarkDiscardWriter struct{}

func (benchmarkDiscardWriter) Write(p []byte) (int, error) { return len(p), nil }

type comparisonCase struct {
	name string
	new  func() func()
}

func runComparisonCases(b *testing.B, cases []comparisonCase) {
	b.Helper()
	for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
		b.Run(string(mode), func(b *testing.B) {
			for _, bc := range cases {
				b.Run(bc.name, func(b *testing.B) {
					runBenchmark(b, mode, bc.new())
				})
			}
		})
	}
}

// BenchmarkComparisonEncodeOnly measures level checks, field handling, and
// encoding. All loggers target io.Discard. It intentionally does not represent
// writer synchronization or device I/O.
func BenchmarkComparisonEncodeOnly(b *testing.B) {
	b.Run("Disabled", func(b *testing.B) {
		runComparisonCases(b, disabledComparisonCases())
	})
	b.Run("Message", func(b *testing.B) {
		runComparisonCases(b, messageComparisonCases())
	})
	b.Run("AccumulatedFields", func(b *testing.B) {
		runComparisonCases(b, accumulatedComparisonCases())
	})
	b.Run("CallsiteFields", func(b *testing.B) {
		runComparisonCases(b, callsiteComparisonCases())
	})
}

func disabledComparisonCases() []comparisonCase {
	return []comparisonCase{
		{"Nexuer", func() func() {
			logger := newDisabledNexuerLogger()
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"Zap", func() func() {
			logger := newZapLogger(zap.ErrorLevel)
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Zerolog", func() func() {
			logger := newDisabledZerolog()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Phuslu", func() func() {
			logger := newDisabledPhusluLog()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Slog", func() func() {
			logger := newDisabledSlog()
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Logrus", func() func() {
			logger := newDisabledLogrus()
			return func() { logger.Info(getMessage(0)) }
		}},
	}
}

func messageComparisonCases() []comparisonCase {
	return []comparisonCase{
		{"Nexuer", func() func() {
			logger := newNexuerLogger()
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"Zap", func() func() {
			logger := newZapLogger(zap.DebugLevel)
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Zerolog", func() func() {
			logger := newZerolog()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Phuslu", func() func() {
			logger := newPhusluLog()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Slog", func() func() {
			logger := newSlog()
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Logrus", func() func() {
			logger := newLogrus()
			return func() { logger.Info(getMessage(0)) }
		}},
	}
}

func accumulatedComparisonCases() []comparisonCase {
	return []comparisonCase{
		{"Nexuer", func() func() {
			logger := newNexuerLogger().With(fakeNexuerLogKvs()...)
			return func() { logger.InfoS(getMessage(0)) }
		}},
		{"Zap", func() func() {
			logger := newZapLogger(zap.DebugLevel).With(fakeFields()...)
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Zerolog", func() func() {
			logger := fakeZerologContext(newZerolog().With()).Logger()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Phuslu", func() func() {
			logger := newPhusluLog()
			logger.Context = fakePhusluContext()
			return func() { logger.Info().Msg(getMessage(0)) }
		}},
		{"Slog", func() func() {
			logger := newSlog(fakeSlogFields()...)
			return func() { logger.Info(getMessage(0)) }
		}},
		{"Logrus", func() func() {
			logger := newLogrus().WithFields(fakeLogrusFields())
			return func() { logger.Info(getMessage(0)) }
		}},
	}
}

func callsiteComparisonCases() []comparisonCase {
	return []comparisonCase{
		{"Nexuer", func() func() {
			logger := newNexuerLogger()
			fields := fakeNexuerLogKvs()
			return func() { logger.InfoS(getMessage(0), fields...) }
		}},
		{"Zap", func() func() {
			logger := newZapLogger(zap.DebugLevel)
			fields := fakeFields()
			return func() { logger.Info(getMessage(0), fields...) }
		}},
		{"Zerolog", func() func() {
			logger := newZerolog()
			return func() { fakeZerologFields(logger.Info()).Msg(getMessage(0)) }
		}},
		{"Phuslu", func() func() {
			logger := newPhusluLog()
			return func() { fakePhusluFields(logger.Info()).Msg(getMessage(0)) }
		}},
		{"Slog", func() func() {
			logger := newSlog()
			fields := fakeSlogArgs()
			return func() { logger.Info(getMessage(0), fields...) }
		}},
		{"Logrus", func() func() {
			logger := newLogrus()
			fields := fakeLogrusFields()
			return func() { logger.WithFields(fields).Info(getMessage(0)) }
		}},
	}
}

type writerCase struct {
	name string
	new  func(w io.Writer, withFields bool) func()
}

// BenchmarkComparisonWritePath forces every implementation to call Write.
// It preserves each library's native synchronization policy. No external lock
// is added to implementations that expect a concurrency-safe writer.
func BenchmarkComparisonWritePath(b *testing.B) {
	for _, withFields := range []bool{false, true} {
		scenario := "Message"
		if withFields {
			scenario = "ShortFields"
		}
		b.Run(scenario, func(b *testing.B) {
			for _, mode := range []benchmarkMode{benchmarkSerial, benchmarkParallel} {
				b.Run(string(mode), func(b *testing.B) {
					for _, wc := range writerComparisonCases() {
						b.Run(wc.name, func(b *testing.B) {
							w := benchmarkDiscardWriter{}
							runBenchmark(b, mode, wc.new(w, withFields))
						})
					}
				})
			}
		})
	}
}

func writerComparisonCases() []writerCase {
	return []writerCase{
		{"Nexuer", newNexuerWriterCall},
		{"Zap", newZapWriterCall},
		{"Zerolog", newZerologWriterCall},
		{"Phuslu", newPhusluWriterCall},
		{"Slog", newSlogWriterCall},
		{"Logrus", newLogrusWriterCall},
	}
}

func newNexuerWriterCall(w io.Writer, withFields bool) func() {
	logger := nlog.New(w, nlog.Json())
	if withFields {
		return func() { logger.InfoS(getMessage(0), "request_id", "req-1", "status", 200, "ok", true) }
	}
	return func() { logger.InfoS(getMessage(0)) }
}

func newZapWriterCall(w io.Writer, withFields bool) func() {
	ec := zap.NewProductionEncoderConfig()
	ws := zapcore.AddSync(w)
	logger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(ec), ws, zap.DebugLevel))
	if withFields {
		return func() {
			logger.Info(getMessage(0), zap.String("request_id", "req-1"), zap.Int("status", 200), zap.Bool("ok", true))
		}
	}
	return func() { logger.Info(getMessage(0)) }
}

func newZerologWriterCall(w io.Writer, withFields bool) func() {
	logger := zerolog.New(w).With().Timestamp().Logger()
	if withFields {
		return func() {
			logger.Info().Str("request_id", "req-1").Int("status", 200).Bool("ok", true).Msg(getMessage(0))
		}
	}
	return func() { logger.Info().Msg(getMessage(0)) }
}

func newPhusluWriterCall(w io.Writer, withFields bool) func() {
	logger := &phuslulog.Logger{Level: phuslulog.DebugLevel, Writer: phuslulog.IOWriter{Writer: w}}
	if withFields {
		return func() {
			logger.Info().Str("request_id", "req-1").Int("status", 200).Bool("ok", true).Msg(getMessage(0))
		}
	}
	return func() { logger.Info().Msg(getMessage(0)) }
}

func newSlogWriterCall(w io.Writer, withFields bool) func() {
	logger := slog.New(slog.NewJSONHandler(w, nil))
	if withFields {
		return func() {
			logger.LogAttrs(nil, slog.LevelInfo, getMessage(0), slog.String("request_id", "req-1"), slog.Int("status", 200), slog.Bool("ok", true))
		}
	}
	return func() { logger.LogAttrs(nil, slog.LevelInfo, getMessage(0)) }
}

func newLogrusWriterCall(w io.Writer, withFields bool) func() {
	logger := &logrus.Logger{
		Out:       w,
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
	if withFields {
		fields := logrus.Fields{"request_id": "req-1", "status": 200, "ok": true}
		return func() { logger.WithFields(fields).Info(getMessage(0)) }
	}
	return func() { logger.Info(getMessage(0)) }
}
