// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package benchmarks

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSS(t *testing.T) {
	logger := fakeZerologContext(zerolog.New(io.Discard).With()).Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	logger.Info().Msg("test msg")

	ec := zap.NewProductionEncoderConfig()
	ec.EncodeDuration = zapcore.NanosDurationEncoder
	ec.EncodeTime = zapcore.EpochNanosTimeEncoder
	enc := zapcore.NewConsoleEncoder(ec)
	zapLogger := zap.New(zapcore.NewCore(
		enc,
		os.Stderr,
		zapcore.DebugLevel,
	)).With(fakeFields()...)
	zapLogger.Info("test msg")
}

func BenchmarkDisabledWithoutFields(b *testing.B) {
	b.Logf("Logging at a disabled level without any structured context.")
	b.Run("NexuerLog.Info", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Infof", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.InfoS", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Formatting", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if m := logger.Check(zap.InfoLevel, getMessage(0)); m != nil {
					m.Write()
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newDisabledApexLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newDisabledLogrus()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := newDisabledZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msg(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog.Formatting", func(b *testing.B) {
		logger := newDisabledZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msgf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newDisabledSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newDisabledSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0))
			}
		})
	})
}

func BenchmarkDisabledAccumulatedContext(b *testing.B) {
	b.Logf("Logging at a disabled level with some accumulated context.")
	b.Run("NexuerLog.Info", func(b *testing.B) {
		logger := newDisabledNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Info.hasValuer", func(b *testing.B) {
		logger := newDisabledNexuerLogger().With(fakeNexuerLogKvs(true)...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Infof", func(b *testing.B) {
		logger := newDisabledNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.InfoS", func(b *testing.B) {
		logger := newDisabledNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Formatting", func(b *testing.B) {
		logger := newDisabledNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).With(fakeFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).With(fakeFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if m := logger.Check(zap.InfoLevel, getMessage(0)); m != nil {
					m.Write()
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newDisabledApexLog().WithFields(fakeApexFields())
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newDisabledLogrus().WithFields(fakeLogrusFields())
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := fakeZerologContext(newDisabledZerolog().With()).Logger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msg(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog.Formatting", func(b *testing.B) {
		logger := fakeZerologContext(newDisabledZerolog().With()).Logger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msgf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newDisabledSlog(fakeSlogFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newDisabledSlog(fakeSlogFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0))
			}
		})
	})
}

func BenchmarkDisabledAddingFields(b *testing.B) {
	b.Logf("Logging at a disabled level, adding context at each log site.")
	b.Run("NexuerLog", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0), fakeNexuerLogKvs()...)
			}
		})
	})
	b.Run("NexuerLog.hasValuer", func(b *testing.B) {
		logger := newDisabledNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0), fakeNexuerLogKvs(true)...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeFields()...)
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if m := logger.Check(zap.InfoLevel, getMessage(0)); m != nil {
					m.Write(fakeFields()...)
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.ErrorLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infow(getMessage(0), fakeSugarFields()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newDisabledApexLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.WithFields(fakeApexFields()).Info(getMessage(0))
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newDisabledLogrus()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.WithFields(fakeLogrusFields()).Info(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := newDisabledZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				fakeZerologFields(logger.Info()).Msg(getMessage(0))
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newDisabledSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeSlogArgs()...)
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newDisabledSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0), fakeSlogFields()...)
			}
		})
	})
}

func BenchmarkWithoutFields(b *testing.B) {
	b.Logf("Logging without any structured context.")
	b.Run("NexuerLog.Info", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Infof", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.InfoS", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Formatting", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if ce := logger.Check(zap.InfoLevel, getMessage(0)); ce != nil {
					ce.Write()
				}
			}
		})
	})
	b.Run("Zap.CheckSampled", func(b *testing.B) {
		logger := newSampledLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				i++
				if ce := logger.Check(zap.InfoLevel, getMessage(i)); ce != nil {
					ce.Write()
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newApexLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("go-kit/kit/log", func(b *testing.B) {
		logger := newKitLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := logger.Log(getMessage(0), getMessage(1)); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	b.Run("inconshreveable/log15", func(b *testing.B) {
		logger := newLog15()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newLogrus()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("stdlib.Println", func(b *testing.B) {
		logger := log.New(&Discarder{}, "", log.LstdFlags)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Println(getMessage(0))
			}
		})
	})
	b.Run("stdlib.Printf", func(b *testing.B) {
		logger := log.New(&Discarder{}, "", log.LstdFlags)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Printf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := newZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msg(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog.Formatting", func(b *testing.B) {
		logger := newZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msgf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("rs/zerolog.Check", func(b *testing.B) {
		logger := newZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if e := logger.Info(); e.Enabled() {
					e.Msg(getMessage(0))
				}
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0))
			}
		})
	})
}

func BenchmarkAccumulatedContext(b *testing.B) {
	b.Logf("Logging with some accumulated context.")
	b.Run("NexuerLog.Info", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Info.hasValuer", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs(true)...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Infof", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Infof.hasValuer", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs(true)...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.InfoS", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0))
			}
		})
	})
	b.Run("NexuerLog.Formatting", func(b *testing.B) {
		logger := newNexuerLogger().With(fakeNexuerLogKvs()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if ce := logger.Check(zap.InfoLevel, getMessage(0)); ce != nil {
					ce.Write()
				}
			}
		})
	})
	b.Run("Zap.CheckSampled", func(b *testing.B) {
		logger := newSampledLogger(zap.DebugLevel).With(fakeFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				i++
				if ce := logger.Check(zap.InfoLevel, getMessage(i)); ce != nil {
					ce.Write()
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newApexLog().WithFields(fakeApexFields())
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("go-kit/kit/log", func(b *testing.B) {
		logger := newKitLog(fakeSugarFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := logger.Log(getMessage(0), getMessage(1)); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	b.Run("inconshreveable/log15", func(b *testing.B) {
		logger := newLog15().New(fakeSugarFields())
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newLogrus().WithFields(fakeLogrusFields())
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := fakeZerologContext(newZerolog().With()).Logger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msg(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog.Check", func(b *testing.B) {
		logger := fakeZerologContext(newZerolog().With()).Logger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if e := logger.Info(); e.Enabled() {
					e.Msg(getMessage(0))
				}
			}
		})
	})
	b.Run("rs/zerolog.Formatting", func(b *testing.B) {
		logger := fakeZerologContext(newZerolog().With()).Logger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msgf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newSlog(fakeSlogFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newSlog(fakeSlogFields()...)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0))
			}
		})
	})
}

func BenchmarkAddingFields(b *testing.B) {
	b.Logf("Logging with additional context at each log site.")
	b.Run("NexuerLog", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0), fakeNexuerLogKvs()...)
			}
		})
	})
	b.Run("NexuerLog.hsaValuer", func(b *testing.B) {
		logger := newNexuerLogger()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.InfoS(getMessage(0), fakeNexuerLogKvs(true)...)
			}
		})
	})
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeFields()...)
			}
		})
	})
	b.Run("Zap.Check", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if ce := logger.Check(zap.InfoLevel, getMessage(0)); ce != nil {
					ce.Write(fakeFields()...)
				}
			}
		})
	})
	b.Run("Zap.CheckSampled", func(b *testing.B) {
		logger := newSampledLogger(zap.DebugLevel)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				i++
				if ce := logger.Check(zap.InfoLevel, getMessage(i)); ce != nil {
					ce.Write(fakeFields()...)
				}
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infow(getMessage(0), fakeSugarFields()...)
			}
		})
	})
	b.Run("apex/log", func(b *testing.B) {
		logger := newApexLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.WithFields(fakeApexFields()).Info(getMessage(0))
			}
		})
	})
	b.Run("go-kit/kit/log", func(b *testing.B) {
		logger := newKitLog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := logger.Log(fakeSugarFields()...); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	b.Run("inconshreveable/log15", func(b *testing.B) {
		logger := newLog15()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeSugarFields()...)
			}
		})
	})
	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newLogrus()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.WithFields(fakeLogrusFields()).Info(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog", func(b *testing.B) {
		logger := newZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				fakeZerologFields(logger.Info()).Msg(getMessage(0))
			}
		})
	})
	b.Run("rs/zerolog.Check", func(b *testing.B) {
		logger := newZerolog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if e := logger.Info(); e.Enabled() {
					fakeZerologFields(e).Msg(getMessage(0))
				}
			}
		})
	})
	b.Run("slog", func(b *testing.B) {
		logger := newSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeSlogArgs()...)
			}
		})
	})
	b.Run("slog.LogAttrs", func(b *testing.B) {
		logger := newSlog()
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.LogAttrs(context.Background(), slog.LevelInfo, getMessage(0), fakeSlogFields()...)
			}
		})
	})
}
