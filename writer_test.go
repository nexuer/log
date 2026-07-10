package log

import (
	"bytes"
	"testing"
)

type closeBuffer struct {
	bytes.Buffer
	closed bool
}

func (b *closeBuffer) Close() error {
	b.closed = true
	return nil
}

func TestMultiWriterWritesAndClosesAll(t *testing.T) {
	first := &closeBuffer{}
	second := &closeBuffer{}
	logger := New(MultiWriter(first, second))

	logger.Info("hello")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}

	want := "INFO msg=hello\n"
	if got := first.String(); got != want {
		t.Fatalf("first writer = %q, want %q", got, want)
	}
	if got := second.String(); got != want {
		t.Fatalf("second writer = %q, want %q", got, want)
	}
	if !first.closed || !second.closed {
		t.Fatalf("close state = (%v, %v), want both closed", first.closed, second.closed)
	}
}

func TestTryMultiWriterWritesAll(t *testing.T) {
	first := &closeBuffer{}
	second := &closeBuffer{}
	logger := New(TryMultiWriter(StrategyFirst, first, second))

	logger.Info("hello")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}

	want := "INFO msg=hello\n"
	if got := first.String(); got != want {
		t.Fatalf("first writer = %q, want %q", got, want)
	}
	if got := second.String(); got != want {
		t.Fatalf("second writer = %q, want %q", got, want)
	}
	if !first.closed || !second.closed {
		t.Fatalf("close state = (%v, %v), want both closed", first.closed, second.closed)
	}
}
