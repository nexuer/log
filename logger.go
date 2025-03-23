package log

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Handler interface {
	WithFields(ctx context.Context, fields ...Field) Handler

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

func (l *Logger) Context() context.Context {
	return l.ctx
}

// SetLevel set the current minimum severity level for
// logging output. Note: This is not concurrency-safe.
func (l *Logger) SetLevel(level Level) *Logger {
	if level == l.level {
		return l
	}
	l.level = level
	return l
}

// SetOutput set the current io.Writer
// Note: This is not concurrency-safe.
func (l *Logger) SetOutput(w io.Writer) *Logger {
	if l.Writer() == w {
		return l
	}
	l.w = addWriteCloser(w)
	return l
}

func (l *Logger) Write(p []byte) (n int, err error) {
	err = l.log(LevelInfo, string(p), nil)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// SetHandler set the current Handler
// Note: This is not concurrency-safe.
func (l *Logger) SetHandler(h Handler) *Logger {
	if l.handler == h {
		return l
	}
	l.handler = h
	return l
}

func (l *Logger) log(level Level, template string, fmtArgs []any, kvs ...any) error {
	if !l.level.Enable(level) {
		return nil
	}

	if l.handler != nil {
		msg := getMessage(template, fmtArgs)
		return l.Handle(l.ctx, l.w, level, msg, kvs...)
	}
	return nil
}

func (l *Logger) Log(ctx context.Context, level Level, msg string, kvs ...any) error {
	if !l.level.Enable(level) {
		return nil
	}

	if l.handler != nil {
		return l.Handle(ctx, l.w, level, msg, kvs...)
	}
	return nil
}

func (l *Logger) Handle(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error {
	return l.handler.Handle(ctx, w, level, msg, kvs...)
}

func (l *Logger) With(kvs ...any) *Logger {
	if len(kvs) == 0 || l.handler == nil {
		return l
	}
	l2 := l.clone()
	l2.handler = l.handler.WithFields(l.ctx, kvsToFieldSlice(kvs)...)
	return l2
}

func (l *Logger) WithFields(fields ...Field) *Logger {
	if len(fields) == 0 || l.handler == nil {
		return l
	}
	l2 := l.clone()
	l2.handler = l.handler.WithFields(l.ctx, fields...)
	return l2
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	l2 := l.clone()
	l2.ctx = ctx
	return l2
}

// Debug logs a message at debug level.
func (l *Logger) Debug(args ...any) {
	err := l.log(LevelDebug, "", args)
	errorHandler(err)
}

// Debugf logs a message at debug level.
func (l *Logger) Debugf(format string, args ...any) {
	err := l.log(LevelDebug, format, args)
	errorHandler(err)
}

// DebugS logs a message at debug level with key vals.
func (l *Logger) DebugS(msg string, kvs ...any) {
	err := l.log(LevelDebug, msg, nil, kvs...)
	errorHandler(err)
}

// Info logs a message at info level.
func (l *Logger) Info(args ...any) {
	err := l.log(LevelInfo, "", args)
	errorHandler(err)
}

// Infof logs a message at info level.
func (l *Logger) Infof(format string, args ...any) {
	err := l.log(LevelInfo, format, args)
	errorHandler(err)
}

// InfoS logs a message at info level with key vals.
func (l *Logger) InfoS(msg string, kvs ...any) {
	err := l.log(LevelInfo, msg, nil, kvs...)
	errorHandler(err)
}

// Warn logs a message at warn level.
func (l *Logger) Warn(args ...any) {
	err := l.log(LevelWarn, "", args)
	errorHandler(err)
}

// Warnf logs a message at warn level.
func (l *Logger) Warnf(format string, args ...any) {
	err := l.log(LevelWarn, format, args)
	errorHandler(err)
}

// WarnS logs a message at warn level with key vals.
func (l *Logger) WarnS(msg string, kvs ...any) {
	err := l.log(LevelWarn, msg, nil, kvs...)
	errorHandler(err)
}

// Error logs a message at error level.
func (l *Logger) Error(args ...any) {
	err := l.log(LevelError, "", args)
	errorHandler(err)
}

// Errorf logs a message at warn level.
func (l *Logger) Errorf(format string, args ...any) {
	err := l.log(LevelError, format, args)
	errorHandler(err)
}

// ErrorS logs a message at error level with key vals.
func (l *Logger) ErrorS(err error, msg string, kvs ...any) {
	if err == nil {
		errorHandler(l.log(LevelError, msg, nil, kvs...))
		return
	}
	if len(kvs) == 0 {
		errorHandler(l.log(LevelError, msg, nil, ErrKey, err.Error()))
		return
	}
	nv := make([]any, 0, len(kvs)+2)
	nv = append(nv, ErrKey, err.Error())
	nv = append(nv, kvs...)
	errorHandler(l.log(LevelError, msg, nil, nv...))
}

// Fatal logs a message at fatal level.
func (l *Logger) Fatal(args ...any) {
	err := l.log(LevelFatal, "", args)
	errorHandler(err)

	os.Exit(1)
}

// Fatalf logs a message at warn level.
func (l *Logger) Fatalf(format string, args ...any) {
	err := l.log(LevelFatal, format, args)
	errorHandler(err)

	os.Exit(1)
}

// FatalS logs a message at fatal level with key vals.
func (l *Logger) FatalS(err error, msg string, kvs ...any) {
	if err == nil {
		errorHandler(l.log(LevelFatal, msg, nil, kvs...))
		return
	}
	if len(kvs) == 0 {
		errorHandler(l.log(LevelFatal, msg, nil, ErrKey, err.Error()))
		return
	}
	nv := make([]any, 0, len(kvs)+2)
	nv = append(nv, ErrKey, err.Error())
	nv = append(nv, kvs...)
	errorHandler(l.log(LevelFatal, msg, nil, nv...))

	os.Exit(1)
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(template string, fmtArgs []interface{}) string {
	if len(fmtArgs) == 0 {
		return template
	}

	if template != "" {
		return fmt.Sprintf(template, fmtArgs...)
	}

	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(fmtArgs...)
}

func errorHandler(err error) {
	if err == nil {
		return
	}
	if ErrorHandler != nil {
		ErrorHandler(err)
	}
}
