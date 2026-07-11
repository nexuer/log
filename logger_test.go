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

func TestLoggerWithFieldsUsesZeroCapacityBuffer(t *testing.T) {
	logger := New(io.Discard, Json()).With("k", "v")
	handler := logger.handler.(*jsonHandler).handler
	if len(handler.preformattedAttrs) != 1 {
		t.Fatalf("preformatted segments = %d, want 1", len(handler.preformattedAttrs))
	}
	if got := cap(handler.preformattedAttrs[0].bytes); got >= 1024 {
		t.Fatalf("preformatted buffer capacity = %d, want less than 1024", got)
	}
}

func TestLoggerWithMultipleValuersPreservesSegments(t *testing.T) {
	var buf bytes.Buffer
	first := Valuer(func(context.Context) Value { return StringValue("one") })
	second := Valuer(func(context.Context) Value { return IntValue(2) })
	logger := New(&buf, Json()).With(
		"before", true,
		"first", first,
		"middle", "value",
		"second", second,
		"after", 3,
	)

	logger.InfoS("done")
	want := `{"level":"INFO","msg":"done","before":true,"first":"one","middle":"value","second":2,"after":3}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestLoggerKeyValueContracts(t *testing.T) {
	t.Run("odd input", func(t *testing.T) {
		var text, json bytes.Buffer
		New(&text).InfoS("done", "missing")
		New(&json, Json()).InfoS("done", "missing")
		if got, want := text.String(), `INFO msg=done <BAD_KEY>=missing`+"\n"; got != want {
			t.Fatalf("text output = %q, want %q", got, want)
		}
		if got, want := json.String(), `{"level":"INFO","msg":"done","<BAD_KEY>":"missing"}`+"\n"; got != want {
			t.Fatalf("json output = %q, want %q", got, want)
		}
	})

	t.Run("duplicate keys", func(t *testing.T) {
		var text, json bytes.Buffer
		New(&text).InfoS("original", "msg", "replacement", "id", 1, "id", 2)
		New(&json, Json()).InfoS("original", "msg", "replacement", "id", 1, "id", 2)
		if got, want := text.String(), "INFO msg=original msg=replacement id=1 id=2\n"; got != want {
			t.Fatalf("text output = %q, want %q", got, want)
		}
		if got, want := json.String(), `{"level":"INFO","msg":"original","msg":"replacement","id":1,"id":2}`+"\n"; got != want {
			t.Fatalf("json output = %q, want %q", got, want)
		}
	})
}

func TestLoggerMixedFieldFormsKeepOrder(t *testing.T) {
	var buf bytes.Buffer
	dynamic := Valuer(func(context.Context) Value { return StringValue("resolved") })
	logger := New(&buf, Json()).With(
		String("field", "typed"),
		slog.String("attr", "slog"),
		"pair", "plain",
		Group("group", "inside", 1, "dynamic", dynamic),
	)

	logger.InfoS("done", Bool("tail", true))
	want := `{"level":"INFO","msg":"done","field":"typed","attr":"slog","pair":"plain","group":{"inside":1,"dynamic":"resolved"},"tail":true}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestReplacerCanModifyBuiltInFields(t *testing.T) {
	replacer := func(_ context.Context, _ []string, field Field) Field {
		switch field.Key {
		case LevelKey:
			return String("severity", "notice")
		case MessageKey:
			return String("message", "changed")
		case NameKey:
			return String("name", "worker")
		default:
			return field
		}
	}

	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Json(&HandlerOptions{Name: "server", Replacer: replacer})).InfoS("ready", "id", 1)
		want := `{"severity":"notice","message":"changed","name":"worker","id":1}` + "\n"
		if got := buf.String(); got != want {
			t.Fatalf("json output = %q, want %q", got, want)
		}
	})

	t.Run("Text", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Text(&HandlerOptions{Name: "server", Replacer: replacer})).InfoS("ready", "id", 1)
		want := "severity=notice message=changed name=worker id=1\n"
		if got := buf.String(); got != want {
			t.Fatalf("text output = %q, want %q", got, want)
		}
	})
}

func TestReplacerCanDeleteBuiltInFields(t *testing.T) {
	replacer := func(_ context.Context, _ []string, field Field) Field {
		switch field.Key {
		case LevelKey, MessageKey, NameKey:
			return Field{}
		default:
			return field
		}
	}

	var jsonBuf, textBuf bytes.Buffer
	New(&jsonBuf, Json(&HandlerOptions{Name: "server", Replacer: replacer})).InfoS("ready", "id", 1)
	New(&textBuf, Text(&HandlerOptions{Name: "server", Replacer: replacer})).InfoS("ready", "id", 1)
	if got, want := jsonBuf.String(), `{"id":1}`+"\n"; got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
	if got, want := textBuf.String(), "id=1\n"; got != want {
		t.Fatalf("text output = %q, want %q", got, want)
	}
}

func TestLoggerNameAllowsDuplicateLoggerField(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Json(&HandlerOptions{Name: "server"})).InfoS("ready", NameKey, "worker")
	want := `{"level":"INFO","msg":"ready","logger":"server","logger":"worker"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}
}

func TestReplacerCanModifyTextLoggerPrefix(t *testing.T) {
	var buf bytes.Buffer
	replacer := func(_ context.Context, _ []string, field Field) Field {
		if field.Key == NameKey {
			return String(NameKey, "worker")
		}
		return field
	}
	New(&buf, Text(&HandlerOptions{Name: "server", Replacer: replacer})).InfoS("ready")
	want := "[worker] INFO msg=ready\n"
	if got := buf.String(); got != want {
		t.Fatalf("text output = %q, want %q", got, want)
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
