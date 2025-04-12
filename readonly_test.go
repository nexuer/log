package log_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/nexuer/log"
)

func TestNewReadonly(t *testing.T) {
	l := log.NewReadonly(nil)

	l.Info("hello world")

	v := reflect.ValueOf(l).Elem().FieldByName("logger")
	if v.IsValid() {
		//z := v.Interface() // panic: reflect: call of reflect.Value.Interface on unexported type
		//_ = z
	}

	l = log.NewReadonly(log.New(os.Stderr, log.Text()).With(log.DefaultFields...))
	l.Info("hello world")
}
