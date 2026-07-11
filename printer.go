package log

// Printer is a logger wrapper that only emits plain log messages.
// It does not expose configuration, lifecycle, fatal, or structured-field methods.
type Printer interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Write(p []byte) (int, error)
}

type printer struct {
	logger *Logger
}

// NewPrinter creates a new Printer, wrapping the provided Logger.
// If log is nil, it uses a default Logger from Default().
func NewPrinter(log *Logger) Printer {
	if log == nil {
		log = defaultLogger.Load().global
	} else {
		log = log.WithContext(AddCallerDepth(log.ctx, 1))
	}
	return &printer{
		logger: log,
	}
}

func (r *printer) Debug(args ...any) {
	r.logger.Debug(args...)
}

func (r *printer) Debugf(format string, args ...any) {
	r.logger.Debugf(format, args...)
}

func (r *printer) Info(args ...any) {
	r.logger.Info(args...)
}

func (r *printer) Infof(format string, args ...any) {
	r.logger.Infof(format, args...)
}

func (r *printer) Warn(args ...any) {
	r.logger.Warn(args...)
}

func (r *printer) Warnf(format string, args ...any) {
	r.logger.Warnf(format, args...)
}

func (r *printer) Error(args ...any) {
	r.logger.Error(args...)
}

func (r *printer) Errorf(format string, args ...any) {
	r.logger.Errorf(format, args...)
}

func (r *printer) Write(p []byte) (int, error) {
	return r.logger.Write(p)
}
