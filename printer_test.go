package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/nexuer/log"
)

func callerSource(t *testing.T, data []byte) log.Source {
	t.Helper()
	var record struct {
		Caller log.Source `json:"caller"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record.Caller
}

func TestLoggerCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, log.Json()).WithFields(log.DefaultFields...)
	_, _, line, _ := runtime.Caller(0)
	logger.Info("caller")
	caller := callerSource(t, buf.Bytes())
	if !strings.HasSuffix(caller.Function, ".TestLoggerCaller") || caller.Line != line+1 {
		t.Fatalf("caller = %s:%d (%s), want line %d in TestLoggerCaller", caller.File, caller.Line, caller.Function, line+1)
	}
}

func TestLoggerLogCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, log.Json()).WithFields(log.DefaultFields...)
	_, _, line, _ := runtime.Caller(0)
	if err := logger.Log(context.Background(), log.LevelInfo, "caller"); err != nil {
		t.Fatal(err)
	}
	caller := callerSource(t, buf.Bytes())
	if !strings.HasSuffix(caller.Function, ".TestLoggerLogCaller") || caller.Line != line+1 {
		t.Fatalf("caller = %s:%d (%s), want line %d in TestLoggerLogCaller", caller.File, caller.Line, caller.Function, line+1)
	}
}

func TestPrinterCaller(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf, log.Json()).WithFields(log.DefaultFields...))
	_, _, line, _ := runtime.Caller(0)
	printer.Info("caller")
	caller := callerSource(t, buf.Bytes())
	if !strings.HasSuffix(caller.Function, ".TestPrinterCaller") || caller.Line != line+1 {
		t.Fatalf("caller = %s:%d (%s), want line %d in TestPrinterCaller", caller.File, caller.Line, caller.Function, line+1)
	}
}

func TestNewPrinterUsesProvidedLogger(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf))

	printer.Info("hello")

	want := "INFO msg=hello\n"
	if got := buf.String(); got != want {
		t.Fatalf("printer output = %q, want %q", got, want)
	}
}

func TestPrinterWrite(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf))

	n, err := printer.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello") {
		t.Fatalf("Write returned n = %d, want %d", n, len("hello"))
	}

	want := "INFO msg=hello\n"
	if got := buf.String(); got != want {
		t.Fatalf("printer output = %q, want %q", got, want)
	}
}
