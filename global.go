package log

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var (
	// ErrorHandler is called whenever fails to write an event on its
	// output. default an error is printed on the stderr. This handler must
	// be thread safe and non-blocking.
	ErrorHandler func(err error) = func(err error) {
		_, _ = fmt.Fprintf(os.Stderr, "log: write failed, %v\n", err)
	}
)

var (
	// Sprint is used to concatenate log message components into a single string.
	// It formats its arguments using the default behavior (fmt.Sprint).
	// The caller can replace this function to customize the log message formatting.
	Sprint func(a ...any) string = fmt.Sprint
	// Sprintf is used to format log messages with a specific format string.
	// It formats its arguments using the default behavior (fmt.Sprintf).
	// The caller can replace this function to customize the format or behavior of the log messages.
	Sprintf func(format string, a ...any) string = fmt.Sprintf
)

var (

	// DefaultCaller is a Valuer that returns the file and line.
	DefaultCaller = Caller(7)

	// DefaultTimestamp is a Valuer that returns the current wallclock time.
	DefaultTimestamp = Timestamp(time.RFC3339)

	DefaultFields = []any{
		"ts", DefaultTimestamp,
		"caller", DefaultCaller,
	}
)

var (
	defaultLogger  atomic.Pointer[Logger]
	defaultManager atomic.Pointer[Manager]
)

func init() {
	SetDefault(New(os.Stderr).With(DefaultFields...))
}

// SetDefault makes l the default [Logger], which is used by
// the top-level functions [Info], [Debug] and so on.
func SetDefault(l *Logger) {
	if l == nil {
		return
	}
	defaultLogger.Store(l.WithContext(withCallerDepthKey(l.ctx, 1)))
}

// Default returns the default [Logger].
func Default() *Logger { return defaultLogger.Load() }

// InitManager init global manager
func InitManager(name string, fields ...any) {
	defaultManager.Store(NewManager(name, fields...))
	SetDefault(M().Logger())
}

func M() *Manager {
	m := defaultManager.Load()
	if m == nil {
		panic("log: uninitialized manager not (forgotten use log.InitManager(name)?")
	}
	return m
}

func Close() error {
	m := defaultManager.Load()
	if m != nil {
		return m.Close()
	}
	return Default().Close()
}

// Debug logs a message at debug level.
func Debug(args ...any) {
	Default().Debug(args...)
}

// Debugf logs a message at debug level.
func Debugf(format string, args ...any) {
	Default().Debugf(format, args...)
}

// DebugS logs a message at debug level with fields.
func DebugS(msg string, fields ...any) {
	Default().DebugS(msg, fields...)
}

// Info logs a message at info level.
func Info(args ...any) {
	Default().Info(args...)
}

// Infof logs a message at info level.
func Infof(format string, args ...any) {
	Default().Infof(format, args...)
}

// InfoS logs a message at info level with fields.
func InfoS(msg string, fields ...any) {
	Default().InfoS(msg, fields...)
}

// Warn logs a message at warn level.
func Warn(args ...any) {
	Default().Warn(args...)
}

// Warnf logs a message at warn level.
func Warnf(format string, args ...any) {
	Default().Warnf(format, args...)
}

// WarnS logs a message at warn level with fields.
func WarnS(msg string, fields ...any) {
	Default().WarnS(msg, fields...)
}

// Error logs a message at error level.
func Error(args ...any) {
	Default().Error(args...)
}

// Errorf logs a message at error level.
func Errorf(format string, args ...any) {
	Default().Errorf(format, args...)
}

// ErrorS logs a message at error level with fields.
func ErrorS(err error, msg string, fields ...any) {
	Default().ErrorS(err, msg, fields...)
}

// Fatal logs a message at fatal level.
func Fatal(args ...any) {
	Default().Fatal(args...)
}

// Fatalf logs a message at fatal level.
func Fatalf(format string, args ...any) {
	Default().Fatalf(format, args...)
}

// FatalS logs a message at fatal level with key vals.
func FatalS(err error, msg string, fields ...any) {
	Default().FatalS(err, msg, fields...)
}
