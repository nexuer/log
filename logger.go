package log

import (
	"context"
	"errors"
	"io"
	"os"
)

type Handler interface {
	// With returns a new Handler whose attributes consist of
	// both the receiver's attributes and the key vals.
	// The Handler owns the slice: it may retain, modify or discard it.
	With(kvs ...any) Handler
	// Handle handles the Log with Context, Writer, Level , Message and the arguments.
	Handle(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error
}

// Keys for "built-in" attributes.
const (
	// LevelKey is the key used by the built-in handlers for the level
	// of the log call.
	LevelKey = "level"
	// MessageKey is the key used by the built-in handlers for the
	// message of the log call. The associated value is a string.
	MessageKey = "msg"
	// NameKey is the key used by the built-in handlers for the logger name.
	NameKey = "logger"
	// ErrKey is the key used by the built-in handlers for the error message.
	ErrKey = "err"
)

type Logger struct {
	ctx     context.Context
	level   Level
	handler Handler
	w       io.WriteCloser
}

func New(w io.Writer, h ...Handler) *Logger {
	if w == nil {
		w = io.Discard
	}
	l := &Logger{
		w: addWriteCloser(w),
	}
	if len(h) > 0 && h[0] != nil {
		l.handler = h[0]
	} else {
		l.handler = Text()
	}
	return l
}

func (l *Logger) clone() *Logger {
	return &Logger{
		ctx:     l.ctx,
		w:       l.w,
		level:   l.level,
		handler: l.handler,
	}
}

func (l *Logger) Close() error {
	if l.w == nil {
		return nil
	}
	return l.w.Close()
}

func (l *Logger) Writer() io.Writer {
	switch w := l.w.(type) {
	case writerWrapper:
		return w.Writer
	default:
		return l.w
	}
}

// SetLevel set the current minimum severity level for
// logging output. Note: This is not concurrency-safe.
func (l *Logger) SetLevel(level Level) {
	if level == l.level {
		return
	}
	l.level = level
}

// SetOutput set the current io.Writer
// Note: This is not concurrency-safe.
func (l *Logger) SetOutput(w io.Writer) {
	if l.Writer() == w {
		return
	}
	l.w = addWriteCloser(w)
}

// SetHandler set the current Handler
// Note: This is not concurrency-safe.
func (l *Logger) SetHandler(h Handler) {
	if l.handler == h {
		return
	}
	l.handler = h
}

func (l *Logger) enable(level Level) bool {
	return level >= l.level
}

// LogTo writes a log entry to the specified io.Writer with the given level, message, and fields.
// It skips logging if both the message is empty and no fields are provided, or if the specified level
// is not enabled for this Logger. The log entry is formatted and written using the Logger's handler.
//
// Parameters:
//   - w: The io.Writer to which the log entry will be written (e.g., os.Stdout, a file).
//   - level: The log level (e.g., INFO, WARN, ERROR) for this entry.
//   - msg: The log message to be written.
//   - kvs: Optional variadic arguments representing additional log fields (e.g., key-value pairs).
//
// Returns:
//   - error: Returns nil if the log entry is successfully written or skipped, an error if the handler
//     is nil or if the handler fails to process the entry.
//
// Example:
//
//	logger.LogTo(os.Stdout, log.LevelInfo, "Service started", GOOS, runtime.GOOS)
func (l *Logger) LogTo(w io.Writer, level Level, msg string, kvs ...any) error {
	return l.LogContextTo(l.ctx, w, level, msg, kvs...)
}

// LogContextTo writes a log entry to the specified io.Writer with the given context, level, message, and fields.
// It skips logging if both the message is empty and no fields are provided, or if the specified level
// is not enabled for this Logger. The log entry is formatted and written using the Logger's handler.
//
// Parameters:
//   - ctx: The context to pass to the handler, which may include metadata or cancellation signals.
//   - w: The io.Writer to which the log entry will be written (e.g., os.Stdout, a file).
//   - level: The log level (e.g., INFO, WARN, ERROR) for this entry.
//   - msg: The log message to be written.
//   - kvs: Optional variadic arguments representing additional log fields (e.g., key-value pairs).
//
// Returns:
//   - error: Returns nil if the log entry is successfully written or skipped, an error if the handler
//     is nil or if the handler fails to process the entry.
//
// Example:
//
//	logger.LogContextTo(ctx, os.Stdout, log.LevelInfo, "Service started", GOOS, runtime.GOOS)
func (l *Logger) LogContextTo(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error {
	if (msg == "" && len(kvs) == 0) || !l.enable(level) {
		return nil
	}

	if l.handler == nil {
		return errors.New("logger handler is nil")
	}

	if err := l.handler.Handle(ctx, w, level, msg, kvs...); err != nil {
		return err
	}

	return nil
}

// Log writes a log entry using the logger's default io.Writer and context with the given level, message, and fields.
// It skips logging if both the message is empty and no fields are provided, or if the specified level
// is not enabled for this Logger. The log entry is formatted and written using the Logger's handler.
//
// Parameters:
//   - level: The log level (e.g., INFO, WARN, ERROR) for this entry.
//   - msg: The log message to be written.
//   - kvs: Optional variadic arguments representing additional log fields (e.g., key-value pairs).
//
// Returns:
//   - error: Returns nil if the log entry is successfully written or skipped, an error if the handler
//     is nil or if the handler fails to process the entry.
//
// Example:
//
//	logger.Log(log.LevelInfo, "Service started", GOOS, runtime.GOOS)
func (l *Logger) Log(level Level, msg string, kvs ...any) error {
	return l.LogContextTo(l.ctx, l.w, level, msg, kvs...)
}

// LogContext writes a log entry using the specified context and the logger's default io.Writer with the given level, message, and fields.
// It skips logging if both the message is empty and no fields are provided, or if the specified level
// is not enabled for this Logger. The log entry is formatted and written using the Logger's handler.
//
// Parameters:
//   - ctx: The context to pass to the handler, which may include metadata or cancellation signals.
//   - level: The log level (e.g., INFO, WARN, ERROR) for this entry.
//   - msg: The log message to be written.
//   - kvs: Optional variadic arguments representing additional log fields (e.g., key-value pairs).
//
// Returns:
//   - error: Returns nil if the log entry is successfully written or skipped, an error if the handler
//     is nil or if the handler fails to process the entry.
//
// Example:
//
//	logger.LogContext(ctx, log.LevelInfo, "Service started", GOOS, runtime.GOOS)
func (l *Logger) LogContext(ctx context.Context, level Level, msg string, kvs ...any) error {
	return l.LogContextTo(ctx, l.w, level, msg, kvs...)
}

func (l *Logger) With(kvs ...any) *Logger {
	if len(kvs) == 0 || l.handler == nil {
		return l
	}
	l2 := l.clone()
	l2.handler = l.handler.With(kvs...)
	return l2
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	l2 := l.clone()
	l2.ctx = ctx
	return l2
}

// Debug logs a message at debug level.
func (l *Logger) Debug(args ...any) {
	err := l.Log(LevelDebug, getMessage("", args))
	errorHandler(err)
}

// Debugf logs a message at debug level.
func (l *Logger) Debugf(format string, args ...any) {
	err := l.Log(LevelDebug, getMessage(format, args))
	errorHandler(err)
}

// DebugS logs a message at debug level with key vals.
func (l *Logger) DebugS(msg string, kvs ...any) {
	err := l.Log(LevelDebug, msg, kvs...)
	errorHandler(err)
}

// Info logs a message at info level.
func (l *Logger) Info(args ...any) {
	err := l.Log(LevelInfo, getMessage("", args))
	errorHandler(err)
}

// Infof logs a message at info level.
func (l *Logger) Infof(format string, args ...any) {
	err := l.Log(LevelInfo, getMessage(format, args))
	errorHandler(err)
}

// InfoS logs a message at info level with key vals.
func (l *Logger) InfoS(msg string, kvs ...any) {
	err := l.Log(LevelInfo, msg, kvs...)
	errorHandler(err)
}

// Warn logs a message at warn level.
func (l *Logger) Warn(args ...any) {
	err := l.Log(LevelWarn, getMessage("", args))
	errorHandler(err)
}

// Warnf logs a message at warn level.
func (l *Logger) Warnf(format string, args ...any) {
	err := l.Log(LevelWarn, getMessage(format, args))
	errorHandler(err)
}

// WarnS logs a message at warn level with key vals.
func (l *Logger) WarnS(msg string, kvs ...any) {
	err := l.Log(LevelWarn, msg, kvs...)
	errorHandler(err)
}

// Error logs a message at error level.
func (l *Logger) Error(args ...any) {
	err := l.Log(LevelError, getMessage("", args))
	errorHandler(err)
}

// Errorf logs a message at warn level.
func (l *Logger) Errorf(format string, args ...any) {
	err := l.Log(LevelError, getMessage(format, args))
	errorHandler(err)
}

// ErrorS logs a message at error level with key vals.
func (l *Logger) ErrorS(err error, msg string, kvs ...any) {
	if err == nil {
		errorHandler(l.Log(LevelError, msg, kvs...))
		return
	}
	if len(kvs) == 0 {
		errorHandler(l.Log(LevelError, msg, ErrKey, err.Error()))
		return
	}
	nv := make([]any, 0, len(kvs)+2)
	nv = append(nv, ErrKey, err.Error())
	nv = append(nv, kvs...)
	errorHandler(l.Log(LevelError, msg, nv...))
}

// Fatal logs a message at fatal level.
func (l *Logger) Fatal(args ...any) {
	err := l.Log(LevelFatal, getMessage("", args))
	errorHandler(err)

	os.Exit(1)
}

// Fatalf logs a message at warn level.
func (l *Logger) Fatalf(format string, args ...any) {
	err := l.Log(LevelFatal, getMessage(format, args))
	errorHandler(err)

	os.Exit(1)
}

// FatalS logs a message at fatal level with key vals.
func (l *Logger) FatalS(err error, msg string, kvs ...any) {
	if err == nil {
		errorHandler(l.Log(LevelFatal, msg, kvs...))
		return
	}
	if len(kvs) == 0 {
		errorHandler(l.Log(LevelFatal, msg, ErrKey, err.Error()))
		return
	}
	nv := make([]any, 0, len(kvs)+2)
	nv = append(nv, ErrKey, err.Error())
	nv = append(nv, kvs...)
	errorHandler(l.Log(LevelFatal, msg, nv...))

	os.Exit(1)
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(template string, args []any) string {
	if len(args) == 0 {
		return template
	}

	if template != "" {
		return Sprintf(template, args...)
	}

	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			return str
		}
	}

	return Sprint(args...)
}

func errorHandler(err error) {
	if err == nil {
		return
	}
	if ErrorHandler != nil {
		ErrorHandler(err)
	}
}
