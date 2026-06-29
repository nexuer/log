package log_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/nexuer/log"
)

func TestNewPrinter(t *testing.T) {
	l := log.NewPrinter(nil)

	l.Info("hello world")

	v := reflect.ValueOf(l).Elem().FieldByName("logger")
	if v.IsValid() {
		//z := v.Interface() // panic: reflect: call of reflect.Value.Interface on unexported type
		//_ = z
	}

	l = log.NewPrinter(log.New(os.Stderr, log.Text()).With(log.DefaultFields...))
	l.Info("hello world")
}
