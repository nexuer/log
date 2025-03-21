package log

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWriteCloser(t *testing.T) {
	fmt.Println(os.Stderr.Close(), os.Stdout.Close(), os.Stdin.Close())
}

func TestLogger(t *testing.T) {
	//Info("it is info log")

	l := New(os.Stderr, Text("HTTP")).With(
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

	jsonL := New(os.Stderr, Json("gRPC")).With(
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
