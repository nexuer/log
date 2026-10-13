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

//go:noinline
func logCallerWrapper(logger *log.Logger, ctx context.Context) error {
	return logger.Log(ctx, log.LevelInfo, "caller")
}

//go:noinline
func logCallerOuterWrapper(logger *log.Logger, ctx context.Context) error {
	return logCallerWrapper(logger, ctx)
}

func TestLoggerLogCallerDepth(t *testing.T) {
	for _, tc := range []struct {
		name        string
		loggerDepth int
		callDepth   int
		call        func(*log.Logger, context.Context) error
	}{
		{"call context", 0, 1, logCallerWrapper},
		{"logger context", 1, 0, logCallerWrapper},
		{"both contexts", 1, 1, logCallerOuterWrapper},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New(&buf, log.Json()).WithFields(log.DefaultFields...).
				WithContext(log.AddCallerDepth(context.Background(), tc.loggerDepth))
			ctx := log.AddCallerDepth(context.Background(), tc.callDepth)
			_, file, line, _ := runtime.Caller(0)
			if err := tc.call(logger, ctx); err != nil {
				t.Fatal(err)
			}
			caller := callerSource(t, buf.Bytes())
			if !strings.HasSuffix(file, caller.File) || caller.Line != line+1 ||
				!strings.Contains(caller.Function, "TestLoggerLogCallerDepth") {
				t.Fatalf("caller = %#v, want test call at %s:%d", caller, file, line+1)
			}
		})
	}
}

func TestLoggerLogValuerUsesCallContext(t *testing.T) {
	type contextKey string
	const key contextKey = "request"
	const hidden contextKey = "hidden"
	loggerCtx := log.AddCallerDepth(context.WithValue(context.Background(), key, "logger"), 1)
	callCtx, cancel := context.WithCancel(context.WithValue(
		context.WithValue(context.Background(), key, "call"), hidden, "not logged"))
	cancel()
	var buf bytes.Buffer
	logger := log.New(&buf, log.Json()).WithContext(loggerCtx).WithFields(
		log.Dynamic("request", func(ctx context.Context) log.Value {
			if ctx.Err() != context.Canceled {
				t.Error("Valuer did not receive call context cancellation")
			}
			return log.AnyValue(ctx.Value(key))
		}),
	)
	if err := logger.Log(callCtx, log.LevelInfo, "context"); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["request"] != "call" {
		t.Fatalf("request = %v, want call", record["request"])
	}
	if _, ok := record["hidden"]; ok {
		t.Fatal("context value was logged without explicit configuration")
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
