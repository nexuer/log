package logmgr

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"runtime"
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

	Init("server", WithLevel(log.LevelWarn), WithFields(log.DefaultFields...))
	if got := Get(); got != Get("server") {
		t.Fatal("DefaultScope did not return the default scope")
	}
	if got := Get().Name(); got != "server" {
		t.Fatalf("default scope name = %q, want %q", got, "server")
	}

	worker := Get().Logger("worker")
	if got := Get().Logger("worker"); got != worker {
		t.Fatal("Printer did not return the existing default-scope printer")
	}

	db := Register("db", WithLevel(log.LevelError))
	if got := Get("db"); got != db {
		t.Fatal("Scope did not return the registered scope")
	}
	if got := db.Name(); got != "db" {
		t.Fatalf("db scope name = %q, want %q", got, "db")
	}
	scopes := Scopes()
	if len(scopes) != 2 {
		t.Fatalf("Scopes returned %d scopes, want 2", len(scopes))
	}
	if scopes[0].Name() != "db" || scopes[1].Name() != "server" {
		t.Fatalf("Scopes returned names [%q %q], want [db server]", scopes[0].Name(), scopes[1].Name())
	}
	mysql := db.Logger("mysql")
	if got := db.Logger("mysql"); got != mysql {
		t.Fatal("Printer did not return the existing scope printer")
	}

	Get().Logger().Warn("server ready")
	Get().Logger("worker").Error("worker failed")
	db.Logger().Error("database ready")
	db.Logger("mysql").Error("query failed")

	Get().Apply(WithFormat(TextFormat))
	db.Apply(WithOutput(StderrOutput))
}

func TestDefaultLoggerFollowsDefaultScope(t *testing.T) {
	resetDefault(t)

	Init("server", WithOutput(StdoutOutput))
	if got := log.Default().Writer(); got != os.Stdout {
		t.Fatalf("default logger writer = %T, want stdout", got)
	}

	Get().Apply(WithOutput(StderrOutput))
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("default logger writer after Apply = %T, want stderr", got)
	}

	db := Register("db", WithOutput(StdoutOutput))
	db.Apply(WithOutput(StdoutOutput))
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("named scope changed default logger writer to %T, want stderr", got)
	}
}

func TestScopeInheritsInitOptionsAndOverrides(t *testing.T) {
	resetDefault(t)

	Init("server",
		WithFields(log.String("service", "api")),
		WithOutput(StdoutOutput),
		WithLevel(log.LevelInfo),
	)
	db := Register("db", WithLevel(log.LevelError))

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

func TestAppendFieldsScopeIsolation(t *testing.T) {
	resetDefault(t)
	fields := make([]log.Field, 1, 4)
	fields[0] = log.String("service", "api")
	option := WithFields(fields...)
	Init("server", option)
	left, right := Register("left"), Register("right")
	left.Apply(AppendFields(log.String("tenant", "left")))
	right.Apply(AppendKeyValues("tenant", "right"))
	if !left.config.Fields[1].Equal(log.String("tenant", "left")) {
		t.Fatal("appending in another scope changed left's fields")
	}
	if len(Get().config.Fields) != 1 ||
		!Get().config.Fields[0].Equal(log.String("service", "api")) {
		t.Fatal("scope append changed the default scope's fields")
	}
}

func TestApplyPreservesCurrentScopeConfig(t *testing.T) {
	resetDefault(t)

	Init("server", WithFields(log.String("service", "api")), WithOutput(StdoutOutput))
	db := Register("db", WithLevel(log.LevelWarn))

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
	Init("server",
		WithOutput(FileOutput),
		WithFileDir(dir),
		WithLevel(log.LevelError),
	)
	p := Get().Logger()
	p.Debug("before apply")

	Get().Apply(WithLevel(log.LevelDebug))
	if got := Get().Logger(); got != p {
		t.Fatal("Apply replaced the managed Printer")
	}
	p.Debug("after apply")
	if err := Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dir + "/server.log")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "before apply") ||
		!strings.Contains(string(data), "after apply") {
		t.Fatalf("file output = %q", data)
	}
}

func TestApplyUpdatesFileWriterConfiguration(t *testing.T) {
	resetDefault(t)
	Init("server",
		WithOutput(FileOutput),
		WithFileDir(t.TempDir()),
		WithFileSize(1),
		WithFileBackups(1),
	)
	entry := Get().loggers[""]
	before := entry.Load().Writer().(*lumberjack.Logger)

	Get().Apply(
		WithFileSize(2),
		WithFileBackups(3),
		WithFileCompress(true),
	)
	after := entry.Load().Writer().(*lumberjack.Logger)
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
	Init("server", WithOutput(FileOutput), WithFileDir(firstDir))
	p := Get().Logger()
	p.Info("first file")
	Get().Apply(WithFileDir(secondDir))
	p.Info("second file")
	if err := Close(); err != nil {
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
	Init("server",
		WithOutput(FileOutput),
		WithFileDir(dir),
		WithFormat(JsonFormat),
		WithFields(log.DefaultFields...),
	)
	_, _, line, _ := runtime.Caller(0)
	Get().Logger().Info("caller")
	if err := Close(); err != nil {
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
	if record.Caller.Line != line+1 {
		t.Fatalf("caller line = %d, want %d", record.Caller.Line, line+1)
	}
}

func TestConcurrentApplyAndHeldPrinter(t *testing.T) {
	resetDefault(t)
	Init("server", WithOutput(FileOutput), WithFileDir(t.TempDir()))
	p := Get().Logger()
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
			Get().Apply(WithLevel(log.Level(i & 1)))
		}
	}()
	wg.Wait()
	if got := Get().Logger(); got != p {
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

	Init("server")
	server := Get().Logger("worker")
	db := Register("db")
	mysql := db.Logger("mysql")
	for _, scope := range Scopes() {
		if got := *scope.config.Format; got != JsonFormat {
			t.Fatalf("scope %s format = %v, want json", scope.Name(), got)
		}
		if got := *scope.config.File.Size; got != 128 {
			t.Fatalf("scope %s file size = %d, want 128", scope.Name(), got)
		}
		if got := *scope.config.File.Backups; got != 3 || !*scope.config.File.Compress {
			t.Fatalf("scope %s did not apply global file rotation flags", scope.Name())
		}
	}
	if got := *db.config.Level; got != log.LevelDebug {
		t.Fatalf("db level = %v, want debug", got)
	}

	server.Info("filtered by global error level")
	server.Error("global error")
	mysql.Debug("scope debug")
}

func TestGlobalFormatPreservesScopeOutput(t *testing.T) {
	resetDefault(t)
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	if err := fs.Parse([]string{"--log-format=json"}); err != nil {
		t.Fatal(err)
	}
	Init("server", WithFormat(TextFormat), WithOutput(StderrOutput))
	t.Cleanup(func() { _ = Close() })
	dir := t.TempDir()
	db := Register("db", WithFormat(TextFormat), WithOutput(FileOutput), WithFileDir(dir))
	mysql := db.Logger("mysql")
	Get().Logger("http")
	Get().Logger("grpc")
	if got := Get().loggers["http"].Load().Writer(); got != os.Stderr {
		t.Fatalf("http writer = %T, want stderr", got)
	}
	if got := Get().loggers["grpc"].Load().Writer(); got != os.Stderr {
		t.Fatalf("grpc writer = %T, want stderr", got)
	}
	if got := *Get().config.Format; got != JsonFormat {
		t.Fatalf("default format = %v, want json", got)
	}
	// Apply must retain CLI format precedence and the scope's file output.
	db.Apply(WithFormat(TextFormat), WithLevel(log.LevelDebug))
	mysql.Info("query completed")
	data, err := os.ReadFile(dir + "/db.mysql.log")
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("file log is not JSON: %s: %v", data, err)
	}
	if record["msg"] != "query completed" {
		t.Fatalf("unexpected file log: %s", data)
	}
}

func TestLogSetFormatPrecedence(t *testing.T) {
	resetDefault(t)
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	// Specificity determines precedence, even when scoped flags come first.
	if err := fs.Parse([]string{
		"--log-set=db.format=text",
		"--log-set=format=json",
		"--log-format=text",
	}); err != nil {
		t.Fatal(err)
	}
	Init("server")
	db := Register("db")
	cache := Register("cache")
	for _, scope := range []*Scope{Get(), cache} {
		if got := *scope.config.Format; got != JsonFormat {
			t.Fatalf("scope %s format = %v, want json", scope.Name(), got)
		}
	}
	db.Apply(WithFormat(JsonFormat))
	if got := *db.config.Format; got != TextFormat {
		t.Fatalf("db format = %v, want text", got)
	}
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

	Init("server")
	if got := log.Default().Writer(); got != os.Stderr {
		t.Fatalf("named default scope log-set was not applied last: got %T, want stderr", got)
	}
	_ = Register("db")
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
	Init("server")
	mustPanic(t, func() { Register("server") })
	db := Register("db", WithLevel(log.LevelWarn))
	mustPanic(t, func() { Register("db", WithLevel(log.LevelDebug)) })
	if Get("db") != db || *Get("db").config.Level != log.LevelWarn {
		t.Fatal("duplicate registration changed the existing scope")
	}
}

func TestEmptyNames(t *testing.T) {
	resetDefault(t)
	mustPanic(t, func() { Init("") })
	Init("server")
	mustPanic(t, func() { Register("") })
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

	Init("server")

	var wg sync.WaitGroup
	results := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			succeeded := false
			defer func() {
				_ = recover()
				results <- succeeded
			}()
			Register("db")
			succeeded = true
		}()
	}
	wg.Wait()
	close(results)
	var successes int
	for success := range results {
		if success {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("AddScope successes = %d, want 1", successes)
	}

	scope := Get("db")
	printers := make(chan log.Printer, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			printers <- scope.Logger("mysql")
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

func TestAddScopePanicOnDuplicate(t *testing.T) {
	resetDefault(t)

	Init("server")
	Register("db")

	mustPanic(t, func() {
		Register("db")
	})
}

func TestMissingScopePanicAndPrinterCreates(t *testing.T) {
	resetDefault(t)

	Init("server")

	mustPanic(t, func() {
		Get("missing")
	})
	if got := Get().Logger("missing"); got == nil {
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
