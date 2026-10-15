package logmgr

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/nexuer/log"
)

func TestReleaseLifecycle(t *testing.T) {
	resetDefault(t)
	previous := log.Default()
	t.Cleanup(func() { log.SetDefault(previous) })
	Init("server")
	s := Register("db")
	p := s.Logger("mysql")
	if err := Remove("db", "mysql", "missing"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.loggers["mysql"]; ok {
		t.Fatal("logger registration survived removal")
	}
	if s.Logger("mysql") == p {
		t.Fatal("recreated logger reused the removed printer")
	}
	if err := Remove("db", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.loggers[""]; ok {
		t.Fatal("named scope default logger survived removal")
	}
	if err := Remove("server", ""); err != nil {
		t.Fatal(err)
	}
	if Get().loggers[""] == nil {
		t.Fatal("default logger was removed")
	}
	if err := Remove("server"); err == nil {
		t.Fatal("default scope was removed")
	}
	if err := Remove("db"); err != nil {
		t.Fatal(err)
	}
	if len(s.loggers) != 0 {
		t.Fatal("scope logger map was not cleared")
	}
	if _, ok := Lookup("db"); ok {
		t.Fatal("scope registration survived removal")
	}
	if err := Remove("missing"); err != nil {
		t.Fatal(err)
	}
	if fresh := Register("db"); fresh == s {
		t.Fatal("recreated scope reused removed scope")
	}
	server := Get()
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if len(Scopes()) != 0 || len(server.loggers) != 0 {
		t.Fatal("Close did not clear registrations")
	}
	mustPanic(t, func() { Get() })
	mustPanic(t, func() { Register("late") })
	mustPanic(t, func() { Init("again") })
	if err := Close(); err != nil {
		t.Fatal(err)
	}
}

type failingCloser struct {
	calls int
	err   error
}

func (*failingCloser) Write(p []byte) (int, error) { return len(p), nil }
func (w *failingCloser) Close() error              { w.calls++; return w.err }

func TestReleaseReturnsWriterErrors(t *testing.T) {
	previous := log.Default()
	t.Cleanup(func() { log.SetDefault(previous) })
	m, err := newManager("server")
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.Register("db")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("close failed")
	w := &failingCloser{err: want}
	s.Logger("mysql")
	s.loggers["mysql"].Store(log.New(w))
	if err := m.Remove("db", "mysql"); !errors.Is(err, want) {
		t.Fatalf("Remove = %v", err)
	}
	s.loggers[""].Store(log.New(w))
	if err := m.Close(); !errors.Is(err, want) {
		t.Fatalf("Close = %v", err)
	}
	if w.calls != 2 {
		t.Fatalf("Close calls = %d, want 2", w.calls)
	}
}

func TestPrinterMethodsAndHeldConfiguration(t *testing.T) {
	resetDefault(t)
	previous := log.Default()
	t.Cleanup(func() { log.SetDefault(previous) })
	dir := t.TempDir()
	Init("server", WithOutput(FileOutput), WithFileDir(dir), WithFormat(JsonFormat), WithLevel(log.LevelError))
	t.Cleanup(func() { _ = Close() })
	s := Get()
	p := s.Logger("http")
	p.Debug("filtered")
	s.Apply(WithLevel(log.LevelDebug), WithKeyValues("service", "api"), AppendKeyValues("component", "http"))
	p.Debug("debug")
	p.Debugf("debug-%d", 1)
	p.Info("info")
	p.Infof("info-%d", 1)
	p.Warn("warn")
	p.Warnf("warn-%d", 1)
	p.Error("error")
	p.Errorf("error-%d", 1)
	if n, err := p.Write([]byte("write")); n != 5 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	data, err := os.ReadFile(dir + "/server.http.log")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(rows) != 9 {
		t.Fatalf("record count = %d, want 9: %s", len(rows), data)
	}
	for _, row := range rows {
		var record map[string]any
		if err := json.Unmarshal([]byte(row), &record); err != nil {
			t.Fatal(err)
		}
		if record["service"] != "api" || record["component"] != "http" {
			t.Fatalf("missing applied fields: %s", row)
		}
	}
}
