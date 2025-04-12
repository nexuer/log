package log

// Readonly is a logger wrapper that restricts modifications to the underlying logger.
// It prevents changes to log configuration, such as altering the log level or output,
// and excludes the Fatal level to avoid program termination.
// All methods delegate to the wrapped Logger, providing only logging output functionality.
type Readonly struct {
	logger *Logger
}

// NewReadonly creates a new Readonly logger, wrapping the provided Logger.
// If log is nil, it uses a default Logger from Default().
func NewReadonly(log *Logger) *Readonly {
	if log == nil {
		log = loadDefault()
	} else {
		log = log.WithContext(WithCallerDepth(log.ctx, 1))
	}
	return &Readonly{
		logger: log,
	}
}

func (r *Readonly) Debug(args ...any) {
	r.logger.Debug(args...)
}

func (r *Readonly) Debugf(format string, args ...any) {
	r.logger.Debugf(format, args...)
}

func (r *Readonly) DebugS(msg string, kvs ...any) {
	r.logger.DebugS(msg, kvs...)
}

func (r *Readonly) Info(args ...any) {
	r.logger.Info(args...)
}

func (r *Readonly) Infof(format string, args ...any) {
	r.logger.Infof(format, args...)
}

func (r *Readonly) InfoS(msg string, kvs ...any) {
	r.logger.InfoS(msg, kvs...)
}

func (r *Readonly) Warn(args ...any) {
	r.logger.Warn(args...)
}

func (r *Readonly) Warnf(format string, args ...any) {
	r.logger.Warnf(format, args...)
}

func (r *Readonly) WarnS(msg string, kvs ...any) {
	r.logger.WarnS(msg, kvs...)
}

func (r *Readonly) Error(args ...any) {
	r.logger.Error(args...)
}

func (r *Readonly) Errorf(format string, args ...any) {
	r.logger.Errorf(format, args...)
}

func (r *Readonly) ErrorS(err error, msg string, kvs ...any) {
	r.logger.ErrorS(err, msg, kvs...)
}

func (r *Readonly) Write(p []byte) (int, error) {
	return r.logger.Write(p)
}
