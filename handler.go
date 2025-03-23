package log

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nexuer/log/internal/buffer"
)

type preformattedAttr struct {
	bytes  []byte
	valuer Valuer
}

type HandlerOptions struct {
	Name     string
	Replacer Replacer
}

type commonHandler struct {
	json              bool
	opts              HandlerOptions
	preformattedAttrs []preformattedAttr
	mu                *sync.Mutex
}

func newCommonHandler(json bool, opts HandlerOptions) *commonHandler {
	ch := &commonHandler{
		mu:   &sync.Mutex{},
		json: json,
		opts: opts,
	}
	if json && opts.Name != "" {
		return ch.withFields(context.Background(), []Field{String(NameKey, opts.Name)})
	}
	return ch
}

func (h *commonHandler) clone() *commonHandler {
	// We can't use assignment because we can't copy the mutex.
	return &commonHandler{
		json:              h.json,
		opts:              h.opts,
		preformattedAttrs: slices.Clip(h.preformattedAttrs),
		mu:                h.mu, // mutex shared among all clones of this handler
	}
}

func (h *commonHandler) withFields(ctx context.Context, fields []Field) *commonHandler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(fields) == len(fields) {
		return h
	}
	h2 := h.clone()
	var state handleState
	state = h2.newHandleState(buffer.NewNonCap(), false, h.attrSep())
	defer state.free()
	state.appendFields(ctx, fields, true, false)
	return h2
}

func (s *handleState) appendFields(ctx context.Context, fields []Field, isPreformat bool, isGroup bool) bool {
	nonEmpty := false
	for _, field := range fields {
		if s.appendField(ctx, field, isPreformat) {
			nonEmpty = true
		}
	}
	if isPreformat && !isGroup && s.buf.Len() > 0 {
		s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
			bytes: *s.buf,
		})
	}
	return nonEmpty
}

func (s *handleState) appendField(ctx context.Context, field Field, isPreformat bool) bool {
	if rep := s.h.opts.Replacer; rep != nil && field.Value.Kind() != KindGroup {
		var gs []string
		if s.groups != nil {
			gs = *s.groups
		}
		// a.Value is resolved before calling ReplaceAttr, so the user doesn't have to.
		field = rep(ctx, gs, field)
	}
	// Elide empty Attrs.
	if field.isEmpty() {
		return false
	}
	// Valuer
	if v := field.Value; v.Kind() == KindValuer {
		s.appendKey(field.Key)
		if isPreformat {
			s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
				bytes:  *s.buf,
				valuer: v.valuer(),
			})
			// new buffer
			s.buf = buffer.NewNonCap()
		} else {
			s.appendValue(v.Resolve(ctx))
		}
		return true
	}

	if field.Value.Kind() == KindGroup {
		fs := field.Value.group()
		// Output only non-empty groups.
		if len(fs) == 0 {
			return false
		}
		// Inline a group with an empty key.
		if field.Key != "" {
			s.openGroup(field.Key)
		}

		s.appendFields(ctx, fs, isPreformat, true)

		if field.Key != "" {
			s.closeGroup(field.Key)
		}
	} else {
		s.appendKey(field.Key)
		s.appendValue(field.Value)
	}

	return true
}

// attrSep returns the separator between attributes.
func (h *commonHandler) attrSep() string {
	if h.json {
		return ","
	}
	return " "
}

// handleState holds state for a single call to commonHandler.handle.
// The initial value of sep determines whether to emit a separator
// before the next key, after which it stays true.
type handleState struct {
	h       *commonHandler
	buf     *buffer.Buffer
	freeBuf bool           // should buf be freed?
	sep     string         // separator to write before next key
	prefix  *buffer.Buffer // for text: key prefix
	groups  *[]string      // pool-allocated slice of active groups, for ReplaceAttr
}

var groupPool = sync.Pool{New: func() any {
	s := make([]string, 0, 10)
	return &s
}}

func (h *commonHandler) newHandleState(buf *buffer.Buffer, freeBuf bool, sep string) handleState {
	s := handleState{
		h:       h,
		buf:     buf,
		freeBuf: freeBuf,
		sep:     sep,
		prefix:  buffer.New(),
	}
	// enable group
	if h.opts.Replacer != nil {
		s.groups = groupPool.Get().(*[]string)
	}
	return s
}

func (s *handleState) free() {
	if s.freeBuf {
		s.buf.Free()
	}
	if gs := s.groups; gs != nil {
		*gs = (*gs)[:0]
		groupPool.Put(gs)
	}
	s.prefix.Free()
}

// Separator for group names and keys.
const keyComponentSep = '.'

// openGroup starts a new group of attributes
// with the given name.
func (s *handleState) openGroup(name string) {
	if s.h.json {
		s.appendKey(name)
		_ = s.buf.WriteByte('{')
		s.sep = ""
	} else {
		_, _ = s.prefix.WriteString(name)
		_ = s.prefix.WriteByte(keyComponentSep)
	}
	// Collect group names for ReplaceAttr.
	if s.groups != nil {
		*s.groups = append(*s.groups, name)
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	if s.h.json {
		_ = s.buf.WriteByte('}')
	} else {
		(*s.prefix) = (*s.prefix)[:len(*s.prefix)-len(name)-1 /* for keyComponentSep */]
	}
	s.sep = s.h.attrSep()
	if s.groups != nil {
		*s.groups = (*s.groups)[:len(*s.groups)-1]
	}
}

func (s *handleState) appendKey(key string) {
	_, _ = s.buf.WriteString(s.sep)
	if s.h.json {
		s.appendString(key)
		_ = s.buf.WriteByte(':')
	} else {
		if s.prefix != nil && len(*s.prefix) > 0 {
			// TODO: optimize by avoiding allocation.
			s.appendString(bytesToString(*s.prefix) + key)
		} else {
			s.appendString(key)
		}
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
		// text
		if needsQuoting(str) {
			*s.buf = strconv.AppendQuote(*s.buf, str)
		} else {
			_, _ = s.buf.WriteString(str)
		}
	}
}

func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			// Quote anything except a backslash that would need quoting in a
			// JSON string, as well as space and '='
			if b != '\\' && (b == ' ' || b == '=' || !safeSet[b]) {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
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
	if len(s.h.preformattedAttrs) == 0 {
		return
	}
	for _, attr := range s.h.preformattedAttrs {
		if len(attr.bytes) > 0 {
			_, _ = s.buf.Write(attr.bytes)
		}
		if attr.valuer != nil {
			s.appendValue(attr.valuer(ctx).Resolve(ctx))
		}
	}
}

func (s *handleState) appendNonBuiltIns(ctx context.Context, kvs []any) {
	var a Field
	for len(kvs) > 0 {
		a, kvs = kvsToField(kvs)
		s.appendField(ctx, a, false)
	}
}

func (h *commonHandler) handle(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error {
	state := h.newHandleState(buffer.New(), true, "")
	defer state.free()

	if h.json {
		state.appendByte('{')
	}

	if !h.json && h.opts.Name != "" {
		_, _ = state.buf.WriteString("[")
		_, _ = state.buf.WriteString(h.opts.Name)
		_, _ = state.buf.WriteString("] ")
	}
	// Built-in attributes. They are not in a group.
	stateGroups := state.groups
	state.groups = nil // So ReplaceAttrs sees no groups instead of the pre groups.
	rep := h.opts.Replacer
	// level
	levelStr := level.String()
	if rep != nil {
		rep(ctx, nil, String(LevelKey, levelStr))
	}
	if h.json {
		state.appendKey(LevelKey)
		state.appendString(levelStr)
	} else {
		state.appendString(levelStr)
		state.sep = h.attrSep()
	}

	state.appendPreformattedAttrs(ctx)

	// message
	if rep != nil {
		rep(ctx, nil, String(MessageKey, msg))
	}

	if msg != "" {
		state.appendKey(MessageKey)
		state.appendString(msg)
	}

	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.

	state.appendNonBuiltIns(ctx, kvs)

	if h.json {
		state.appendByte('}')
	}

	state.appendByte('\n')

	if w == nil || w == io.Discard || w == Discard {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := w.Write(*state.buf)
	return err
}
