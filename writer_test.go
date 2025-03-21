package log

import (
	"os"
	"testing"
)

func TestMultiWriteCloser(t *testing.T) {
	logger := New(MultiWriteCloser(os.Stdout, os.Stderr))

	logger.Info("hello world")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWriter(t *testing.T) {
	logger := New(TryMultiWriteCloser(StrategyFirst, os.Stdout, os.Stderr))
	logger.Info("hello world")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
}
