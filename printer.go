package log

// Printer is a logger wrapper that only emits plain log messages.
// It does not expose configuration, lifecycle, fatal, or structured-field methods.
type Printer struct {
	logger *Logger
}

// NewPrinter creates a new Printer, wrapping the provided Logger.
// If log is nil, it uses a default Logger from Default().
func NewPrinter(log *Logger) *Printer {
	if log == nil {
		log = defaultLogger.Load()
	} else {
		log = log.WithContext(WithCallerDepth(log.ctx, 1))
	}
	return &Printer{
		logger: log,
	}
}

func (r *Printer) Debug(args ...any) {
	r.logger.Debug(args...)
}

func (r *Printer) Debugf(format string, args ...any) {
	r.logger.Debugf(format, args...)
}

func (r *Printer) Info(args ...any) {
	r.logger.Info(args...)
}

func (r *Printer) Infof(format string, args ...any) {
	r.logger.Infof(format, args...)
}

func (r *Printer) Warn(args ...any) {
	r.logger.Warn(args...)
}

func (r *Printer) Warnf(format string, args ...any) {
	r.logger.Warnf(format, args...)
}

func (r *Printer) Error(args ...any) {
	r.logger.Error(args...)
}

func (r *Printer) Errorf(format string, args ...any) {
	r.logger.Errorf(format, args...)
}

func (r *Printer) Write(p []byte) (int, error) {
	return r.logger.Write(p)
}
