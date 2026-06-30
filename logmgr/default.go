package logmgr

import (
	"errors"
	"flag"
	"sync/atomic"
)

var defaultManager atomic.Pointer[Manager]
var defaultFlags = newFlags()

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

// M returns the singleton manager installed by Init.
func M() *Manager {
	checkInit()
	return defaultManager.Load()
}

// AddFlags registers log manager flags on fs.
//
// Call AddFlags and parse the flag set before Init so the default scope can
// apply parsed flag values when it is created.
func AddFlags(fs *flag.FlagSet) {
	defaultFlags.AddFlags(fs)
}
