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

func Text(name ...string) Handler {
	n := ""
	if len(name) > 0 && name[0] != "" {
		n = name[0]
	}
	return &textHandler{
		handler: newCommonHandler(n, false),
	}
}

func (h *textHandler) With(kvs ...any) Handler {
	fields, ok := fieldsToAttrSlice(kvs)
	return &textHandler{
		handler: h.handler.withFields(fields, ok),
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
	case KindString:
		s.appendString(v.str())
	case KindTime:
		s.appendTime(v.time())
	case KindAny:
		switch anyVal := v.any.(type) {
		case error:
			s.appendString(anyVal.Error())
		case encoding.TextMarshaler:
			data, err := anyVal.MarshalText()
			if err != nil {
				return err
			}
			s.appendString(bytesToString(data))
		default:
			if bs, ok := byteSlice(v.any); ok {
				// As of Go 1.19, this only allocates for strings longer than 32 bytes.
				_, _ = s.buf.WriteString(strconv.Quote(string(bs)))
				return nil
			}
			s.appendString(fmt.Sprintf("%+v", anyVal))
		}
	default:
		*s.buf = v.append(*s.buf)
	}
	return nil
}
