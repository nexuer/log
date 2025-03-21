package log

import (
	"os"
	"testing"
)

func TestInitManager(t *testing.T) {
	logger := New(os.Stderr).With(DefaultFields...)
	logger.Info("hello world")
	Info("hello world")
	InitManager("manager", DefaultFields...)
	Info("hello world")
	M().Logger().Info("hello world")
}
