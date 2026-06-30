package logmgr

import (
	"errors"
	"sync/atomic"
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

// M returns the singleton manager installed by Init.
func M() *Manager {
	checkInit()
	return defaultManager.Load()
}
