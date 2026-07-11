package logmgr

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nexuer/log"
	"gopkg.in/natefinch/lumberjack.v2"
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

func TestApplyUpdatesHeldPrinter(t *testing.T) {
	resetDefault(t)
	dir := t.TempDir()
	m := Init("server",
		WithOutput(FileOutput),
		WithFileDir(dir),
		WithLevel(log.LevelError),
	)
	p := m.Printer()
	p.Debug("before apply")

	m.Apply(WithLevel(log.LevelDebug))
	if got := m.Printer(); got != p {
		t.Fatal("Apply replaced the managed Printer")
	}
	p.Debug("after apply")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	p.Debug("after close")

	data, err := os.ReadFile(dir + "/server.log")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "before apply") ||
		!strings.Contains(string(data), "after apply") ||
		strings.Contains(string(data), "after close") {
		t.Fatalf("file output = %q", data)
	}
}

func TestApplyUpdatesFileWriterConfiguration(t *testing.T) {
	resetDefault(t)
	m := Init("server",
		WithOutput(FileOutput),
		WithFileDir(t.TempDir()),
		WithFileSize(1),
		WithFileBackups(1),
	)
	entry := m.DefaultScope().entries["server"]
	before := entry.logger.Writer().(*lumberjack.Logger)

	m.Apply(
		WithFileSize(2),
		WithFileBackups(3),
		WithFileCompress(true),
	)
	after := entry.logger.Writer().(*lumberjack.Logger)
	if before == after {
		t.Fatal("Apply reused a file writer whose rotation configuration changed")
	}
	if after.MaxSize != 2 || after.MaxBackups != 3 || !after.Compress {
		t.Fatalf("file config = (%d, %d, %v), want (2, 3, true)", after.MaxSize, after.MaxBackups, after.Compress)
	}
}

func TestApplyMovesHeldPrinterToNewFile(t *testing.T) {
	resetDefault(t)
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	m := Init("server", WithOutput(FileOutput), WithFileDir(firstDir))
	p := m.Printer()
	p.Info("first file")
	m.Apply(WithFileDir(secondDir))
	p.Info("second file")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	first, err := os.ReadFile(firstDir + "/server.log")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondDir + "/server.log")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(first), "second file") {
		t.Fatalf("old file received post-Apply record: %q", first)
	}
	if !strings.Contains(string(second), "second file") {
		t.Fatalf("new file output = %q", second)
	}
}

func TestManagedPrinterCaller(t *testing.T) {
	resetDefault(t)
	dir := t.TempDir()
	m := Init("server",
		WithOutput(FileOutput),
		WithFileDir(dir),
		WithFormat(JsonFormat),
		WithKeyValues(log.DefaultFields...),
	)
	m.Printer().Info("caller")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dir + "/server.log")
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Caller log.Source `json:"caller"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(record.Caller.Function, ".TestManagedPrinterCaller") {
		t.Fatalf("caller function = %q, want TestManagedPrinterCaller", record.Caller.Function)
	}
}

func TestConcurrentApplyAndHeldPrinter(t *testing.T) {
	resetDefault(t)
	m := Init("server", WithOutput(FileOutput), WithFileDir(t.TempDir()))
	p := m.Printer()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			p.Debug("record")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			m.Apply(WithLevel(log.Level(i & 1)))
		}
	}()
	wg.Wait()
	if got := m.Printer(); got != p {
		t.Fatal("concurrent Apply replaced the managed Printer")
	}
}

type trackingCloser struct {
	closed bool
}

func (*trackingCloser) Write(p []byte) (int, error) { return len(p), nil }

func (w *trackingCloser) Close() error {
	w.closed = true
	return nil
}

func TestCloseWriterOwnership(t *testing.T) {
	closer := new(trackingCloser)
	if err := closeWriter(closer); err != nil {
		t.Fatal(err)
	}
	if !closer.closed {
		t.Fatal("owned writer was not closed")
	}
	for _, writer := range []io.Writer{os.Stdout, os.Stderr, io.Discard, log.Discard} {
		if err := closeWriter(writer); err != nil {
			t.Fatalf("closeWriter(%T) = %v", writer, err)
		}
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
