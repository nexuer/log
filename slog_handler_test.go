package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSlogJSONHandlerMatchesNativeOutput(t *testing.T) {
	var native bytes.Buffer
	New(&native, Json()).WarnS("done", "id", 1, "ok", true)

	var adapted bytes.Buffer
	logger := slog.New(NewSlogHandler(New(&adapted, Json())))
	logger.LogAttrs(context.Background(), slog.LevelWarn, "done", slog.Int("id", 1), slog.Bool("ok", true))

	if got, want := adapted.String(), native.String(); got != want {
		t.Fatalf("slog JSON output = %q, want native output %q", got, want)
	}
	if strings.Contains(adapted.String(), `"time"`) || strings.Contains(adapted.String(), `"source"`) {
		t.Fatalf("slog JSON output contains time or source: %q", adapted.String())
	}
}

func TestSlogTextHandlerMatchesNativeOutput(t *testing.T) {
	var native bytes.Buffer
	New(&native, Text()).WarnS("done", "id", 1, "ok", true)

	var adapted bytes.Buffer
	logger := slog.New(NewSlogHandler(New(&adapted, Text())))
	logger.LogAttrs(context.Background(), slog.LevelWarn, "done", slog.Int("id", 1), slog.Bool("ok", true))

	if got, want := adapted.String(), native.String(); got != want {
		t.Fatalf("slog text output = %q, want native output %q", got, want)
	}
}

func TestSlogHandlerIgnoresRecordTimeAndPC(t *testing.T) {
	var buf bytes.Buffer
	handler := NewSlogHandler(New(&buf, Json()))
	record := slog.NewRecord(time.Unix(123, 456), slog.LevelInfo, "done", 12345)
	record.AddAttrs(slog.String("id", "req-1"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	want := `{"level":"INFO","msg":"done","id":"req-1"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerLevel(t *testing.T) {
	handler := NewSlogHandler(New(io.Discard, Json()).SetLevel(LevelWarn))
	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Info unexpectedly enabled")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("Warn unexpectedly disabled")
	}
}

func TestSlogHandlerPreservesLoggerState(t *testing.T) {
	var buf bytes.Buffer
	native := New(&buf, Json(&HandlerOptions{Name: "server"})).
		SetLevel(LevelWarn).
		With("service", "api").
		WithGroup("request")
	logger := slog.New(NewSlogHandler(native))

	logger.Info("ignored")
	logger.Warn("done", "id", "req-1")

	want := `{"logger":"server","level":"WARN","service":"api","msg":"done","request":{"id":"req-1"}}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestNewSlogHandlerIsSnapshot(t *testing.T) {
	var before, after bytes.Buffer
	native := New(&before, Json()).SetLevel(LevelInfo)
	logger := slog.New(NewSlogHandler(native))

	native.SetOutput(&after).SetLevel(LevelError).SetHandler(Text())
	logger.Info("snapshot")

	want := `{"level":"INFO","msg":"snapshot"}` + "\n"
	if got := before.String(); got != want {
		t.Fatalf("snapshot output = %q, want %q", got, want)
	}
	if got := after.String(); got != "" {
		t.Fatalf("updated Logger output = %q, want empty", got)
	}
}

func TestSlogHandlerMergesCallerDepth(t *testing.T) {
	var buf bytes.Buffer
	depthValue := Valuer(func(ctx context.Context) Value {
		return IntValue(callerDepth(ctx))
	})
	native := New(&buf, Json()).
		WithContext(AddCallerDepth(context.Background(), 2)).
		With("depth", depthValue)
	logger := slog.New(NewSlogHandler(native))

	logger.Info("native-depth")
	logger.LogAttrs(AddCallerDepth(context.Background(), 3), slog.LevelInfo, "merged-depth")

	want := "" +
		`{"level":"INFO","depth":0,"msg":"native-depth"}` + "\n" +
		`{"level":"INFO","depth":3,"msg":"merged-depth"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSlogHandler(New(&buf, Json()).WithFields(DefaultFields...)))
	_, _, line, _ := runtime.Caller(0)
	logger.Info("caller")
	var record struct {
		Caller Source `json:"caller"`
	}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(record.Caller.Function, ".TestSlogHandlerCaller") {
		t.Fatalf("caller function = %q, want TestSlogHandlerCaller", record.Caller.Function)
	}
	if record.Caller.Line != line+1 {
		t.Fatalf("caller line = %d, want %d", record.Caller.Line, line+1)
	}
}

func TestNewSlogHandlerNilUsesDefaultCaller(t *testing.T) {
	old := defaultLogger.Load()
	defer defaultLogger.Store(old)

	var buf bytes.Buffer
	SetDefault(New(&buf, Json()).WithFields(DefaultFields...))
	slog.New(NewSlogHandler(nil)).Info("caller")

	var record struct {
		Caller Source `json:"caller"`
	}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(record.Caller.Function, ".TestNewSlogHandlerNilUsesDefaultCaller") {
		t.Fatalf("caller function = %q, want TestNewSlogHandlerNilUsesDefaultCaller", record.Caller.Function)
	}
}

type wrappedHandler struct {
	Handler
}

func (h wrappedHandler) WithFields(ctx context.Context, fields ...Field) Handler {
	return wrappedHandler{Handler: h.Handler.WithFields(ctx, fields...)}
}

func (h wrappedHandler) WithGroup(name string) Handler {
	return wrappedHandler{Handler: h.Handler.WithGroup(name)}
}

func TestSlogHandlerAdaptsCustomHandler(t *testing.T) {
	var buf bytes.Buffer
	native := New(&buf, wrappedHandler{Handler: Json()}).With("service", "api")
	logger := slog.New(NewSlogHandler(native)).WithGroup("request")
	logger.Info("done", "id", 42)

	want := `{"level":"INFO","service":"api","msg":"done","request":{"id":42}}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCustomSlogHandlerResolvesWithAttrsLogValuerLazily(t *testing.T) {
	var buf bytes.Buffer
	valuer := new(countingSlogValuer)
	logger := slog.New(NewSlogHandler(New(&buf, wrappedHandler{Handler: Json()}))).
		With("dynamic", valuer)
	if got := valuer.Calls(); got != 0 {
		t.Fatalf("LogValuer calls after With = %d, want 0", got)
	}

	logger.Info("first")
	logger.Info("second")
	if got := valuer.Calls(); got != 2 {
		t.Fatalf("LogValuer calls = %d, want 2", got)
	}
	want := "" +
		`{"level":"INFO","dynamic":{"count":1},"msg":"first"}` + "\n" +
		`{"level":"INFO","dynamic":{"count":2},"msg":"second"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerWithAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSlogHandler(New(&buf, Json()))).
		With("service", "api").
		WithGroup("request").
		With("id", "req-1").
		WithGroup("user")
	logger.LogAttrs(context.Background(), slog.LevelInfo, "login", slog.Int("id", 42))

	want := `{"level":"INFO","service":"api","request":{"id":"req-1","user":{"id":42}},"msg":"login"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

type countingSlogValuer struct {
	mu    sync.Mutex
	calls int
}

func (v *countingSlogValuer) LogValue() slog.Value {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	return slog.GroupValue(slog.Int("count", v.calls))
}

func (v *countingSlogValuer) Calls() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls
}

func TestSlogHandlerResolvesWithAttrsLogValuerLazily(t *testing.T) {
	var buf bytes.Buffer
	valuer := new(countingSlogValuer)
	logger := slog.New(NewSlogHandler(New(&buf, Json()))).With("dynamic", valuer)
	if got := valuer.Calls(); got != 0 {
		t.Fatalf("LogValuer calls after With = %d, want 0", got)
	}

	logger.Info("first")
	logger.Info("second")
	if got := valuer.Calls(); got != 2 {
		t.Fatalf("LogValuer calls = %d, want 2", got)
	}
	want := "" +
		`{"level":"INFO","dynamic":{"count":1},"msg":"first"}` + "\n" +
		`{"level":"INFO","dynamic":{"count":2},"msg":"second"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerLazyPathPreservesPriorAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	valuer := new(countingSlogValuer)
	logger := slog.New(NewSlogHandler(New(&buf, Json()))).
		With("service", "api").
		WithGroup("request").
		With("dynamic", valuer).
		WithGroup("user")
	logger.Info("done", "id", 42)

	want := `{"level":"INFO","service":"api","request":{"dynamic":{"count":1},"user":{"id":42}},"msg":"done"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerLazyPathContinuesNativeGroups(t *testing.T) {
	var buf bytes.Buffer
	valuer := new(countingSlogValuer)
	native := New(&buf, Json()).
		With("service", "api").
		WithGroup("request").
		With("request_id", "req-1")
	logger := slog.New(NewSlogHandler(native)).
		With("dynamic", valuer).
		WithGroup("user")
	logger.Info("done", "id", 42)

	want := `{"level":"INFO","service":"api","request":{"request_id":"req-1","dynamic":{"count":1},"user":{"id":42}},"msg":"done"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerReplacerCanDeleteBuiltIns(t *testing.T) {
	var buf bytes.Buffer
	var calls []string
	handler := NewSlogHandler(New(&buf, Json(&HandlerOptions{
		Replacer: func(_ context.Context, groups []string, field Field) Field {
			calls = append(calls, strings.Join(groups, ".")+":"+field.Key)
			switch field.Key {
			case slog.LevelKey, slog.MessageKey:
				return Field{}
			case "secret":
				return String("secret", "[redacted]")
			default:
				return field
			}
		},
	})))

	logger := slog.New(handler).WithGroup("request")
	logger.Info("done", "secret", "token", "status", 200)

	want := `{"request":{"secret":"[redacted]","status":200}}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	wantCalls := []string{":level", ":msg", "request:secret", "request:status"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("Replacer calls = %#v, want %#v", calls, wantCalls)
	}
	for i := range calls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("Replacer calls = %#v, want %#v", calls, wantCalls)
		}
	}
}

func TestSlogHandlerOmitsWithGroupEmptiedByReplacer(t *testing.T) {
	var buf bytes.Buffer
	replacer := func(_ context.Context, _ []string, field Field) Field {
		if field.Key == "secret" {
			return Field{}
		}
		return field
	}
	logger := slog.New(NewSlogHandler(New(&buf, Json(&HandlerOptions{Replacer: replacer})))).
		With(slog.Group("credentials", slog.String("secret", "token")))
	logger.Info("done", "id", 1)
	want := `{"level":"INFO","msg":"done","id":1}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerNameWithLazyAttrs(t *testing.T) {
	var buf bytes.Buffer
	valuer := new(countingSlogValuer)
	logger := slog.New(NewSlogHandler(New(&buf, Json(&HandlerOptions{Name: "server"})))).
		With("service", "api").
		With("dynamic", valuer)
	logger.Info("done", "id", 42)

	want := `{"logger":"server","level":"INFO","service":"api","dynamic":{"count":1},"msg":"done","id":42}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerOwnsWithAttrsSlice(t *testing.T) {
	var buf bytes.Buffer
	attrs := []slog.Attr{slog.String("service", "api")}
	handler := NewSlogHandler(New(&buf, Json())).WithAttrs(attrs)
	attrs[0] = slog.String("service", "mutated")
	if err := handler.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "done", 0)); err != nil {
		t.Fatal(err)
	}

	want := `{"level":"INFO","service":"api","msg":"done"}` + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSlogHandlerConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSlogHandler(New(&buf, Json())))
	const count = 100
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			logger.Info("done", "id", i)
		}(i)
	}
	wg.Wait()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{'\n'})
	if len(lines) != count {
		t.Fatalf("line count = %d, want %d", len(lines), count)
	}
	for _, line := range lines {
		if !json.Valid(line) {
			t.Fatalf("invalid JSON line: %q", line)
		}
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestSlogHandlerReturnsWriterError(t *testing.T) {
	want := errors.New("write failed")
	handler := NewSlogHandler(New(errorWriter{err: want}, Json()))
	err := handler.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "done", 0))
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestJSONMarshalErrorRollsBackTemporaryOutput(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Json()).InfoS("done", "bad", make(chan int), "ok", true)
	line := bytes.TrimSpace(buf.Bytes())
	if !json.Valid(line) {
		t.Fatalf("invalid JSON after marshal error: %q", line)
	}
}

func TestJSONFastSlicesMatchEncoder(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"ints", []int{1, -2, 3}},
		{"nil ints", []int(nil)},
		{"empty ints", []int{}},
		{"strings", []string{"plain", "<html>", "line\nquote\""}},
		{"nil strings", []string(nil)},
		{"times", []time.Time{time.Unix(0, 0).UTC(), time.Unix(1, 2).UTC()}},
		{"nil times", []time.Time(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			New(&output, Json()).InfoS("done", "value", tt.value)
			var record map[string]json.RawMessage
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}

			var expected bytes.Buffer
			encoder := json.NewEncoder(&expected)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(tt.value); err != nil {
				t.Fatal(err)
			}
			want := bytes.TrimSpace(expected.Bytes())
			if got := record["value"]; !bytes.Equal(got, want) {
				t.Fatalf("encoded value = %s, want %s", got, want)
			}
		})
	}
}
