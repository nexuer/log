package logmgr

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/nexuer/log"
)

// Manager manages logger scopes and shared configuration.
type Manager struct {
	mu *sync.RWMutex

	name   string
	flags  *flags
	scopes map[string]*Scope
}

// newManager creates a Manager with a default scope named after name.
func newManager(name string, opts ...Option) *Manager {
	m := &Manager{
		name: name,
		mu:   new(sync.RWMutex),
		flags: &flags{
			config: new(config),
			scopes: make(map[string]*config),
		},
		scopes: make(map[string]*Scope),
	}
	// add default scope
	_ = m.addScope(name, opts...)
	return m
}

func (m *Manager) isDefaultScope(name string) bool {
	return m.name == name
}

func (m *Manager) flagsConfig(name string) *config {
	if m.isDefaultScope(name) {
		return m.flags.config
	}
	return m.flags.scopes[name]
}

func (m *Manager) addScope(name string, opts ...Option) *Scope {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.addScopeLocked(name, opts...)
}

func (m *Manager) addScopeLocked(name string, opts ...Option) *Scope {
	scope := &Scope{
		name:    name,
		manager: m,
		config:  newConfig(opts, m.flagsConfig(name)),
		entries: make(map[string]*entry),
	}

	scope.upsertEntryLocked(name, true)
	m.scopes[name] = scope
	return scope
}

func (m *Manager) getScope(name string) (*Scope, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	scope, ok := m.scopes[name]
	return scope, ok
}

// AddFlags registers log manager flags on fs.
func (m *Manager) AddFlags(fs *flag.FlagSet) {
	m.flags.AddFlags(fs)
}

// Add registers a named printer in the default scope.
//
// It returns an error if the printer already exists.
func (m *Manager) Add(name string) (*log.Printer, error) {
	return m.DefaultScope().Add(name)
}

// MustAdd is like Add but panics if the printer already exists.
func (m *Manager) MustAdd(name string) *log.Printer {
	return m.DefaultScope().MustAdd(name)
}

// AddScope registers a named scope.
//
// It returns an error if the scope already exists.
func (m *Manager) AddScope(name string, opts ...Option) (*Scope, error) {
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
// name, it returns a printer previously registered by Add or MustAdd.
func (m *Manager) Printer(name ...string) *log.Printer {
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
	printer *log.Printer
}

func (e *entry) apply(name string, cfg *config) {
	h := cfg.handler(name)
	if len(cfg.Fields) > 0 {
		h = h.WithFields(e.logger.Context(), cfg.Fields...)
	}
	w, newPath := cfg.writer(name, e.logger.Writer())
	if newPath != "" {
		e.logger.Infof("redirecting log output to file %q", newPath)
	}
	e.logger.SetLevel(*cfg.Level)
	e.logger.SetHandler(h)
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
	s.config = newConfig(opts, s.manager.flagsConfig(s.name))

	for k, v := range s.entries {
		v.apply(k, s.config)
		s.setDefaultLogger(k, v)
	}
}

// Printer returns a printer from the scope.
//
// With no name, it returns the scope's default printer. With a name, it returns
// a printer previously registered by Add or MustAdd.
func (s *Scope) Printer(name ...string) *log.Printer {
	fullName := s.name

	if len(name) > 0 {
		fullName = s.fullName(name[0])
	}

	e, _ := s.getEntry(fullName)
	if e == nil {
		panic(fmt.Errorf(`logmgr: %q printer does not exist in scope %q`, name, s.name))
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

// MustAdd is like Add but panics if the printer already exists.
func (s *Scope) MustAdd(name string) *log.Printer {
	p, err := s.Add(name)
	if err != nil {
		panic(err)
	}
	return p
}

// Add registers a named printer in the scope.
//
// It returns an error if the printer already exists.
func (s *Scope) Add(name string) (*log.Printer, error) {
	s.locker().Lock()
	defer s.locker().Unlock()

	fullName := s.fullName(name)
	if _, ok := s.entries[fullName]; ok {
		return nil, fmt.Errorf(`logmgr: %q printer already exists in scope %q`, name, s.name)
	}

	e := s.upsertEntryLocked(name, false)
	return e.printer, nil
}

func (s *Scope) upsertEntryLocked(name string, isInit bool) *entry {
	fullName := s.fullName(name)
	if isInit {
		fullName = name
	}

	e := s.entries[fullName]
	if e == nil {
		e = &entry{
			logger: log.New(os.Stderr),
		}
	}

	e.apply(fullName, s.config)
	s.entries[fullName] = e
	s.setDefaultLogger(fullName, e)
	return e
}

func (s *Scope) setDefaultLogger(name string, e *entry) {
	if s.manager.isDefaultScope(s.name) && name == s.name {
		log.SetDefault(e.logger)
	}
}
