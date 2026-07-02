package log

import (
	"bytes"
	"os"
	"testing"
)

type closeBuffer struct {
	bytes.Buffer
}

func (b *closeBuffer) Close() error {
	return nil
}

func TestMultiWriteCloser(t *testing.T) {
	logger := New(MultiWriter(&closeBuffer{}, &closeBuffer{}))

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
