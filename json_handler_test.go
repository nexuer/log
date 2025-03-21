package log

import (
	"log/slog"
	"testing"
	"time"
)

func BenchmarkJsonInfo(b *testing.B) {
	l := New(output, Json())
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info(fakeMessage)
		}
	})
}

func BenchmarkJsonInfof(b *testing.B) {
	l := New(output, Json())
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Infof(fakeMessage)
		}
	})
}

func BenchmarkJsonInfoWith(b *testing.B) {
	l := New(output, Json()).With(
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
		i := 0
		for pb.Next() {
			i++
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

func BenchmarkSlogJsonWith(b *testing.B) {
	l := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		//ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		//	if a.Key == slog.LevelKey && a.Value.String() == "ERROR+119" {
		//		return slog.String(a.Key, "FATAL")
		//	}
		//	return a
		//},
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
