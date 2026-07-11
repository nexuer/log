package log

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDefaultFieldsAreUsedAsReadOnlyTemplate(t *testing.T) {
	beforeKeys := []string{DefaultFields[0].Key, DefaultFields[1].Key}
	beforeValuers := []uintptr{
		reflectValuerPointer(DefaultFields[0].Value.Valuer()),
		reflectValuerPointer(DefaultFields[1].Value.Valuer()),
	}
	var buf bytes.Buffer
	New(&buf, Json()).WithFields(DefaultFields...).InfoS("done")

	if len(DefaultFields) != 2 || DefaultFields[0].Key != beforeKeys[0] || DefaultFields[1].Key != beforeKeys[1] ||
		reflectValuerPointer(DefaultFields[0].Value.Valuer()) != beforeValuers[0] ||
		reflectValuerPointer(DefaultFields[1].Value.Valuer()) != beforeValuers[1] {
		t.Fatalf("DefaultFields changed: %#v", DefaultFields)
	}
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if _, ok := record["ts"]; !ok {
		t.Fatal("default timestamp field is missing")
	}
	if _, ok := record["caller"]; !ok {
		t.Fatal("default caller field is missing")
	}
}

func TestGlobalLoggerCaller(t *testing.T) {
	old := defaultLogger.Load()
	defer defaultLogger.Store(old)
	var buf bytes.Buffer
	SetDefault(New(&buf, Json()).WithFields(DefaultFields...))
	Info("caller")
	var record struct {
		Caller Source `json:"caller"`
	}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(record.Caller.Function, ".TestGlobalLoggerCaller") {
		t.Fatalf("caller function = %q, want TestGlobalLoggerCaller", record.Caller.Function)
	}
}

func reflectValuerPointer(value any) uintptr {
	return reflect.ValueOf(value).Pointer()
}

func TestTimestampValuePreservesFormattingAndEscaping(t *testing.T) {
	timestamp := time.Date(2026, time.July, 10, 21, 30, 45, 0, time.FixedZone("CST", 8*60*60))
	layout := "2006-01-02 \"quoted\"\nZ07:00"
	spec := &timestampSpec{layout: layout, location: timestamp.Location()}
	valuer := Valuer(func(context.Context) Value {
		return timestampStringValue(timestamp, spec)
	})
	want := timestamp.Format(layout)

	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Json()).InfoS("done", "ts", valuer)
		var record map[string]any
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if got := record["ts"]; got != want {
			t.Fatalf("timestamp = %q, want %q", got, want)
		}
	})

	t.Run("Text", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Text()).InfoS("done", "ts", valuer)
		wantLine := "INFO msg=done ts=" + strconv.Quote(want) + "\n"
		if got := buf.String(); got != wantLine {
			t.Fatalf("output = %q, want %q", got, wantLine)
		}
	})

	if got := valuer(context.Background()).String(); got != want {
		t.Fatalf("Value.String() = %q, want %q", got, want)
	}
}

func TestInvalidTimeRemainsValidJSON(t *testing.T) {
	tests := []struct {
		name  string
		value time.Time
	}{
		{name: "year", value: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
		{name: "zone", value: time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("invalid", 24*60*60))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			New(&buf, Json()).InfoS("done", "value", test.value)
			if !json.Valid(buf.Bytes()) {
				t.Fatalf("invalid JSON: %q", buf.String())
			}
			var record map[string]any
			if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if got, ok := record["value"].(string); !ok || !strings.HasPrefix(got, "!ERROR:") {
				t.Fatalf("value = %#v, want encoded error", record["value"])
			}
		})
	}
}

//go:noinline
func callerValueAtTestSite(full bool) Value {
	return Caller(1, full)(context.Background())
}

func TestCallerValuePreservesSource(t *testing.T) {
	for _, full := range []bool{false, true} {
		value := callerValueAtTestSite(full)
		source := value.Source()
		if !strings.HasSuffix(source.Function, ".callerValueAtTestSite") {
			t.Fatalf("function = %q", source.Function)
		}
		if !strings.HasSuffix(source.File, "/metadata_test.go") {
			t.Fatalf("file = %q", source.File)
		}
		if full && !strings.HasPrefix(source.File, "/") {
			t.Fatalf("full file = %q, want absolute path", source.File)
		}
		if !full && strings.Count(source.File, "/") > 1 {
			t.Fatalf("short file = %q, want at most two path components", source.File)
		}
		if source.Line == 0 {
			t.Fatal("line = 0")
		}
	}
}

func TestCallerValueEncoding(t *testing.T) {
	value := callerValueAtTestSite(false)
	source := value.Source()
	valuer := Valuer(func(context.Context) Value { return value })

	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Json()).InfoS("done", "caller", valuer)
		var record struct {
			Caller Source `json:"caller"`
		}
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Caller != *source {
			t.Fatalf("caller = %#v, want %#v", record.Caller, *source)
		}
	})

	t.Run("Text", func(t *testing.T) {
		var buf bytes.Buffer
		New(&buf, Text()).InfoS("done", "caller", valuer)
		want := "INFO msg=done caller=" + source.File + ":" + strconv.Itoa(source.Line) + "\n"
		if got := buf.String(); got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})
}

//go:noinline
func callerDepthLeaf(ctx context.Context) Value {
	return Caller(1)(ctx)
}

//go:noinline
func callerDepthInner(ctx context.Context) Value {
	return callerDepthLeaf(ctx)
}

//go:noinline
func callerDepthOuter(ctx context.Context) Value {
	return callerDepthInner(ctx)
}

func TestAddCallerDepthAccumulatesWrappers(t *testing.T) {
	ctx := AddCallerDepth(context.Background(), 1)
	ctx = AddCallerDepth(ctx, 1)
	if got := callerDepth(ctx); got != 2 {
		t.Fatalf("caller depth = %d, want 2", got)
	}
	source := callerDepthOuter(ctx).Source()
	if !strings.HasSuffix(source.Function, ".callerDepthOuter") {
		t.Fatalf("function = %q, want callerDepthOuter", source.Function)
	}
}
