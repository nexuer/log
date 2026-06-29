package logmgr

import (
	"errors"
	"flag"
	"sync/atomic"

	"github.com/nexuer/log"
)

var defaultManager atomic.Pointer[Manager]

func checkInit() {
	if defaultManager.Load() == nil {
		panic(errors.New("logmgr is not initialized"))
	}
}

// Init creates and installs the singleton manager.
//
// Calling Init again replaces the current manager.
func Init(name string, opts ...Option) *Manager {
	m := newManager(name, opts...)
	defaultManager.Store(m)
	return m
}

// Close closes all printers managed by the singleton manager.
func Close() error {
	checkInit()
	return defaultManager.Load().Close()
}

// AddFlags registers log manager flags on fs.
func AddFlags(fs *flag.FlagSet) {
	checkInit()
	defaultManager.Load().AddFlags(fs)
}

// Apply applies options to the singleton manager's default scope.
//
// It does not update other scopes; use Scope.Apply for named scopes.
//
// If opts is empty, Apply is a no-op.
func Apply(opts ...Option) {
	checkInit()
	defaultManager.Load().Apply(opts...)
}

// Printer returns a printer from the singleton manager's default scope.
//
// With no name, it returns the default printer for the default scope. With a
// name, it returns a printer previously registered by Add or MustAdd.
func Printer(name ...string) *log.Printer {
	checkInit()
	return defaultManager.Load().Printer(name...)
}

// M returns the singleton manager installed by Init.
func M() *Manager {
	checkInit()
	return defaultManager.Load()
}

// AddScope registers a named scope on the singleton manager.
//
// It returns an error if the scope already exists.
func AddScope(name string, opts ...Option) (*Scope, error) {
	checkInit()
	return defaultManager.Load().AddScope(name, opts...)
}

// MustAddScope is like AddScope but panics if the scope already exists.
func MustAddScope(name string, opts ...Option) *Scope {
	checkInit()
	return defaultManager.Load().MustAddScope(name, opts...)
}

// Add registers a named printer in the singleton manager's default scope.
//
// It returns an error if the printer already exists.
func Add(name string) (*log.Printer, error) {
	checkInit()
	return defaultManager.Load().Add(name)
}

// MustAdd is like Add but panics if the printer already exists.
func MustAdd(name string) *log.Printer {
	checkInit()
	return defaultManager.Load().MustAdd(name)
}
