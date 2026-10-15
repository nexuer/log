package logmgr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/nexuer/log"
)

// manager manages logger scopes and shared configuration.
type manager struct {
	initOptions []Option
	name        string
	mu          sync.RWMutex
	scopes      map[string]*Scope
	closed      atomic.Bool
}

// newManager creates a manager with a default scope named after name.
func newManager(name string, opts ...Option) (*manager, error) {
	m := &manager{
		name:        name,
		initOptions: append([]Option(nil), opts...),
	}
	// add default scope
	scope, err := m.Register(name)
	if err != nil {
		return nil, err
	}
	scope.isDefault = true
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
	if m.closed.Load() {
		return nil, errors.New("logmgr: closed")
	}
	if name == "" {
		return nil, errors.New("logmgr: scope name is empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.scopes[name]
	if ok {
		return nil, fmt.Errorf("logmgr: duplicate scope %s", name)
	}
	scope := newScope(m, name, opts...)
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
	return scopes
}

// Remove releases a scope or selected printers under the manager lock.
func (m *manager) Remove(scope string, logger ...string) error {
	if m.closed.Load() {
		return nil
	}
	s, err := m.Get(scope)
	if err != nil {
		return err
	}
	if s.isDefault && len(logger) == 0 {
		return errors.New("logmgr: cannot remove default scop")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// delete scope
	if len(logger) == 0 {
		_ = s.close()
		delete(m.scopes, scope)
		return nil
	}

	return s.remove(logger...)
}

func (m *manager) Close() error {
	if m.closed.Load() {
		return errors.New("logmgr: already closed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for _, scope := range m.scopes {
		if err := scope.close(); err != nil {
			errs = append(errs, err)
		}
	}
	m.closed.Store(true)
	clear(m.scopes)
	return errors.Join(errs...)
}

func closeWriter(w io.Writer) {
	if w == os.Stdout || w == os.Stderr {
		return
	}
	closer, ok := w.(io.Closer)
	if !ok {
		return
	}
	err := closer.Close()
	if err != nil && !errors.Is(err, os.ErrClosed) && log.ErrorHandler != nil {
		log.ErrorHandler(fmt.Errorf("logmgr: close previous output: %w", err))
	}
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
