package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexuer/log"
)

func callerFunction(t *testing.T, data []byte) string {
	t.Helper()
	var record struct {
		Caller log.Source `json:"caller"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record.Caller.Function
}

func TestLoggerCaller(t *testing.T) {
	var buf bytes.Buffer
	log.New(&buf, log.Json()).With(log.DefaultFields...).Info("caller")
	if function := callerFunction(t, buf.Bytes()); !strings.HasSuffix(function, ".TestLoggerCaller") {
		t.Fatalf("caller function = %q, want TestLoggerCaller", function)
	}
}

func TestLoggerLogCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, log.Json()).With(log.DefaultFields...)
	if err := logger.Log(context.Background(), log.LevelInfo, "caller"); err != nil {
		t.Fatal(err)
	}
	if function := callerFunction(t, buf.Bytes()); !strings.HasSuffix(function, ".TestLoggerLogCaller") {
		t.Fatalf("caller function = %q, want TestLoggerLogCaller", function)
	}
}

func TestPrinterCaller(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf, log.Json()).With(log.DefaultFields...))
	printer.Info("caller")
	if function := callerFunction(t, buf.Bytes()); !strings.HasSuffix(function, ".TestPrinterCaller") {
		t.Fatalf("caller function = %q, want TestPrinterCaller", function)
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
