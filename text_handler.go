package log

import (
	"context"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"unsafe"
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
		if v.any != nil {
			source := v.source()
			_, _ = s.buf.WriteString(source.File)
			_, _ = s.buf.WriteString(":")
			*s.buf = strconv.AppendInt(*s.buf, int64(source.Line), 10)
		} else {
			_, _ = s.buf.WriteString("<nil>")
		}
	case KindString:
		s.appendString(v.str())
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
		s.appendString(fmt.Sprintf("%+v", v.Any()))
	default:
		*s.buf = v.append(*s.buf)
	}
	return nil
}
