package logmgr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/nexuer/log"
)

// manager manages logger scopes and shared configuration.
type manager struct {
	initOptions []Option
	name        string
	mu          sync.RWMutex
	scopes      map[string]*Scope
	closed      bool // protected by mu
}

// newManager creates a manager with a default scope named after name.
func newManager(name string, opts ...Option) (*manager, error) {
	m := &manager{
		name:        name,
		initOptions: append([]Option(nil), opts...),
		scopes:      make(map[string]*Scope),
	}
	// add default scope
	_, err := m.register(name, true)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *manager) isDefaultScope(name string) bool {
	return m.name == name
}

func (m *manager) flagConfigs(name string) []*config {
	return []*config{defaultFlags.config, defaultFlags.set[""], defaultFlags.set[name]}
}

func (m *manager) Lookup(name ...string) (*Scope, bool) {
	key := m.name
	if len(name) > 0 {
		key = name[0]
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	scope, ok := m.scopes[key]
	return scope, ok
}

func (m *manager) Get(name ...string) (*Scope, error) {
	key := m.name
	if len(name) > 0 {
		key = name[0]
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	scope, ok := m.scopes[key]
	if !ok {
		return nil, fmt.Errorf("logmgr: no such scope: %q", key)
	}
	return scope, nil
}

func (m *manager) Register(name string, opts ...Option) (*Scope, error) {
	return m.register(name, false, opts...)
}

func (m *manager) register(name string, isDefault bool, opts ...Option) (*Scope, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("logmgr: closed")
	}

	if name == "" {
		return nil, errors.New("logmgr: scope name is empty")
	}

	_, ok := m.scopes[name]
	if ok {
		return nil, fmt.Errorf("logmgr: duplicate scope %s", name)
	}
	scope := newScope(m, name, isDefault, opts...)
	m.scopes[name] = scope
	return scope, nil
}

func (m *manager) scopeOpts(opts ...Option) []Option {
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

// Scopes returns a snapshot of all registered scopes sorted by name.
func (m *manager) Scopes() []*Scope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	scopes := make([]*Scope, 0, len(m.scopes))
	for _, scope := range m.scopes {
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].name < scopes[j].name })
	return scopes
}

// Remove releases a scope or selected loggers.
func (m *manager) Remove(scope string, logger ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.scopes[scope]
	if s == nil {
		return nil
	}
	if s.isDefault && len(logger) == 0 {
		return errors.New("logmgr: cannot remove default scope")
	}
	writers := s.takeWriters(logger)
	if len(logger) == 0 {
		delete(m.scopes, scope)
	}
	return closeWriters(writers)
}

func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var writers []io.Writer
	for _, scope := range m.scopes {
		writers = append(writers, scope.takeWriters(nil)...)
	}
	clear(m.scopes)
	return closeWriters(writers)
}

func closeWriter(w io.Writer) error {
	if w == os.Stdout || w == os.Stderr {
		return nil
	}
	closer, ok := w.(io.Closer)
	if !ok {
		return nil
	}
	err := closer.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func closeWriters(writers []io.Writer) error {
	var errs []error
	for _, w := range writers {
		if err := closeWriter(w); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cloneConfig(src *config) *config {
	if src == nil {
		return nil
	}
	dst := *src
	dst.File = src.File
	if src.Fields != nil {
		dst.Fields = append([]log.Field(nil), src.Fields...)
	}
	return &dst
}
