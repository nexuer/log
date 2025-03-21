package log

import (
	"os"
	"testing"
)

func TestMultiWriteCloser(t *testing.T) {
	logger := New(MultiWriter(os.Stdout, os.Stderr))

	logger.Info("hello world")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWriter(t *testing.T) {
	logger := New(TryMultiWriter(StrategyFirst, os.Stdout, os.Stderr))
	logger.Info("hello world")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
}
