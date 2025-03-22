package log

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Reference: https://github.com/golang/go/blob/master/src/log/slog/value.go

// A Value can represent any Go value, but unlike type any,
// it can represent most small values without an allocation.
// The zero Value corresponds to nil.
type Value struct {
	_ [0]func() // disallow ==
	// num holds the value for Kinds Int64, Uint64, Float64, Bool and Duration,
	// the string length for KindString, and nanoseconds since the epoch for KindTime.
	num uint64
	// If any is of type Kind, then the value is in num as described above.
	// If any is of type *time.Location, then the Kind is Time and time.Time value
	// can be constructed from the Unix nanos in num and the location (monotonic time
	// is not preserved).
	// If any is of type stringptr, then the Kind is String and the string value
	// consists of the length in num and the pointer in any.
	// Otherwise, the Kind is Any and any is the value.
	// (This implies that Attrs cannot store values of type Kind, *time.Location
	// or stringptr.)
	any any

	// custom type
	kind Kind
}

type (
	stringptr *byte  // used in Value.any when the Value is a string
	groupptr  *Field // used in Value.any when the Value is a []Attr
)

// Kind is the kind of a [Value].
type Kind int

// The following list is sorted alphabetically, but it's also important that
// KindAny is 0 so that a zero Value represents nil.

const (
	KindAny Kind = iota
	KindBool
	KindDuration
	KindFloat64
	KindInt64
	KindString
	KindTime
	KindUint64
	KindGroup

	// KindValuer Use KindValuer instead of slog.kindLogValuer
	KindValuer
	KindSource
)

var kindStrings = []string{
	"Any",
	"Bool",
	"Duration",
	"Float64",
	"Int64",
	"String",
	"Time",
	"Uint64",
	"Group",
	"Valuer",
	"Source",
}

func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindStrings) {
		return kindStrings[k]
	}
	return "<unknown log.Kind>"
}

// Kind returns v's Kind.
func (v Value) Kind() Kind {
	return v.kind
}

//////////////// Constructors

// StringValue returns a new [Value] for a string.
func StringValue(value string) Value {
	return Value{kind: KindString, num: uint64(len(value)), any: stringptr(unsafe.StringData(value))}
}

// IntValue returns a [Value] for an int.
func IntValue(v int) Value {
	return Int64Value(int64(v))
}

// Int64Value returns a [Value] for an int64.
func Int64Value(v int64) Value {
	return Value{kind: KindInt64, num: uint64(v)}
}

// UintValue returns a [Value] for an uint.
func UintValue(v uint) Value {
	return Uint64Value(uint64(v))
}

// Uint64Value returns a [Value] for a uint64.
func Uint64Value(v uint64) Value {
	return Value{kind: KindUint64, num: v}
}

// Float64Value returns a [Value] for a floating-point number.
func Float64Value(v float64) Value {
	return Value{kind: KindFloat64, num: math.Float64bits(v)}
}

// BoolValue returns a [Value] for a bool.
func BoolValue(v bool) Value {
	u := uint64(0)
	if v {
		u = 1
	}
	return Value{kind: KindBool, num: u}
}

type (
	// Unexported version of *time.Location, just so we can store *time.Locations in
	// Values. (No user-provided value has this type.)
	timeLocation *time.Location

	// timeTime is for times where UnixNano is undefined.
	timeTime time.Time
)

// TimeValue returns a [Value] for a [time.Time].
// It discards the monotonic portion.
func TimeValue(v time.Time) Value {
	if v.IsZero() {
		// UnixNano on the zero time is undefined, so represent the zero time
		// with a nil *time.Location instead. time.Time.Location method never
		// returns nil, so a Value with any == timeLocation(nil) cannot be
		// mistaken for any other Value, time.Time or otherwise.
		return Value{kind: KindTime, any: timeLocation(nil)}
	}
	nsec := v.UnixNano()
	t := time.Unix(0, nsec)
	if v.Equal(t) {
		// UnixNano correctly represents the time, so use a zero-alloc representation.
		return Value{kind: KindTime, num: uint64(nsec), any: timeLocation(v.Location())}
	}
	// Fall back to the general form.
	// Strip the monotonic portion to match the other representation.
	return Value{kind: KindTime, any: timeTime(v.Round(0))}
}

// DurationValue returns a [Value] for a [time.Duration].
func DurationValue(v time.Duration) Value {
	return Value{kind: KindDuration, num: uint64(v.Nanoseconds())}
}

// GroupValue returns a new [Value] for a list of Attrs.
// The caller must not subsequently mutate the argument slice.
func GroupValue(as ...Field) Value {
	// Remove empty groups.
	// It is simpler overall to do this at construction than
	// to check each Group recursively for emptiness.
	if n := countEmptyGroups(as); n > 0 {
		as2 := make([]Field, 0, len(as)-n)
		for _, a := range as {
			if !a.Value.isEmptyGroup() {
				as2 = append(as2, a)
			}
		}
		as = as2
	}
	return Value{kind: KindGroup, num: uint64(len(as)), any: groupptr(unsafe.SliceData(as))}
}

type Source struct {
	// Function is the package path-qualified function name containing the
	// source line. If non-empty, this string uniquely identifies a single
	// function in the program. This may be the empty string if not known.
	Function string `json:"function"`
	// File and Line are the file name and line number (1-based) of the source
	// line. These may be the empty string and zero, respectively, if not known.
	File string `json:"file"`
	Line int    `json:"line"`
}

// SourceValue returns a [Value] for a [Source].
func SourceValue(v *Source) Value {
	return Value{kind: KindSource, any: v}
}

// countEmptyGroups returns the number of empty group values in its argument.
func countEmptyGroups(as []Field) int {
	n := 0
	for _, a := range as {
		if a.Value.isEmptyGroup() {
			n++
		}
	}
	return n
}

// AnyValue returns a [Value] for the supplied value.
//
// If the supplied value is of type Value, it is returned
// unmodified.
//
// Given a value of one of Go's predeclared string, bool, or
// (non-complex) numeric types, AnyValue returns a Value of kind
// [KindString], [KindBool], [KindUint64], [KindInt64], or [KindFloat64].
// The width of the original numeric type is not preserved.
//
// Given a [time.Time] or [time.Duration] value, AnyValue returns a Value of kind
// [KindTime] or [KindDuration]. The monotonic time is not preserved.
//
// For nil, or values of all other types, including named types whose
// underlying type is numeric, AnyValue returns a value of kind [KindAny].
func AnyValue(v any) Value {
	switch v := v.(type) {
	case string:
		return StringValue(v)
	case *Source:
		return SourceValue(v)
	case int:
		return IntValue(v)
	case uint:
		return UintValue(v)
	case int64:
		return Int64Value(v)
	case uint64:
		return Uint64Value(v)
	case bool:
		return BoolValue(v)
	case time.Duration:
		return DurationValue(v)
	case time.Time:
		return TimeValue(v)
	case uint8:
		return Uint64Value(uint64(v))
	case uint16:
		return Uint64Value(uint64(v))
	case uint32:
		return Uint64Value(uint64(v))
	case uintptr:
		return Uint64Value(uint64(v))
	case int8:
		return Int64Value(int64(v))
	case int16:
		return Int64Value(int64(v))
	case int32:
		return Int64Value(int64(v))
	case float64:
		return Float64Value(v)
	case float32:
		return Float64Value(float64(v))
	case []Field:
		return GroupValue(v...)
	case Value:
		return v
	case Valuer:
		return ValuerValue(v)
	default:
		return Value{kind: KindAny, any: v}
	}
}

//////////////// Accessors

// Any returns v's value as an any.
func (v Value) Any() any {
	switch v.Kind() {
	case KindAny:
		return v.any
	case KindValuer:
		return v.any
	case KindGroup:
		return v.group()
	case KindInt64:
		return int64(v.num)
	case KindUint64:
		return v.num
	case KindFloat64:
		return v.float()
	case KindString:
		return v.str()
	case KindBool:
		return v.bool()
	case KindDuration:
		return v.duration()
	case KindTime:
		return v.time()
	case KindSource:
		return v.any
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

// String returns Value's value as a string, formatted like [fmt.Sprint]. Unlike
// the methods Int64, Float64, and so on, which panic if v is of the
// wrong kind, String never panics.
func (v Value) String() string {
	if v.Kind() == KindString {
		return v.str()
	}
	var buf []byte
	return string(v.append(buf))
}

func (v Value) str() string {
	return unsafe.String(v.any.(stringptr), v.num)
}

// Int64 returns v's value as an int64. It panics
// if v is not a signed integer.
func (v Value) Int64() int64 {
	if v.Kind() != KindInt64 {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindInt64))
	}
	return int64(v.num)
}

// Uint64 returns v's value as a uint64. It panics
// if v is not an unsigned integer.
func (v Value) Uint64() uint64 {
	if v.Kind() != KindUint64 {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindUint64))
	}
	return v.num
}

// Bool returns v's value as a bool. It panics
// if v is not a bool.
func (v Value) Bool() bool {
	if v.Kind() != KindBool {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindBool))
	}
	return v.bool()
}

func (v Value) bool() bool {
	return v.num == 1
}

// Duration returns v's value as a [time.Duration]. It panics
// if v is not a time.Duration.
func (v Value) Duration() time.Duration {
	if v.Kind() != KindDuration {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindDuration))
	}
	return v.duration()
}

func (v Value) duration() time.Duration {
	return time.Duration(int64(v.num))
}

// Float64 returns v's value as a float64. It panics
// if v is not a float64.
func (v Value) Float64() float64 {
	if v.Kind() != KindFloat64 {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindFloat64))
	}

	return v.float()
}

func (v Value) float() float64 {
	return math.Float64frombits(v.num)
}

// Time returns v's value as a [time.Time]. It panics
// if v is not a time.Time.
func (v Value) Time() time.Time {
	if v.Kind() != KindTime {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindTime))
	}
	return v.time()
}

// See TimeValue to understand how times are represented.
func (v Value) time() time.Time {
	switch a := v.any.(type) {
	case timeLocation:
		if a == nil {
			return time.Time{}
		}
		return time.Unix(0, int64(v.num)).In(a)
	case timeTime:
		return time.Time(a)
	default:
		panic(fmt.Sprintf("bad time type %T", v.any))
	}
}

// Group returns v's value as a []Attr.
// It panics if v's [Kind] is not [KindGroup].
func (v Value) Group() []Field {
	if v.Kind() != KindGroup {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindGroup))
	}

	return v.group()
}

func (v Value) group() []Field {
	return unsafe.Slice(v.any.(groupptr), v.num)
}

// Valuer returns v's value as a LogValuer. It panics
// if v is not a LogValuer.
func (v Value) Valuer() Valuer {
	valuer, ok := v.any.(Valuer)
	if !ok {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindValuer))
	}
	return valuer
}

func (v Value) valuer() Valuer {
	return v.any.(Valuer)
}

func (v Value) Source() *Source {
	source, ok := v.any.(*Source)
	if !ok {
		panic(fmt.Sprintf("Value kind is %s, not %s", v.Kind(), KindSource))
	}
	return source
}

func (v Value) source() *Source {
	return v.any.(*Source)
}

//////////////// Other

// Equal reports whether v and w represent the same Go value.
func (v Value) Equal(w Value) bool {
	k1 := v.Kind()
	k2 := w.Kind()
	if k1 != k2 {
		return false
	}
	switch k1 {
	case KindInt64, KindUint64, KindBool, KindDuration:
		return v.num == w.num
	case KindString:
		return v.str() == w.str()
	case KindFloat64:
		return v.float() == w.float()
	case KindTime:
		return v.time().Equal(w.time())
	case KindAny:
		return v.any == w.any // may panic if non-comparable
	case KindValuer:
		return false
		//return v.any == w.any // must panic on function
	case KindGroup:
		return slices.EqualFunc(v.group(), w.group(), Field.Equal)
	default:
		panic(fmt.Sprintf("bad kind: %s", k1))
	}
}

// isEmptyGroup reports whether v is a group that has no attributes.
func (v Value) isEmptyGroup() bool {
	if v.Kind() != KindGroup {
		return false
	}
	// We do not need to recursively examine the group's Attrs for emptiness,
	// because GroupValue removed them when the group was constructed, and
	// groups are immutable.
	//return len(v.group()) == 0
	return v.num == 0
}

// append appends a text representation of v to dst.
// v is formatted as with fmt.Sprint.
func (v Value) append(dst []byte) []byte {
	switch v.Kind() {
	case KindString:
		return append(dst, v.str()...)
	case KindInt64:
		return strconv.AppendInt(dst, int64(v.num), 10)
	case KindUint64:
		return strconv.AppendUint(dst, v.num, 10)
	case KindFloat64:
		return strconv.AppendFloat(dst, v.float(), 'g', -1, 64)
	case KindBool:
		return strconv.AppendBool(dst, v.bool())
	case KindDuration:
		return append(dst, v.duration().String()...)
	case KindTime:
		return append(dst, v.time().String()...)
	case KindGroup:
		return fmt.Append(dst, v.group())
	case KindAny, KindValuer:
		return fmt.Append(dst, v.any)
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

const maxValuerValues = 100

// Resolve repeatedly calls Valuer on v while it implements [Valuer],
// and returns the result.
// If v resolves to a group, the group's attributes' values are not recursively
// resolved.
// If the number of Valuer calls exceeds a threshold, a Value containing an
// error is returned.
// Resolve's return value is guaranteed not to be of Kind [KindValuer].
func (v Value) Resolve(ctx context.Context) (rv Value) {
	orig := v
	defer func() {
		if r := recover(); r != nil {
			rv = AnyValue(fmt.Errorf("valuer panicked\n%s", stack(3, 5)))
		}
	}()

	for i := 0; i < maxValuerValues; i++ {
		if v.Kind() != KindValuer {
			return v
		}
		valuer := v.Valuer()
		if valuer == nil {
			return v
		}
		v = ResolveValuer(ctx, valuer)
	}
	err := fmt.Errorf("valuer called too many times on Value of type %T", orig.Any())
	return AnyValue(err)
}

func stack(skip, nFrames int) string {
	pcs := make([]uintptr, nFrames+1)
	n := runtime.Callers(skip+1, pcs)
	if n == 0 {
		return "(no stack)"
	}
	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	i := 0
	for {
		frame, more := frames.Next()
		_, _ = fmt.Fprintf(&b, "called from %s (%s:%d)\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
		i++
		if i >= nFrames {
			_, _ = fmt.Fprintf(&b, "(rest of stack elided)\n")
			break
		}
	}
	return b.String()
}

type Valuer func(ctx context.Context) Value

func (v Valuer) String() string {
	return "<Valuer>"
}

// ValuerValue return the function value on Valuer.
func ValuerValue(valuer Valuer) Value {
	return Value{kind: KindValuer, any: valuer}
}

func ResolveValuer(ctx context.Context, valuer Valuer) Value {
	return valuer(ctx)
}

func Timestamp(layout string) Valuer {
	return func(ctx context.Context) Value {
		return StringValue(time.Now().Format(layout))
	}
}

var callerDepthKey = struct{}{}

func WithCallerDepth(ctx context.Context, depth int) context.Context {
	if ctx == nil {
		ctx = context.TODO()
	}
	return context.WithValue(ctx, callerDepthKey, depth)
}

func (s *Source) String() string {
	var builder strings.Builder
	if s.File != "" {
		builder.WriteString(s.File)
	}
	if s.Line != 0 {
		builder.WriteByte(':')
		builder.WriteString(strconv.Itoa(s.Line))
	}
	return builder.String()
}

func Caller(depth int, full ...bool) Valuer {
	fullFilename := false
	if len(full) > 0 && full[0] {
		fullFilename = true
	}
	return func(ctx context.Context) Value {
		skip := depth
		if ctx != nil {
			if extraDepth, ok := ctx.Value(callerDepthKey).(int); ok && extraDepth > 0 {
				skip += extraDepth
			}
		}

		pc, file, line, _ := runtime.Caller(skip)
		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn != nil {
			fnName = fn.Name()
		}
		if fullFilename {
			return SourceValue(&Source{
				Function: fnName,
				File:     file,
				Line:     line,
			})
		}
		idx := strings.LastIndexByte(file, '/')
		if idx == -1 {
			return SourceValue(&Source{
				Function: fnName,
				File:     file[idx+1:],
				Line:     line,
			})
		}
		idx = strings.LastIndexByte(file[:idx], '/')
		return SourceValue(&Source{
			Function: fnName,
			File:     file[idx+1:],
			Line:     line,
		})
	}
}

//func Caller(depth int, full ...bool) Valuer {
//	fullFilename := false
//	if len(full) > 0 && full[0] {
//		fullFilename = true
//	}
//	return func(ctx context.Context) Value {
//		skip := depth
//		if ctx != nil {
//			if extraDepth, ok := ctx.Value(callerDepthKey).(int); ok && extraDepth > 0 {
//				skip += extraDepth
//			}
//		}
//		_, file, line, _ := runtime.Caller(skip)
//		if fullFilename {
//			return StringValue(file + ":" + strconv.Itoa(line))
//		}
//		idx := strings.LastIndexByte(file, '/')
//		if idx == -1 {
//			return StringValue(file[idx+1:] + ":" + strconv.Itoa(line))
//		}
//		idx = strings.LastIndexByte(file[:idx], '/')
//		return StringValue(file[idx+1:] + ":" + strconv.Itoa(line))
//	}
//}
