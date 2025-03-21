package log

import (
	"fmt"
	"testing"
	"time"
)

func TestAppendFields(t *testing.T) {
	ch := newCommonHandler(false, HandlerOptions{
		Replacer: func(groups []string, field Field) Field {
			fmt.Println(groups, field)
			return field
		},
	})

	fields := []Field{
		String("key", "value"),
		Any("ts", ValuerValue(Timestamp(time.DateTime))),
		String("key1", "value1"),
	}

	ch.withFields2(fields, false)
}
