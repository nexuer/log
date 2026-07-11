package log

import (
	"context"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/nexuer/log/internal/buffer"
)

type textHandler struct {
	handler *commonHandler
}

func Text(opts ...*HandlerOptions) Handler {
	opt := new(HandlerOptions)
	if len(opts) > 0 && opts[0] != nil {
		opt = opts[0]
	}
	return &textHandler{
		handler: newCommonHandler(false, *opt),
	}
}

func (h *textHandler) WithFields(ctx context.Context, fields ...Field) Handler {
	return &textHandler{
		handler: h.handler.withFields(ctx, fields),
	}
}

func (h *textHandler) WithGroup(name string) Handler {
	return &textHandler{
		handler: h.handler.withGroup(name),
	}
}

func (h *textHandler) Handle(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error {
	return h.handler.handle(ctx, w, level, msg, kvs...)
}

// byteSlice returns its argument as a []byte if the argument's
// underlying type is []byte, along with a second return value of true.
// Otherwise it returns nil, false.
func byteSlice(a any) ([]byte, bool) {
	if bs, ok := a.([]byte); ok {
		return bs, true
	}
	// Like Printf's %s, we allow both the slice type and the byte element type to be named.
	t := reflect.TypeOf(a)
	if t != nil && t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf(a).Bytes(), true
	}
	return nil, false
}

func bytesToString(data []byte) string {
	return unsafe.String(unsafe.SliceData(data), len(data))
}

func appendTextValue(s *handleState, v Value) error {
	switch v.Kind() {
	case KindSource:
		if source, ok := v.callerSource(); ok {
			appendTextSource(s, &source)
		} else if v.any != nil {
			appendTextSource(s, v.source())
		} else {
			_, _ = s.buf.WriteString("<nil>")
		}
	case KindString:
		if t, layout, ok := v.timestamp(); ok {
			s.appendTimestamp(t, layout)
		} else {
			s.appendString(v.str())
		}
	case KindTime:
		s.appendTime(v.time())
	case KindAny:
		if e, ok := v.any.(error); ok {
			if e != nil {
				s.appendString(e.Error())
			} else {
				s.appendString("<nil>")
			}
			return nil
		}

		if tm, ok := v.any.(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err != nil {
				return err
			}
			s.appendString(bytesToString(data))
			return nil
		}

		if bs, ok := byteSlice(v.any); ok {
			// As of Go 1.19, this only allocates for strings longer than 32 bytes.
			_, _ = s.buf.WriteString(strconv.Quote(string(bs)))
			return nil
		}
		if !appendTextSlice(s, v.any) {
			appendTextAny(s, v.any)
		}
	default:
		*s.buf = v.append(*s.buf)
	}
	return nil
}

func appendTextAny(s *handleState, value any) {
	formatted := buffer.New()
	defer formatted.Free()
	*formatted = fmt.Appendf(*formatted, "%+v", value)
	s.appendString(bytesToString(*formatted))
}

func appendTextSlice(s *handleState, value any) bool {
	switch value.(type) {
	case []int, []int64, []uint64, []float64, []bool, []string, []time.Time:
	default:
		return false
	}

	formatted := buffer.New()
	defer formatted.Free()
	*formatted = append(*formatted, '[')

	switch values := value.(type) {
	case []int:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = strconv.AppendInt(*formatted, int64(value), 10)
		}
	case []int64:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = strconv.AppendInt(*formatted, value, 10)
		}
	case []uint64:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = strconv.AppendUint(*formatted, value, 10)
		}
	case []float64:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = strconv.AppendFloat(*formatted, value, 'g', -1, 64)
		}
	case []bool:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = strconv.AppendBool(*formatted, value)
		}
	case []string:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = append(*formatted, value...)
		}
	case []time.Time:
		for i, value := range values {
			if i > 0 {
				*formatted = append(*formatted, ' ')
			}
			*formatted = append(*formatted, value.String()...)
		}
	}

	*formatted = append(*formatted, ']')
	s.appendString(bytesToString(*formatted))
	return true
}

func appendTextSource(s *handleState, source *Source) {
	_, _ = s.buf.WriteString(source.File)
	_, _ = s.buf.WriteString(":")
	*s.buf = strconv.AppendInt(*s.buf, int64(source.Line), 10)
}
