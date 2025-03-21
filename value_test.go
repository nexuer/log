package log

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		input Kind
		want  string
	}{
		{
			input: KindAny,
			want:  "Any",
		},
		{
			input: KindBool,
			want:  "Bool",
		},
		{
			input: KindDuration,
			want:  "Duration",
		},
		{
			input: KindFloat64,
			want:  "Float64",
		},
		{
			input: KindInt64,
			want:  "Int64",
		},

		{
			input: KindString,
			want:  "String",
		},
		{
			input: KindTime,
			want:  "Time",
		},
		{
			input: KindUint64,
			want:  "Uint64",
		},
		{
			input: KindGroup,
			want:  "Group",
		},

		{
			input: KindValuer,
			want:  "Valuer",
		},
	}

	for i, tt := range tests {
		if got := tt.input.String(); got != tt.want {
			t.Errorf("#%d Level.String() = %v, want %v", i, got, tt.want)
		}
	}
}

var testTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

func TestValueEqual(t *testing.T) {
	var x, y int

	vals := []Value{
		{},
		Int64Value(1),
		Int64Value(2),
		Float64Value(3.5),
		Float64Value(3.7),
		BoolValue(true),
		BoolValue(false),
		TimeValue(testTime),
		TimeValue(time.Time{}),
		TimeValue(time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC)),
		TimeValue(time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC)),            // overflows nanoseconds
		TimeValue(time.Date(1715, 6, 13, 0, 25, 26, 290448384, time.UTC)), // overflowed value
		AnyValue(&x),
		AnyValue(&y),
		GroupValue(Bool("b", true), Int("i", 3)),
		GroupValue(Bool("b", true), Int("i", 4)),
		GroupValue(Bool("b", true), Int("j", 4)),
		DurationValue(3 * time.Second),
		DurationValue(2 * time.Second),
		StringValue("foo"),
		StringValue("fuu"),
	}
	for i, v1 := range vals {
		for j, v2 := range vals {
			got := v1.Equal(v2)
			want := i == j
			if got != want {
				t.Errorf("%v.Equal(%v): got %t, want %t", v1, v2, got, want)
			}
		}
	}
}

func TestValueString(t *testing.T) {
	for _, test := range []struct {
		v    Value
		want string
	}{
		{Int64Value(-3), "-3"},
		{Uint64Value(1), "1"},
		{Float64Value(.15), "0.15"},
		{BoolValue(true), "true"},
		{StringValue("foo"), "foo"},
		{TimeValue(testTime), "2000-01-02 03:04:05 +0000 UTC"},
		{AnyValue(time.Duration(3 * time.Second)), "3s"},
		{ValuerValue(Caller(0)), "<Valuer>"},
		{ValuerValue(Timestamp(time.RFC3339)), "<Valuer>"},
		{GroupValue(Int("a", 1), Bool("b", true)), "[a=1 b=true]"},
	} {
		if got := test.v.String(); got != test.want {
			t.Errorf("%#v:\ngot  %q\nwant %q", test.v, got, test.want)
		}
	}
}

func TestAnyValue(t *testing.T) {
	for _, test := range []struct {
		in   any
		want Value
	}{
		{1, IntValue(1)},
		{1.5, Float64Value(1.5)},
		{float32(2.5), Float64Value(2.5)},
		{"s", StringValue("s")},
		{true, BoolValue(true)},
		{testTime, TimeValue(testTime)},
		{time.Hour, DurationValue(time.Hour)},
		{[]Field{Int("i", 3)}, GroupValue(Int("i", 3))},
		{IntValue(4), IntValue(4)},
		{uint(2), Uint64Value(2)},
		{uint8(3), Uint64Value(3)},
		{uint16(4), Uint64Value(4)},
		{uint32(5), Uint64Value(5)},
		{uint64(6), Uint64Value(6)},
		{uintptr(7), Uint64Value(7)},
		{int8(8), Int64Value(8)},
		{int16(9), Int64Value(9)},
		{int32(10), Int64Value(10)},
		{int64(11), Int64Value(11)},
	} {
		got := AnyValue(test.in)
		if !got.Equal(test.want) {
			t.Errorf("%v (%[1]T): got %v (kind %s), want %v (kind %s)",
				test.in, got, got.Kind(), test.want, test.want.Kind())
		}
	}
}

func TestValueAny(t *testing.T) {
	for _, want := range []any{
		nil,
		LevelDebug + 100,
		time.UTC, // time.Locations treated specially...
		KindBool, // ...as are Kinds
		[]Field{Int("a", 1)},
		int64(2),
		uint64(3),
		true,
		time.Minute,
		time.Time{},
		3.14,
		"foo",
	} {
		v := AnyValue(want)
		got := v.Any()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

type replace struct {
	v Value
}

func replacedValuer(value Value) Valuer {
	return func(ctx context.Context) Value {
		return value
	}
}

func panickingValuer() Valuer {
	return func(ctx context.Context) Value {
		panic("panicking")
	}
}

func fieldsEqual(as1, as2 []Field) bool {
	return slices.EqualFunc(as1, as2, Field.Equal)
}

func TestValuer(t *testing.T) {
	want := "replaced"
	r := &replace{v: StringValue(want)}
	v := AnyValue(replacedValuer(r.v))
	if g, w := v.Kind(), KindValuer; g != w {
		t.Errorf("got %s, want %s", g, w)
	}
	got := v.Valuer()(context.Background()).Any()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	//
	// Test Resolve.
	got = v.Resolve(context.Background()).Any()
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	//
	// Test Resolve max iteration.
	//r.v = AnyValue(replacedValuer(r.v)) // create a cycle
	//got = AnyValue(replacedValuer(r.v)).Resolve(context.Background()).Any()
	//if _, ok := got.(error); !ok {
	//	t.Errorf("expected error, got %T", got)
	//}
	//
	// Groups are not recursively resolved.
	c := Any("c", StringValue("d"))
	v = AnyValue(replacedValuer(GroupValue(Int("a", 1), Group("b", c))))
	got2 := v.Resolve(context.Background()).Any().([]Field)
	want2 := []Field{Int("a", 1), Group("b", c)}
	if !fieldsEqual(got2, want2) {
		t.Errorf("got %v, want %v", got2, want2)
	}
	//
	//// Verify that panics in Resolve are caught and turn into errors.
	v = AnyValue(panickingValuer())
	got = v.Resolve(context.Background()).Any()
	gotErr, ok := got.(error)
	if !ok {
		t.Errorf("expected error, got %T", got)
	}
	// The error should provide some context information.
	// We'll just check that this function name appears in it.
	if got, want := gotErr.Error(), "valuer panicked"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

// A Value with "unsafe" strings is significantly faster:
// safe:  1785 ns/op, 0 allocs
// unsafe: 690 ns/op, 0 allocs

// Run this with and without -tags unsafe_kvs to compare.
func BenchmarkUnsafeStrings(b *testing.B) {
	b.ReportAllocs()
	dst := make([]Value, 100)
	src := make([]Value, len(dst))
	b.Logf("Value size = %d", unsafe.Sizeof(Value{}))
	for i := range src {
		src[i] = StringValue(fmt.Sprintf("string#%d", i))
	}
	b.ResetTimer()
	var d string
	for i := 0; i < b.N; i++ {
		copy(dst, src)
		for _, a := range dst {
			d = a.String()
		}
	}
	_ = d
}

func BenchmarkSlogUnsafeStrings(b *testing.B) {
	b.ReportAllocs()
	dst := make([]slog.Value, 100)
	src := make([]slog.Value, len(dst))
	b.Logf("Value size = %d", unsafe.Sizeof(slog.Value{}))
	for i := range src {
		src[i] = slog.StringValue(fmt.Sprintf("string#%d", i))
	}
	b.ResetTimer()
	var d string
	for i := 0; i < b.N; i++ {
		copy(dst, src)
		for _, a := range dst {
			d = a.String()
		}
	}
	_ = d
}
