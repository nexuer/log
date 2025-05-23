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
	defaultLogger.Store(l.WithContext(WithCallerDepth(l.ctx, 1)))
}

// Default returns the default [Logger].
func Default() *Logger {
	logger := loadDefault()
	return logger.WithContext(WithCallerDepth(logger.ctx, 0))
}

func loadDefault() *Logger {
	return defaultLogger.Load()
}

// InitManager init global manager
func InitManager(name string, fields ...any) *Manager {
	m := NewManager(name, fields...)
	defaultManager.Store(m)
	return m
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
	return loadDefault().Close()
}

// Debug logs a message at debug level.
func Debug(args ...any) {
	loadDefault().Debug(args...)
}

// Debugf logs a message at debug level.
func Debugf(format string, args ...any) {
	loadDefault().Debugf(format, args...)
}

// DebugS logs a message at debug level with key vals.
func DebugS(msg string, kvs ...any) {
	loadDefault().DebugS(msg, kvs...)
}

// Info logs a message at info level.
func Info(args ...any) {
	loadDefault().Info(args...)
}

// Infof logs a message at info level.
func Infof(format string, args ...any) {
	loadDefault().Infof(format, args...)
}

// InfoS logs a message at info level with key vals.
func InfoS(msg string, kvs ...any) {
	loadDefault().InfoS(msg, kvs...)
}

// Warn logs a message at warn level.
func Warn(args ...any) {
	loadDefault().Warn(args...)
}

// Warnf logs a message at warn level.
func Warnf(format string, args ...any) {
	loadDefault().Warnf(format, args...)
}

// WarnS logs a message at warn level with key vals.
func WarnS(msg string, kvs ...any) {
	loadDefault().WarnS(msg, kvs...)
}

// Error logs a message at error level.
func Error(args ...any) {
	loadDefault().Error(args...)
}

// Errorf logs a message at error level.
func Errorf(format string, args ...any) {
	loadDefault().Errorf(format, args...)
}

// ErrorS logs a message at error level with key vals.
func ErrorS(err error, msg string, kvs ...any) {
	loadDefault().ErrorS(err, msg, kvs...)
}

// Fatal logs a message at fatal level.
func Fatal(args ...any) {
	loadDefault().Fatal(args...)
}

// Fatalf logs a message at fatal level.
func Fatalf(format string, args ...any) {
	loadDefault().Fatalf(format, args...)
}

// FatalS logs a message at fatal level with key vals.
func FatalS(err error, msg string, kvs ...any) {
	loadDefault().FatalS(err, msg, kvs...)
}
