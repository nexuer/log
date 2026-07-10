package log

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
)

func TestLoggerTextOutput(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).InfoS("done", "id", 1, "ok", true)

	want := "INFO msg=done id=1 ok=true\n"
	if got := buf.String(); got != want {
		t.Fatalf("text output = %q, want %q", got, want)
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Json()).InfoS("done", "id", 1, "ok", true)

	want := `{"level":"INFO","msg":"done","id":1,"ok":true}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Json()).With("service", "api")

	logger.InfoS("done", "id", 1)

	want := `{"level":"INFO","msg":"done","service":"api","id":1}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerWithGroupText(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf).WithGroup("request")

	logger.InfoS("done", "id", "req-1", "method", "GET")

	want := "INFO msg=done request.id=req-1 request.method=GET\n"
	if got := buf.String(); got != want {
		t.Fatalf("text output = %q, want %q", got, want)
	}
}

func TestLoggerWithGroupJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Json()).WithGroup("request")

	logger.InfoS("done", "id", "req-1", "method", "GET")

	want := `{"level":"INFO","msg":"done","request":{"id":"req-1","method":"GET"}}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerWithGroupKeepsCallOrder(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Json()).
		With("service", "api").
		WithGroup("request").
		With("id", "req-1").
		WithGroup("user")

	logger.InfoS("login", "id", 42)

	want := `{"level":"INFO","msg":"login","service":"api","request":{"id":"req-1","user":{"id":42}}}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerWithEmptyGroupReturnsReceiver(t *testing.T) {
	logger := New(io.Discard)
	if got := logger.WithGroup(""); got != logger {
		t.Fatalf("WithGroup(empty) returned a new logger")
	}
}

func TestLoggerWithGroupReplacerGroups(t *testing.T) {
	var buf bytes.Buffer
	var calls []string
	logger := New(&buf, Text(&HandlerOptions{
		Replacer: func(ctx context.Context, groups []string, field Field) Field {
			if field.Key == LevelKey || field.Key == MessageKey {
				return field
			}
			calls = append(calls, field.Key+":"+joinGroups(groups))
			return field
		},
	})).WithGroup("request").With("id", "req-1")

	logger.InfoS("done", "method", "GET")

	wantCalls := []string{"id:request", "method:request"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("replacer calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestLoggerAcceptsSlogAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, Json()).With(
		slog.String("service", "api"),
		slog.Group("request",
			slog.String("id", "req-1"),
		),
	)

	logger.InfoS("done",
		slog.String("method", "GET"),
		slog.Int("status", 200),
	)

	want := `{"level":"INFO","msg":"done","service":"api","request":{"id":"req-1"},"method":"GET","status":200}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerAcceptsSlogAttrsWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf).WithGroup("request").With(
		slog.String("id", "req-1"),
	)

	logger.InfoS("done",
		slog.String("method", "GET"),
		slog.Int("status", 200),
	)

	want := "INFO msg=done request.id=req-1 request.method=GET request.status=200\n"
	if got := buf.String(); got != want {
		t.Fatalf("text output = %q, want %q", got, want)
	}
}

func TestLoggerSlogAttrReplacerGroups(t *testing.T) {
	var buf bytes.Buffer
	var calls []string
	logger := New(&buf, Json(&HandlerOptions{
		Replacer: func(ctx context.Context, groups []string, field Field) Field {
			if field.Key == LevelKey || field.Key == MessageKey {
				return field
			}
			calls = append(calls, field.Key+":"+joinGroups(groups))
			return field
		},
	})).WithGroup("request")

	logger.InfoS("done",
		slog.String("method", "GET"),
		slog.Group("user",
			slog.String("id", "u-1"),
		),
	)

	wantCalls := []string{"method:request", "id:request.user"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("replacer calls = %#v, want %#v", calls, wantCalls)
	}
}

func joinGroups(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	out := groups[0]
	for _, group := range groups[1:] {
		out += "." + group
	}
	return out
}

func TestLoggerFatalSExit(t *testing.T) {
	oldExit := exitFunc
	defer func() {
		exitFunc = oldExit
	}()

	tests := []struct {
		name string
		log  func(*Logger)
	}{
		{
			name: "fatal",
			log: func(l *Logger) {
				l.Fatal("fatal")
			},
		},
		{
			name: "fatalf",
			log: func(l *Logger) {
				l.Fatalf("fatalf %d", 1)
			},
		},
		{
			name: "fatals",
			log: func(l *Logger) {
				l.FatalS("fatalS", Err(errors.New("error msg")), "key", "value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			exitFunc = func(code int) {
				got = code
			}

			tt.log(New(io.Discard))

			if got != 1 {
				t.Fatalf("exit code = %d, want 1", got)
			}
		})
	}
}
