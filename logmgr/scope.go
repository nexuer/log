package logmgr

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/nexuer/log"
)

const (
	defaultLoggerName = ""
)

// Scope groups loggers sharing the same configuration.
type Scope struct {
	manager *manager

	isDefault bool
	name      string
	config    *config
	mu        sync.RWMutex
	loggers   map[string]*logger // default logger key is empty str
}

func newScope(m *manager, name string, isDefault bool, opts ...Option) *Scope {
	s := &Scope{
		name:      name,
		manager:   m,
		isDefault: isDefault,
		config:    applyConfig(nil, m.scopeOpts(opts...), m.flagConfigs(name)...),
		loggers:   make(map[string]*logger),
	}
	// add default logger
	s.addLogger(defaultLoggerName)
	return s
}

// Name returns the scope name.
func (s *Scope) Name() string {
	return s.name
}

// Logger returns an existing logger or creates one. Omit name for the default logger.
func (s *Scope) Logger(name ...string) log.Printer {
	n := defaultLoggerName
	if len(name) > 0 {
		n = name[0]
	}
	l := s.logger(n, true)
	return l
}

func (s *Scope) isDefaultLogger(name string) bool {
	return s.isDefault && name == defaultLoggerName
}

// Apply updates the scope configuration, including previously obtained loggers.
func (s *Scope) Apply(opts ...Option) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(opts) == 0 {
		return
	}
	oldCfg := cloneConfig(s.config)
	s.config = applyConfig(oldCfg, opts, s.manager.flagConfigs(s.name)...)
	for k, v := range s.loggers {
		v.apply(s.fullName(k), s.config, s.isDefaultLogger(k))
	}

}

func (s *Scope) remove(name ...string) error {
	if len(name) == 0 {
		return nil
	}
	return closeWriters(s.takeWriters(name))
}

// takeWriters removes loggers and returns their outputs. Empty name selects all.
func (s *Scope) takeWriters(name []string) []io.Writer {
	s.mu.Lock()
	defer s.mu.Unlock()
	var writers []io.Writer
	if len(name) == 0 {
		for _, l := range s.loggers {
			writers = append(writers, l.Load().Writer())
		}
		clear(s.loggers)
		return writers
	}
	for _, v := range name {
		if s.isDefault && v == defaultLoggerName {
			continue
		}
		l, ok := s.loggers[v]
		if !ok {
			continue
		}
		writers = append(writers, l.Load().Writer())
		delete(s.loggers, v)
	}
	return writers
}

func (s *Scope) close() error {
	return closeWriters(s.takeWriters(nil))
}

func (s *Scope) fullName(name string) string {
	if name == defaultLoggerName { // default logger name
		return s.name
	}
	return s.name + "." + name
}

func (s *Scope) logger(name string, create bool) *logger {
	s.mu.RLock()
	l, ok := s.loggers[name]
	if ok {
		s.mu.RUnlock()
		return l
	}
	s.mu.RUnlock()
	// create new logger
	if create {
		return s.addLogger(name)
	}
	return nil
}

func (s *Scope) addLogger(name string) *logger {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.loggers[name]
	if ok {
		return l
	}
	l = &logger{}
	initial := log.New(os.Stderr)
	l.Store(initial.WithContext(log.AddCallerDepth(initial.Context(), 1)))
	l.apply(s.fullName(name), s.config, s.isDefaultLogger(name))
	s.loggers[name] = l
	return l
}

type logger struct {
	atomic.Pointer[log.Logger]
}

var _ log.Printer = (*logger)(nil)

func (l *logger) Debug(args ...any)                 { l.Load().Debug(args...) }
func (l *logger) Debugf(format string, args ...any) { l.Load().Debugf(format, args...) }
func (l *logger) Info(args ...any)                  { l.Load().Info(args...) }
func (l *logger) Infof(format string, args ...any)  { l.Load().Infof(format, args...) }
func (l *logger) Warn(args ...any)                  { l.Load().Warn(args...) }
func (l *logger) Warnf(format string, args ...any)  { l.Load().Warnf(format, args...) }
func (l *logger) Error(args ...any)                 { l.Load().Error(args...) }
func (l *logger) Errorf(format string, args ...any) { l.Load().Errorf(format, args...) }
func (l *logger) Write(p []byte) (int, error)       { return l.Load().Write(p) }

func (l *logger) logger() *log.Logger {
	return l.Load()
}

func (l *logger) apply(name string, config *config, isDefault bool) {
	h := config.handler(name)
	old := l.Load()
	ctx := old.Context()
	if len(config.Fields) > 0 {
		h = h.WithFields(ctx, config.Fields...)
	}
	oldWriter := old.Writer()
	w, filepath := config.writer(name, oldWriter)
	if filepath != "" {
		// use new handler
		log.New(oldWriter, h).
			SetLevel(*config.Level).
			Infof("log output redirected to %s", filepath)
	}

	next := log.New(w, h).
		SetLevel(*config.Level).
		WithContext(ctx)

	l.Store(next)
	// close old writer
	if oldWriter != w {
		if err := closeWriter(oldWriter); err != nil && log.ErrorHandler != nil {
			log.ErrorHandler(fmt.Errorf("logmgr: close previous output: %w", err))
		}
	}
	if isDefault {
		// The global logger does not pass through this printer's wrapper frame.
		log.SetDefault(next.WithContext(log.AddCallerDepth(ctx, -1)))
	}
}
