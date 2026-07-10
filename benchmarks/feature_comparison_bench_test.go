package benchmarks

import (
	"context"
	"log/slog"
	"testing"

	"go.uber.org/zap"
)

// BenchmarkComparisonTextEncodeOnly compares each library's native text or
// console formatter. Console formats are not byte-for-byte equivalent.
func BenchmarkComparisonTextEncodeOnly(b *testing.B) {
	b.Run("Message", func(b *testing.B) {
		runComparisonCases(b, textComparisonCases(false))
	})
	b.Run("ShortFields", func(b *testing.B) {
		runComparisonCases(b, textComparisonCases(true))
	})
}

func textComparisonCases(withFields bool) []comparisonCase {
	if !withFields {
		return []comparisonCase{
			{"Nexuer", func() func() {
				logger := newNexuerTextLogger()
				return func() { logger.InfoS(getMessage(0)) }
			}},
			{"Zap", func() func() {
				logger := newZapTextLogger(zap.DebugLevel)
				return func() { logger.Info(getMessage(0)) }
			}},
			{"Zerolog", func() func() {
				logger := newZerologConsole()
				return func() { logger.Info().Msg(getMessage(0)) }
			}},
			{"Phuslu", func() func() {
				logger := newPhusluConsoleLog()
				return func() { logger.Info().Msg(getMessage(0)) }
			}},
			{"Slog", func() func() {
				logger := newSlogText()
				return func() { logger.Info(getMessage(0)) }
			}},
			{"Logrus", func() func() {
				logger := newLogrusText()
				return func() { logger.Info(getMessage(0)) }
			}},
		}
	}

	nexuerFields := []any{"request_id", "req-1", "status", 200, "ok", true}
	zapFields := []zap.Field{zap.String("request_id", "req-1"), zap.Int("status", 200), zap.Bool("ok", true)}
	slogFields := []any{slog.String("request_id", "req-1"), slog.Int("status", 200), slog.Bool("ok", true)}
	return []comparisonCase{
		{"Nexuer", func() func() {
			logger := newNexuerTextLogger()
			return func() { logger.InfoS(getMessage(0), nexuerFields...) }
		}},
		{"Zap", func() func() {
			logger := newZapTextLogger(zap.DebugLevel)
			return func() { logger.Info(getMessage(0), zapFields...) }
		}},
		{"Zerolog", func() func() {
			logger := newZerologConsole()
			return func() {
				logger.Info().Str("request_id", "req-1").Int("status", 200).Bool("ok", true).Msg(getMessage(0))
			}
		}},
		{"Phuslu", func() func() {
			logger := newPhusluConsoleLog()
			return func() {
				logger.Info().Str("request_id", "req-1").Int("status", 200).Bool("ok", true).Msg(getMessage(0))
			}
		}},
		{"Slog", func() func() {
			logger := newSlogText()
			return func() { logger.Info(getMessage(0), slogFields...) }
		}},
		{"Logrus", func() func() {
			logger := newLogrusText()
			fields := fakeLogrusFields()
			return func() { logger.WithFields(fields).Info(getMessage(0)) }
		}},
	}
}

// BenchmarkComparisonSlogFeatures compares group and Attr handling using each
// implementation's closest public API.
func BenchmarkComparisonSlogFeatures(b *testing.B) {
	b.Run("WithGroup", func(b *testing.B) {
		runComparisonCases(b, []comparisonCase{
			{"SlogJSON", func() func() {
				logger := newSlog().WithGroup("request").With("id", "req-1")
				return func() { logger.Info(getMessage(0), "status", 200, "ok", true) }
			}},
			{"PhusluSlogJSON", func() func() {
				logger := newPhusluSlog().WithGroup("request").With("id", "req-1")
				return func() { logger.Info(getMessage(0), "status", 200, "ok", true) }
			}},
			{"NexuerJSON", func() func() {
				logger := newNexuerLogger().WithGroup("request").With("id", "req-1")
				return func() { logger.InfoS(getMessage(0), "status", 200, "ok", true) }
			}},
		})
	})

	b.Run("Attrs", func(b *testing.B) {
		ctx := context.Background()
		runComparisonCases(b, []comparisonCase{
			{"SlogJSON/LogAttrs", func() func() {
				logger := newSlog(slog.String("request_id", "req-1"))
				return func() {
					logger.LogAttrs(ctx, slog.LevelInfo, getMessage(0), slog.Int("status", 200), slog.Bool("ok", true))
				}
			}},
			{"PhusluSlogJSON/LogAttrs", func() func() {
				logger := newPhusluSlog(slog.String("request_id", "req-1"))
				return func() {
					logger.LogAttrs(ctx, slog.LevelInfo, getMessage(0), slog.Int("status", 200), slog.Bool("ok", true))
				}
			}},
			{"NexuerJSON/InfoS", func() func() {
				logger := newNexuerLogger().With(slog.String("request_id", "req-1"))
				return func() { logger.InfoS(getMessage(0), slog.Int("status", 200), slog.Bool("ok", true)) }
			}},
		})
	})
}
