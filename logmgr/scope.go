package logmgr

import (
	"errors"
	"os"
	"sync"

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

func newScope(m *manager, name string, opts ...Option) *Scope {
	s := &Scope{
		name:    name,
		manager: m,
		config:  applyConfig(nil, m.scopeOpts(opts...), m.flagConfigs(name)...),
		loggers: make(map[string]*logger),
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
	return l.printer
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
	newCfg := applyConfig(oldCfg, opts, s.manager.flagConfigs(s.name)...)
	for k, v := range s.loggers {
		v.apply(s.fullName(k), newCfg, s.isDefaultLogger(k))
	}
}

func (s *Scope) remove(name ...string) error {
	if len(name) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range name {
		l, ok := s.loggers[v]
		if !ok {
			continue
		}
		_ = l.logger.Close()
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
		if err := l.logger.Close(); err != nil {
			errs = errors.Join(errs, err)
		}
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
		s.addLogger(name)
	}
	return nil
}

func (s *Scope) addLogger(name string) *logger {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.loggers[name]
	if !ok {
		l = &logger{
			logger: log.New(os.Stderr),
		}
	}
	l.apply(s.fullName(name), s.config, s.isDefaultLogger(name))
	s.loggers[name] = l
	return l
}

type logger struct {
	logger  *log.Logger
	printer log.Printer
}

func (l *logger) apply(name string, config *config, isDefault bool) {
	h := config.handler(name)
	ctx := l.logger.Context()
	if len(config.Fields) > 0 {
		h = h.WithFields(ctx, config.Fields...)
	}
	oldWriter := l.logger.Writer()
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

	l2 := &logger{
		logger:  next,
		printer: log.NewPrinter(next),
	}

	*l = *l2
	// close old writer
	if oldWriter != w {
		closeWriter(oldWriter)
	}
	if isDefault {
		log.SetDefault(l.logger)
	}
}
