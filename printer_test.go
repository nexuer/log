package log_test

import (
	"bytes"
	"testing"

	"github.com/nexuer/log"
)

func TestNewPrinterUsesProvidedLogger(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf))

	printer.Info("hello")

	want := "INFO msg=hello\n"
	if got := buf.String(); got != want {
		t.Fatalf("printer output = %q, want %q", got, want)
	}
}

func TestPrinterWrite(t *testing.T) {
	var buf bytes.Buffer
	printer := log.NewPrinter(log.New(&buf))

	n, err := printer.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello") {
		t.Fatalf("Write returned n = %d, want %d", n, len("hello"))
	}

	want := "INFO msg=hello\n"
	if got := buf.String(); got != want {
		t.Fatalf("printer output = %q, want %q", got, want)
	}
}
