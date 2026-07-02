package logmgr

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nexuer/log"
)

func resetDefault(t *testing.T) {
	t.Helper()
	defaultManager.Store(nil)
	defaultFlags = newFlags()
	t.Cleanup(func() {
		defaultManager.Store(nil)
		defaultFlags = newFlags()
	})
}

func TestSingletonManagerAPI(t *testing.T) {
	resetDefault(t)

	m := Init("server", WithLevel(log.LevelWarn))
	if got := M(); got != m {
		t.Fatal("M did not return the manager installed by Init")
	}
	if got := m.DefaultScope(); got != m.Scope("server") {
		t.Fatal("DefaultScope did not return the default scope")
	}
	if got := m.DefaultScope().Name(); got != "server" {
		t.Fatalf("default scope name = %q, want %q", got, "server")
	}

	worker := m.Printer("worker")
	if got := m.Printer("worker"); got != worker {
		t.Fatal("Printer did not return the existing default-scope printer")
	}

	db := m.MustAddScope("db", WithLevel(log.LevelError))
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
	mysql := db.Printer("mysql")
	if got := db.Printer("mysql"); got != mysql {
		t.Fatal("Printer did not return the existing scope printer")
	}

	m.Printer().Warn("server ready")
	m.Printer("worker").Error("worker failed")
	db.Printer().Error("database ready")
	db.Printer("mysql").Error("query failed")

	m.Apply(WithFormat(TextFormat))
	db.Apply(WithOutput(StderrOutput))
}

func TestDefaultLoggerFollowsDefaultScope(t *testing.T) {
	resetDefault(t)

	m := Init("server", WithOutput(StdoutOutput))
	if got := log.Default().Writer(); got != os.Stdout {
		t.Fatalf("default logger writer = %T, want stdout", got)
	}

	m.Apply(WithOutput(StderrOutput))
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("default logger writer after Apply = %T, want stderr", got)
	}

	db := m.MustAddScope("db", WithOutput(StdoutOutput))
	db.Apply(WithOutput(StdoutOutput))
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("named scope changed default logger writer to %T, want stderr", got)
	}
}

func TestScopeInheritsInitOptionsAndOverrides(t *testing.T) {
	resetDefault(t)

	m := Init("server",
		WithFields(log.String("service", "api")),
		WithOutput(StdoutOutput),
		WithLevel(log.LevelInfo),
	)
	db := m.MustAddScope("db", WithLevel(log.LevelError))

	if got := *db.config.Output; got != StdoutOutput {
		t.Fatalf("scope output = %v, want %v", got, StdoutOutput)
	}
	if got := *db.config.Level; got != log.LevelError {
		t.Fatalf("scope level = %v, want %v", got, log.LevelError)
	}
	if len(db.config.Fields) != 1 || !db.config.Fields[0].Equal(log.String("service", "api")) {
		t.Fatalf("scope fields = %v, want service=api", db.config.Fields)
	}
}

func TestApplyPreservesCurrentScopeConfig(t *testing.T) {
	resetDefault(t)

	m := Init("server", WithFields(log.String("service", "api")), WithOutput(StdoutOutput))
	db := m.MustAddScope("db", WithLevel(log.LevelWarn))

	db.Apply(WithLevel(log.LevelDebug))

	if got := *db.config.Output; got != StdoutOutput {
		t.Fatalf("scope output after Apply = %v, want %v", got, StdoutOutput)
	}
	if got := *db.config.Level; got != log.LevelDebug {
		t.Fatalf("scope level after Apply = %v, want %v", got, log.LevelDebug)
	}
	if len(db.config.Fields) != 1 || !db.config.Fields[0].Equal(log.String("service", "api")) {
		t.Fatalf("scope fields after Apply = %v, want service=api", db.config.Fields)
	}
}

func TestFlagsAffectNewScopesAndPrinters(t *testing.T) {
	resetDefault(t)

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)

	if err := fs.Parse([]string{
		"--log-level=error",
		"--log-format=json",
		"--log-output=stderr",
		"--log-file-size=128",
		"--log-file-backups=3",
		"--log-file-compress=true",
		"--log-set=db.level=debug",
		"--log-set=db.file-dir=log/db",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	m := Init("server")
	server := m.Printer("worker")
	db := m.MustAddScope("db")
	mysql := db.Printer("mysql")

	server.Info("filtered by global error level")
	server.Error("global error")
	mysql.Debug("scope debug")
}

func TestLogSetOverridesNamedDefaultScope(t *testing.T) {
	resetDefault(t)

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)

	if err := fs.Parse([]string{
		"--log-output=stderr",
		"--log-set=output=stdout",
		"--log-set=server.output=stderr",
		"--log-set=db.output=stdout",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	m := Init("server")
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("named default scope log-set was not applied last: got %T, want stderr", got)
	}
	_ = m.MustAddScope("db")
}

func TestFlagHelpMetavars(t *testing.T) {
	resetDefault(t)

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)

	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	out := buf.String()

	for _, want := range []string{
		"-log-level level",
		"-log-format format",
		"-log-output output",
		"-log-file-dir dir",
		"-log-file-size MB",
		"-log-file-backups count",
		"-log-set key=value",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("flag help missing %q:\n%s", want, out)
		}
	}
}

func TestDuplicateScopeRegistration(t *testing.T) {
	resetDefault(t)

	m := Init("server")

	if _, err := m.AddScope("server"); err == nil {
		t.Fatal("expected duplicate default scope error")
	}

	scope := m.MustAddScope("db")
	if _, err := m.AddScope("db"); err == nil {
		t.Fatal("expected duplicate scope error")
	}
	_ = scope
}

func TestEmptyNames(t *testing.T) {
	resetDefault(t)

	mustPanic(t, func() {
		Init("")
	})

	m := Init("server")
	if _, err := m.AddScope(""); err == nil {
		t.Fatal("expected empty scope name error")
	}
	mustPanic(t, func() {
		m.MustAddScope("")
	})
}

func TestInitPanicsWhenCalledTwice(t *testing.T) {
	resetDefault(t)

	Init("server")
	mustPanic(t, func() {
		Init("worker")
	})
}

func TestConcurrentScopeRegistrationAndPrinterCreation(t *testing.T) {
	resetDefault(t)

	m := Init("server")

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
	printers := make(chan log.Printer, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			printers <- scope.Printer("mysql")
		}()
	}
	wg.Wait()
	close(printers)

	var first log.Printer
	for p := range printers {
		if first == nil {
			first = p
			continue
		}
		if p != first {
			t.Fatal("concurrent Printer calls returned different printers")
		}
	}
}

func TestMustAddScopePanicOnDuplicate(t *testing.T) {
	resetDefault(t)

	m := Init("server")
	m.MustAddScope("db")

	mustPanic(t, func() {
		m.MustAddScope("db")
	})
}

func TestMissingScopePanicAndPrinterCreates(t *testing.T) {
	resetDefault(t)

	m := Init("server")

	mustPanic(t, func() {
		m.Scope("missing")
	})
	if got := m.Printer("missing"); got == nil {
		t.Fatal("Printer returned nil")
	}
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
