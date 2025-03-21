package log

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

var (
	fakeMessage = "Test logging, but use a somewhat realistic message length."
	output      = io.Discard
)

func BenchmarkTextInfo(b *testing.B) {
	l := New(output)
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info(fakeMessage)
		}
	})
}

func BenchmarkTextInfof(b *testing.B) {
	l := New(output)
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Infof(fakeMessage)
		}
	})
}

func BenchmarkSlogTextInfo(b *testing.B) {
	l := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info(fakeMessage)
		}
	})
}

func BenchmarkTextInfoWith(b *testing.B) {
	l := New(output).With(
		"ts", DefaultTimestamp,
		"caller", DefaultCaller,
		"key1", 10,
		"key2", 20.2,
		"key3", true,
		"key4", time.Now(),
		"key5", time.Duration(2222),
		"key6", "value6",
		"key7", "value7",
		"key8", "value8",
		Group("keys",
			"key9", "value9",
			"key10", "value10",
		),
	)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.InfoS(fakeMessage,
				"key11", "value11",
				"key12", "value12",
				"key13", time.Duration(7788),
				"key14", "value14",
				"key15", time.Now(),
				"key16", "value16",
				"key17", 30,
				"key18", 22.22,
				Group("keys",
					"key19", false,
					"key20", "value20",
				),
			)
		}
	})
}

func BenchmarkTextInfoWithFields(b *testing.B) {
	l := New(output).With(
		"ts", DefaultTimestamp,
		"caller", DefaultCaller,
		"key1", 10,
		"key2", 20.2,
		"key3", true,
		"key4", time.Now(),
		"key5", time.Duration(2222),
		"key6", "value6",
		"key7", "value7",
		"key8", "value8",
		"key9", "value9",
		"key10", "value10",
	)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.InfoS(fakeMessage,
				String("key11", "value11"),
				String("key12", "value12"),
				Duration("key13", time.Duration(7788)),
				String("key14", "value14"),
				Time("key15", time.Now()),
				String("key16", "value16"),
				Int("key17", 30),
				Float64("key18", 22.22),
				Bool("key19", false),
				String("key20", "value20"),
			)
		}
	})
}

func BenchmarkSlogTextWith(b *testing.B) {
	l := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})).With(
		"key1", 10,
		"key2", 20.2,
		"key3", true,
		"key4", time.Now(),
		"key5", time.Duration(2222),
		"key6", "value6",
		"key7", "value7",
		"key8", "value8",
		slog.Group("keys",
			"key9", "value9",
			"key10", "value10",
		),
	)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info(fakeMessage,
				"key11", "value11",
				"key12", "value12",
				"key13", time.Duration(7788),
				"key14", "value14",
				"key15", time.Now(),
				"key16", "value16",
				"key17", 30,
				"key18", 22.22,
				slog.Group("keys",
					"key19", false,
					"key20", "value20",
				),
			)
		}
	})
}

func BenchmarkSlogTextWithAttr(b *testing.B) {
	l := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})).With(
		"key1", "value1",
		"key2", "value2",
		"key3", "value3",
		"key4", "value4",
		"key5", "value5",
		"key6", "value6",
		"key7", "value7",
		"key8", "value8",
		"key9", "value9",
		"key10", "value10",
	)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info(fakeMessage,
				slog.String("key11", "value11"),
				slog.String("key12", "value12"),
				slog.String("key13", "value13"),
				slog.String("key14", "value14"),
				slog.String("key15", "value15"),
				slog.String("key16", "value16"),
				slog.String("key17", "value17"),
				slog.String("key18", "value18"),
				slog.String("key19", "value19"),
				slog.String("key20", "value20"),
			)
		}
	})
}
