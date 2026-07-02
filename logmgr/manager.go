package logmgr

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/nexuer/log"
)

// Manager manages logger scopes and shared configuration.
type Manager struct {
	mu *sync.RWMutex

	initOptions []Option
	name        string
	scopes      map[string]*Scope
}

// newManager creates a Manager with a default scope named after name.
func newManager(name string, opts ...Option) *Manager {
	m := &Manager{
		name:        name,
		initOptions: opts,
		mu:          new(sync.RWMutex),
		scopes:      make(map[string]*Scope),
	}
	// add default scope
	_ = m.addScope(name)
	return m
}

func (m *Manager) isDefaultScope(name string) bool {
	return m.name == name
}

func (m *Manager) flagConfigs(name string) []*config {
	if m.isDefaultScope(name) {
		return []*config{defaultFlags.config, defaultFlags.set[""], defaultFlags.set[name]}
	}
	return []*config{defaultFlags.set[name]}
}

func (m *Manager) addScope(name string, opts ...Option) *Scope {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.addScopeLocked(name, opts...)
}

func (m *Manager) scopeOpts(opts ...Option) []Option {
	if len(opts) == 0 {
		return m.initOptions
	}

	if len(m.initOptions) == 0 {
		return opts
	}

	scopeOpts := make([]Option, 0, len(m.initOptions)+len(opts))
	scopeOpts = append(scopeOpts, m.initOptions...)
	scopeOpts = append(scopeOpts, opts...)
	return scopeOpts
}

func (m *Manager) addScopeLocked(name string, opts ...Option) *Scope {
	scope := &Scope{
		name:    name,
		manager: m,
		config:  applyConfig(nil, m.scopeOpts(opts...), m.flagConfigs(name)...),
		entries: make(map[string]*entry),
	}

	scope.upsertEntryLocked(name)
	m.scopes[name] = scope
	return scope
}

func (m *Manager) getScope(name string) (*Scope, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	scope, ok := m.scopes[name]
	return scope, ok
}

// AddScope registers a named scope.
//
// It returns an error if the scope already exists.
func (m *Manager) AddScope(name string, opts ...Option) (*Scope, error) {
	if name == "" {
		return nil, errors.New("logmgr: scope name is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scopes[name]; ok {
		return nil, fmt.Errorf(`logmgr: %q scope already exists`, name)
	}
	return m.addScopeLocked(name, opts...), nil
}

// MustAddScope is like AddScope but panics if the scope already exists.
func (m *Manager) MustAddScope(name string, opts ...Option) *Scope {
	s, err := m.AddScope(name, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// Apply applies options to the default scope.
//
// It does not update other scopes; use Scope.Apply for named scopes.
//
// If opts is empty, Apply is a no-op.
func (m *Manager) Apply(opts ...Option) {
	m.Scope(m.name).Apply(opts...)
}

// Printer returns a printer from the default scope.
//
// With no name, it returns the default printer for the default scope. With a
// name, it returns an existing printer or creates one with the default scope's
// configuration.
func (m *Manager) Printer(name ...string) log.Printer {
	return m.Scope(m.name).Printer(name...)
}

// DefaultScope returns the manager's default scope.
func (m *Manager) DefaultScope() *Scope {
	return m.Scope(m.name)
}

// Scope returns a named scope.
//
// It panics if the scope does not exist.
func (m *Manager) Scope(name string) *Scope {
	scope, _ := m.getScope(name)
	if scope == nil {
		panic(fmt.Errorf(`logmgr: %q scope does not exist`, name))
	}
	return scope
}

// Scopes returns a snapshot of all registered scopes sorted by name.
func (m *Manager) Scopes() []*Scope {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scopes := make([]*Scope, 0, len(m.scopes))
	for _, scope := range m.scopes {
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool {
		return scopes[i].name < scopes[j].name
	})
	return scopes
}

// Close closes all printers managed by m.
func (m *Manager) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error
	for _, scope := range m.scopes {
		for _, v := range scope.entries {
			if err := v.logger.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

type entry struct {
	logger  *log.Logger
	printer log.Printer
}

func (e *entry) apply(name string, cfg *config) {
	e.logger.SetLevel(*cfg.Level)
	h := cfg.handler(name)
	if len(cfg.Fields) > 0 {
		h = h.WithFields(e.logger.Context(), cfg.Fields...)
	}
	e.logger.SetHandler(h)
	w, newPath := cfg.writer(name, e.logger.Writer())
	if newPath != "" {
		e.logger.Infof("log output redirected to %s", newPath)
	}
	e.logger.SetOutput(w)
	e.printer = log.NewPrinter(e.logger)
}

// Scope is a named configuration scope. Printers in the same scope share the
// same resolved configuration.
type Scope struct {
	manager *Manager

	config *config

	name    string
	entries map[string]*entry
}

func (s *Scope) locker() *sync.RWMutex {
	return s.manager.mu
}

// Name returns the scope name.
func (s *Scope) Name() string {
	return s.name
}

// Apply applies options to the scope.
//
// If opts is empty, Apply is a no-op.
func (s *Scope) Apply(opts ...Option) {
	s.apply(false, opts...)
}

func (s *Scope) apply(force bool, opts ...Option) {
	if !force && len(opts) == 0 {
		return
	}
	s.locker().Lock()
	defer s.locker().Unlock()
	// new config

	s.config = applyConfig(s.config, opts, s.manager.flagConfigs(s.name)...)

	for k, v := range s.entries {
		v.apply(k, s.config)
		s.setDefaultLogger(k, v)
	}
}

// Printer returns a printer from the scope.
//
// With no name, it returns the scope's default printer. With a name, it returns
// an existing printer or creates one with the scope's configuration.
func (s *Scope) Printer(name ...string) log.Printer {
	fullName := s.name

	if len(name) > 0 && name[0] != "" {
		fullName = s.fullName(name[0])
	}

	s.locker().RLock()
	e := s.entries[fullName]
	s.locker().RUnlock()
	if e != nil {
		return e.printer
	}

	// create
	s.locker().Lock()
	defer s.locker().Unlock()

	e = s.entries[fullName]
	if e == nil {
		e = s.upsertEntryLocked(fullName)
	}
	return e.printer

}

func (s *Scope) fullName(name string) string {
	if s.name == "" {
		return name
	}
	return s.name + "." + name
}

func (s *Scope) getEntry(name string) (*entry, bool) {
	s.locker().RLock()
	defer s.locker().RUnlock()
	e, ok := s.entries[name]
	return e, ok
}

func (s *Scope) upsertEntryLocked(name string) *entry {
	e := s.entries[name]
	if e == nil {
		e = &entry{
			logger: log.New(os.Stderr),
		}
	}

	e.apply(name, s.config)
	s.entries[name] = e
	s.setDefaultLogger(name, e)
	return e
}

func (s *Scope) setDefaultLogger(name string, e *entry) {
	if s.manager.isDefaultScope(s.name) && name == s.name {
		log.SetDefault(e.logger)
	}
}
