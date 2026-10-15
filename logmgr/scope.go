package logmgr

import (
	"os"
	"sync"
	"sync/atomic"

	"github.com/nexuer/log"
)

const (
	defaultLoggerName = ""
)

type Scope struct {
	manager *manager

	isDefault bool
	name      string
	config    *config
	mu        sync.RWMutex
	loggers   map[string]*logger // default logger key is empty str
	closed    bool
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

func (s *Scope) Name() string {
	return s.name
}

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

func (s *Scope) Apply(opts ...Option) {
	if len(opts) == 0 || s.closed {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range name {
		if v == defaultLoggerName {
			continue
		}
		l, ok := s.loggers[v]
		if !ok {
			continue
		}
		closeWriter(l.logger().Writer())
		delete(s.loggers, v)
	}
	return nil
}

func (s *Scope) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	var errs error
	for _, l := range s.loggers {
		closeWriter(l.logger().Writer())
	}
	s.closed = true
	clear(s.loggers)
	return errs
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
		closeWriter(oldWriter)
	}
	if isDefault {
		// The global logger does not pass through this printer's wrapper frame.
		log.SetDefault(next.WithContext(log.AddCallerDepth(ctx, -1)))
	}
}
