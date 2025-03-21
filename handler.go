package log

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nexuer/log/internal/buffer"
)

type preformattedAttr struct {
	bytes  []byte
	key    string
	valuer Valuer
}

type commonHandler struct {
	json              bool
	name              string
	mu                *sync.Mutex
	preformattedAttrs []preformattedAttr
}

func newCommonHandler(name string, json bool) *commonHandler {
	ch := &commonHandler{
		mu:   &sync.Mutex{},
		json: json,
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ch
	}
	if json {
		return ch.withFields([]Field{String(NameKey, name)}, true)
	} else {
		ch.name = name + " | "
	}
	return ch
}

func (h *commonHandler) clone() *commonHandler {
	// We can't use assignment because we can't copy the mutex.
	return &commonHandler{
		name:              h.name,
		json:              h.json,
		preformattedAttrs: slices.Clip(h.preformattedAttrs),
		mu:                h.mu, // mutex shared among all clones of this handler
	}
}

// attrSep returns the separator between attributes.
func (h *commonHandler) attrSep() string {
	if h.json {
		return ","
	}
	return " "
}

func (h *commonHandler) withFields(fields []Field, min bool) *commonHandler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(fields) == len(fields) {
		return h
	}
	h2 := h.clone()
	var state handleState
	if min {
		state = h2.newHandleState(buffer.NewMin(), false, h.attrSep())
	} else {
		state = h2.newHandleState(buffer.New(), false, h.attrSep())
	}
	defer state.free()
	state.appendFields(context.Background(), "", fields, true)
	return h2
}

// handleState holds state for a single call to commonHandler.handle.
// The initial value of sep determines whether to emit a separator
// before the next key, after which it stays true.
type handleState struct {
	h       *commonHandler
	buf     *buffer.Buffer
	freeBuf bool   // should buf be freed?
	sep     string // separator to write before next key
}

func (h *commonHandler) newHandleState(buf *buffer.Buffer, freeBuf bool, sep string) handleState {
	return handleState{
		h:       h,
		buf:     buf,
		sep:     sep,
		freeBuf: freeBuf,
	}
}

func (s *handleState) free() {
	if s.freeBuf {
		s.buf.Free()
	}
}

// Separator for group names and keys.
const keyComponentSep = '.'

// openGroup starts a new group of attributes
// with the given name.
func (s *handleState) openGroup(name string) {
	if name == "" {
		return
	}
	if s.h.json {
		s.appendKey(name)
		_ = s.buf.WriteByte('{')
		s.sep = ""
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	if name == "" {
		return
	}
	if s.h.json {
		_ = s.buf.WriteByte('}')
	}
	s.sep = s.h.attrSep()
}

func (s *handleState) appendKey(key string) {
	_, _ = s.buf.WriteString(s.sep)
	s.appendString(key)
	if s.h.json {
		_ = s.buf.WriteByte(':')
	} else {
		_ = s.buf.WriteByte('=')
	}
	s.sep = s.h.attrSep()
}

func (s *handleState) appendString(str string) {
	if s.h.json {
		_ = s.buf.WriteByte('"')
		*s.buf = appendEscapedJSONString(*s.buf, str)
		_ = s.buf.WriteByte('"')
	} else {
		// text todo: needsQuoting
		_, _ = s.buf.WriteString(str)
	}
}

func (s *handleState) appendFields(ctx context.Context, group string, fields []Field, isPreformat bool) {
	lastAttrIndex := len(fields) - 1
	s.openGroup(group)
	for i, a := range fields {
		if a.isEmpty() {
			continue
		}
		key := a.Key
		if !s.h.json && group != "" {
			key = group + "." + key
		}
		// Special case: Valuer.
		if v := a.Value; v.Kind() == KindValuer {
			if isPreformat {
				if s.buf.Len() > 0 {
					s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
						bytes: *s.buf,
					})

					if i < lastAttrIndex {
						// hasDynamic
						s.buf = buffer.New()
					}

				}

				s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
					key:    key,
					valuer: a.Value.valuer(),
				})

			} else {
				s.appendKey(key)
				s.appendValue(v.Resolve(ctx))
			}
			continue
		}

		if a.Value.Kind() == KindGroup {
			groupValue := a.Value.Group()
			if len(groupValue) == 0 {
				continue
			}
			s.appendFields(ctx, key, a.Value.Group(), isPreformat)
		} else {
			s.appendKey(key)
			s.appendValue(a.Value)
		}
	}
	s.closeGroup(group)
	if isPreformat && group == "" {
		if s.buf.Len() > 0 {
			s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
				bytes: *s.buf,
			})
		}
	}
}

func (s *handleState) appendValue(v Value) {
	defer func() {
		if r := recover(); r != nil {
			// If it panics with a nil pointer, the most likely cases are
			// an encoding.TextMarshaler or error fails to guard against nil,
			// in which case "<nil>" seems to be the feasible choice.
			//
			// Adapted from the code in fmt/print.go.
			if v := reflect.ValueOf(v.any); v.Kind() == reflect.Pointer && v.IsNil() {
				s.appendString("<nil>")
				return
			}

			// Otherwise just print the original panic message.
			s.appendString(fmt.Sprintf("!PANIC: %v", r))
		}
	}()

	var err error
	if s.h.json {
		err = appendJSONValue(s, v)
	} else {
		err = appendTextValue(s, v)
	}
	if err != nil {
		s.appendError(err)
	}
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendTime(t time.Time) {
	if s.h.json {
		appendJSONTime(s, t)
	} else {
		*s.buf = appendRFC3339Millis(*s.buf, t)
	}
}

func appendRFC3339Millis(b []byte, t time.Time) []byte {
	// Format according to time.RFC3339Nano since it is highly optimized,
	// but truncate it to use millisecond resolution.
	// Unfortunately, that format trims trailing 0s, so add 1/10 millisecond
	// to guarantee that there are exactly 4 digits after the period.
	const prefixLen = len("2006-01-02T15:04:05.000")
	n := len(b)
	t = t.Truncate(time.Millisecond).Add(time.Millisecond / 10)
	b = t.AppendFormat(b, time.RFC3339Nano)
	b = append(b[:n+prefixLen], b[n+prefixLen+1:]...) // drop the 4th digit
	return b
}

func (s *handleState) appendByte(c byte) {
	_ = s.buf.WriteByte(c)
}

func (s *handleState) appendPreformattedAttrs(ctx context.Context) {
	if s.h.preformattedAttrs == nil {
		return
	}
	prevBytes := false
	for _, attr := range s.h.preformattedAttrs {
		if attr.valuer != nil {
			s.appendKey(attr.key)
			s.appendValue(attr.valuer(ctx))
			prevBytes = false
		} else if len(attr.bytes) > 0 {
			_, _ = s.buf.Write(attr.bytes)
			if prevBytes {
				s.sep = ""
			}
			prevBytes = true
		}
	}
	s.sep = s.h.attrSep()
}

func (s *handleState) appendByAnyFields(ctx context.Context, kvs []any) {
	var a Field
	for len(kvs) > 0 {
		a, kvs = fieldsToAttr(kvs)
		s.appendFields(ctx, "", []Field{a}, false)
	}
}

func (h *commonHandler) handle(ctx context.Context, w io.Writer, level Level, msg string, fields ...any) error {
	state := h.newHandleState(buffer.New(), true, "")
	defer state.free()

	if h.json {
		state.appendByte('{')
	}

	if len(h.name) > 0 {
		state.appendString(h.name)
	}

	// level
	if h.json {
		state.appendKey(LevelKey)
		state.appendString(level.String())
	} else {
		state.appendString(level.String())
		state.sep = h.attrSep()
	}

	state.appendPreformattedAttrs(ctx)
	// message
	if msg != "" {
		state.appendKey(MessageKey)
		state.appendString(msg)
	}

	state.appendByAnyFields(ctx, fields)

	if h.json {
		state.appendByte('}')
	}

	state.appendByte('\n')

	// Benchmark test
	if w == nil || w == io.Discard || w == discard {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := w.Write(*state.buf)
	return err
}
