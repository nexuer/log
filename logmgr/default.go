package logmgr

import (
	"errors"
	"flag"
	"sync"
	"sync/atomic"
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

// Init initializes process-wide log management and the default scope.
// An empty name panics.
// Calling Init again panics, including after Close.
// Register and parse flags before calling Init.
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

func Get(name ...string) *Scope {
	scope, err := currentManager().Get(name...)
	if err != nil {
		panic(err)
	}
	return scope
}

func Register(name string, opts ...Option) *Scope {
	scope, err := currentManager().Register(name, opts...)
	if err != nil {
		panic(err)
	}
	return scope
}

func Lookup(name string) (*Scope, bool) {
	return currentManager().Lookup(name)
}

// Scopes returns a snapshot of registered scopes sorted by name.
// It panics before Init or after Close.
func Scopes() []*Scope {
	return currentManager().Scopes()
}

// Remove deletes and closes selected printers within a scope. Printer names
// are relative to the scope; an empty name selects the scope's default printer.
// With no printer names, it deletes and closes the entire scope.
// Missing scopes or printers are ignored. The default scope and its default
// printer cannot be removed. Held printers stop output after removal; a removed
// LoggerScope can no longer be used to create printers or apply configuration.
// Recreating the same name creates new resources, leaving old references inactive.
// It reports output close errors and panics before Init. After Close it is a no-op.
func Remove(scope string, printers ...string) error {
	return currentManager().Remove(scope, printers...)
}

// Close releases all managed outputs and stops output from held printers.
// Further Close calls are no-ops. Scope access, printer creation, and configuration
// changes panic after Close. It panics before Init.
func Close() error {
	return currentManager().Close()
}

// AddFlags registers log manager flags on fs.
// Call AddFlags and parse the flag set before Init. Unscoped flags apply to all
// scopes; named log-set values override them for the specified scope.
func AddFlags(fs *flag.FlagSet) {
	defaultFlags.AddFlags(fs)
}
