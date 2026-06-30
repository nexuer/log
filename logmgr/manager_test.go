package logmgr_test

import (
	"flag"
	"sync"
	"testing"

	"github.com/nexuer/log"
	"github.com/nexuer/log/logmgr"
)

func TestSingletonManagerAPI(t *testing.T) {
	m := logmgr.Init("server", logmgr.WithLevel(log.LevelWarn))

	if got := logmgr.M(); got != m {
		t.Fatal("M did not return the manager installed by Init")
	}
	if got := m.DefaultScope(); got != m.Scope("server") {
		t.Fatal("DefaultScope did not return the default scope")
	}
	if got := m.DefaultScope().Name(); got != "server" {
		t.Fatalf("default scope name = %q, want %q", got, "server")
	}

	if _, err := m.Add("worker"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if _, err := m.Add("worker"); err == nil {
		t.Fatal("expected duplicate default-scope printer error")
	}

	db := m.MustAddScope("db", logmgr.WithLevel(log.LevelError))
	if got := m.Scope("db"); got != db {
		t.Fatal("Scope did not return the registered scope")
	}
	if got := db.Name(); got != "db" {
		t.Fatalf("db scope name = %q, want %q", got, "db")
	}
	scopes := m.Scopes()
	if len(scopes) != 2 {
		t.Fatalf("Scopes returned %d scopes, want 2", len(scopes))
	}
	if scopes[0].Name() != "db" || scopes[1].Name() != "server" {
		t.Fatalf("Scopes returned names [%q %q], want [db server]", scopes[0].Name(), scopes[1].Name())
	}
	if _, err := db.Add("mysql"); err != nil {
		t.Fatalf("scope Add returned error: %v", err)
	}

	m.Printer().Warn("server ready")
	m.Printer("worker").Error("worker failed")
	db.Printer().Error("database ready")
	db.Printer("mysql").Error("query failed")

	m.Apply(logmgr.WithFormat(logmgr.TextFormat))
	db.Apply(logmgr.WithOutput(logmgr.StderrOutput))
}

func TestFlagsAffectNewScopesAndPrinters(t *testing.T) {
	m := logmgr.Init("server")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	m.AddFlags(fs)

	if err := fs.Parse([]string{
		"--log-level=error",
		"--log-format=json",
		"--log-output=stderr",
		"--log-file-size=128",
		"--log-file-backups=3",
		"--log-file-compress=true",
		"--log-scope=db.level=debug",
		"--log-scope=db.file-dir=log/db",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	server := m.MustAdd("worker")
	db := m.MustAddScope("db")
	mysql := db.MustAdd("mysql")

	server.Info("filtered by global error level")
	server.Error("global error")
	mysql.Debug("scope debug")
}

func TestDuplicateRegistration(t *testing.T) {
	m := logmgr.Init("server")

	if _, err := m.AddScope("server"); err == nil {
		t.Fatal("expected duplicate default scope error")
	}

	if _, err := m.Add("worker"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if _, err := m.Add("worker"); err == nil {
		t.Fatal("expected duplicate printer error")
	}

	scope := m.MustAddScope("db")
	if _, err := scope.Add("mysql"); err != nil {
		t.Fatalf("scope Add returned error: %v", err)
	}
	if _, err := scope.Add("mysql"); err == nil {
		t.Fatal("expected duplicate scope printer error")
	}
}

func TestConcurrentDuplicateRegistration(t *testing.T) {
	m := logmgr.Init("server")

	var wg sync.WaitGroup
	scopeErrs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.AddScope("db")
			scopeErrs <- err
		}()
	}
	wg.Wait()
	close(scopeErrs)

	scopeSuccess := 0
	for err := range scopeErrs {
		if err == nil {
			scopeSuccess++
		}
	}
	if scopeSuccess != 1 {
		t.Fatalf("AddScope successes = %d, want 1", scopeSuccess)
	}

	scope := m.Scope("db")
	entryErrs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := scope.Add("mysql")
			entryErrs <- err
		}()
	}
	wg.Wait()
	close(entryErrs)

	entrySuccess := 0
	for err := range entryErrs {
		if err == nil {
			entrySuccess++
		}
	}
	if entrySuccess != 1 {
		t.Fatalf("Scope.Add successes = %d, want 1", entrySuccess)
	}
}

func TestMustHelpersPanicOnDuplicate(t *testing.T) {
	m := logmgr.Init("server")
	m.MustAdd("worker")
	m.MustAddScope("db")

	mustPanic(t, func() {
		m.MustAdd("worker")
	})
	mustPanic(t, func() {
		m.MustAddScope("db")
	})
}

func TestMissingScopeAndPrinterPanic(t *testing.T) {
	m := logmgr.Init("server")

	mustPanic(t, func() {
		m.Scope("missing")
	})
	mustPanic(t, func() {
		m.Printer("missing")
	})
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	f()
}
