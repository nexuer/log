package logmgr

import (
	"errors"
	"flag"
	"sync"
	"sync/atomic"

	"github.com/nexuer/log"
)

var defaultManager atomic.Pointer[manager]
var initMu sync.Mutex
var defaultFlags = newFlags()

func currentManager() *manager {
	m := defaultManager.Load()
	if m == nil {
		panic(errors.New("logmgr: not initialized"))
	}
	return m
}

// Initialized reports whether Init has been called successfully.
func Initialized() bool {
	return defaultManager.Load() != nil
}

// Init creates the default scope. It panics for an empty name or repeated calls.
func Init(name string, opts ...Option) {
	initMu.Lock()
	defer initMu.Unlock()
	if defaultManager.Load() != nil {
		panic(errors.New("logmgr: already initialized"))
	}
	m, err := newManager(name, opts...)
	if err != nil {
		panic(err)
	}
	defaultManager.Store(m)
}

// Get returns a registered scope, or the default scope if name is omitted.
// It panics if the scope does not exist.
func Get(name ...string) *Scope {
	scope, err := currentManager().Get(name...)
	if err != nil {
		panic(err)
	}
	return scope
}

// Register creates a scope inheriting Init options.
// It panics for empty or duplicate names, or after Close.
func Register(name string, opts ...Option) *Scope {
	scope, err := currentManager().Register(name, opts...)
	if err != nil {
		panic(err)
	}
	return scope
}

// Logger returns a logger from the default scope.
func Logger(name ...string) log.Printer {
	return Get().Logger(name...)
}

// Lookup returns a scope and whether it exists.
func Lookup(name string) (*Scope, bool) {
	return currentManager().Lookup(name)
}

// Scopes returns registered scopes sorted by name.
func Scopes() []*Scope {
	return currentManager().Scopes()
}

// Remove deletes named loggers and closes their outputs.
// With no logger names, it removes the entire scope.
// The default scope and its default logger are protected.
func Remove(scope string, printers ...string) error {
	return currentManager().Remove(scope, printers...)
}

// Close closes all outputs and clears registrations. Further registration is forbidden.
// Stop logging before calling Close. Repeated calls succeed.
func Close() error {
	return currentManager().Close()
}

// AddFlags registers logging flags on fs. Parse them before Init.
func AddFlags(fs *flag.FlagSet) {
	defaultFlags.AddFlags(fs)
}
