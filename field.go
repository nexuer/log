package log

import (
	"time"
)

// Reference: https://github.com/golang/go/blob/master/src/log/slog/attr.go

type Field struct {
	Key   string
	Value Value
}

// String returns an Attr for a string value.
func String(key, value string) Field {
	return Field{key, StringValue(value)}
}

// Int converts an int to an int64 and returns
// an Attr with that value.
func Int(key string, value int) Field {
	return Int64(key, int64(value))
}

// Int64 returns a Field for an int64.
func Int64(key string, value int64) Field {
	return Field{key, Int64Value(value)}
}

// Uint64 returns a Field for a uint64.
func Uint64(key string, v uint64) Field {
	return Field{key, Uint64Value(v)}
}

// Float64 returns a Field for a floating-point number.
func Float64(key string, v float64) Field {
	return Field{key, Float64Value(v)}
}

// Bool returns an Attr for a bool.
func Bool(key string, v bool) Field {
	return Field{key, BoolValue(v)}
}

// Time returns an Attr for a [time.Time].
// It discards the monotonic portion.
func Time(key string, v time.Time) Field {
	return Field{key, TimeValue(v)}
}

// Duration returns an Attr for a [time.Duration].
func Duration(key string, v time.Duration) Field {
	return Field{key, DurationValue(v)}
}

// Group returns an Attr for a Group [Value].
// The first argument is the key; the remaining arguments
// are converted to Attrs as in [Logger.Log].
//
// Use Group to collect several key-value pairs under a single
// key on a log line, or as the result of LogValue
// in order to log a single value as multiple Attrs.
func Group(key string, args ...any) Field {
	fields, _ := fieldsToAttrSlice(args)
	return Field{key, GroupValue(fields...)}
}

// Any returns an Attr for the supplied value.
// See [AnyValue] for how values are treated.
func Any(key string, value any) Field {
	return Field{key, AnyValue(value)}
}

const badKey = "<BAD_KEY>"

func fieldsToAttrSlice(args []any) ([]Field, bool) {
	var (
		field  Field
		fields []Field
	)
	hasDynamic := false
	for len(args) > 0 {
		field, args = fieldsToAttr(args)
		if field.Value.Kind() == KindValuer {
			if !hasDynamic {
				hasDynamic = true
			}
		}
		fields = append(fields, field)
	}
	return fields, hasDynamic
}

// fieldsToAttr turns a prefix of the nonempty args slice into an Attr
// and returns the unconsumed portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func fieldsToAttr(args []any) (Field, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return String(badKey, x), nil
		}
		return Any(x, args[1]), args[2:]

	case Field:
		return x, args[1:]

	default:
		return Any(badKey, x), args[1:]
	}
}

// Equal reports whether a and b have equal keys and values.
func (f Field) Equal(b Field) bool {
	return f.Key == b.Key && f.Value.Equal(b.Value)
}

func (f Field) String() string {
	return f.Key + "=" + f.Value.String()
}

// isEmpty reports whether a has an empty key and a nil value.
// That can be written as Attr{} or Any("", nil).
func (f Field) isEmpty() bool {
	return f.Key == "" && f.Value.num == 0 && f.Value.any == nil
}
