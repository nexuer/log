package log

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestWriteCloser(t *testing.T) {
	fmt.Printf("%+v\n", errors.New("1111111"))
	fmt.Println(os.Stderr.Close(), os.Stdout.Close(), os.Stdin.Close())
}

func TestLoggerWith(t *testing.T) {
	l := New(os.Stderr).With(Group("g", "key", "1"))

	l2 := l.With("sep", "|", Group("g", "key", "2"))

	l.Info("info")
	l2.Info("info")

	slog.Info("info", slog.Group("g", "key", "1"), "sep", "|", slog.Group("g", "key", "2"))

	jl := New(os.Stderr, Json()).With(Group("g", "key", "1"))
	jl2 := jl.With("sep", "|", Group("g", "key", "2"))

	jl.Info("info")
	jl2.Info("info")

	sjl := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	sjl.Info("info", slog.Group("g", "key", "1"), "sep", "|", slog.Group("g", "key", "2"))
}

func TestReplacer(t *testing.T) {
	sl := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			fmt.Printf("%+v, %s\n", groups, a.Value)
			return a
		},
	}))

	sl.Info("info", slog.Group("g", "key", "1"), "sep", "|", slog.Group("g2", "key", "2"))
	fmt.Println("----------------------------------------- json ----------------------------------------- ")
	sjl := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			fmt.Printf("%+v, %s\n", groups, a.Value)
			return a
		},
	}))
	sjl.Info("info", slog.Group("g", "key", "1"), "sep", "|", slog.Group("g2", "key", "2"))
}

func TestLogger(t *testing.T) {
	//Info("it is info log")

	l := New(os.Stderr, Text(&HandlerOptions{Name: "http"})).With(
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

	jsonL := New(os.Stderr, Json(&HandlerOptions{Name: "grpc"})).With(
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

	jsonL.InfoS(fakeMessage,
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
