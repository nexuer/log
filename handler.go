package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	// Name identifies the logger. JSON handlers emit it as the built-in logger
	// field; text handlers render it as a [name] prefix.
	Name string
	// Replacer can transform or remove user fields and the built-in level, msg,
	// and logger fields.
	Replacer Replacer
}

type commonHandler struct {
	json              bool
	opts              HandlerOptions
	preformattedAttrs []preformattedAttr
	groupPrefix       string
	groups            []string
	nOpenGroups       int
	mu                *sync.Mutex
}

func newCommonHandler(json bool, opts HandlerOptions) *commonHandler {
	ch := &commonHandler{
		mu:   &sync.Mutex{},
		json: json,
		opts: opts,
	}
	return ch
}

func (h *commonHandler) clone() *commonHandler {
	// We can't use assignment because we can't copy the mutex.
	return &commonHandler{
		json:              h.json,
		opts:              h.opts,
		preformattedAttrs: slices.Clip(h.preformattedAttrs),
		groupPrefix:       h.groupPrefix,
		groups:            slices.Clip(h.groups),
		nOpenGroups:       h.nOpenGroups,
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
	var buf buffer.Buffer
	state := h2.newHandleState(&buf, false, h.attrSep())
	defer state.free()
	_, _ = state.prefix.WriteString(h.groupPrefix)
	if h.hasPreformattedAttrs() {
		state.sep = h.attrSep()
		if h.json && h.lastPreformattedByte() == '{' {
			state.sep = ""
		}
	}
	state.openGroups()
	if state.appendFields(ctx, fields, true, false) {
		h2.groupPrefix = state.prefix.String()
		h2.nOpenGroups = len(h2.groups)
	}
	return h2
}

func (h *commonHandler) withGroup(name string) *commonHandler {
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *commonHandler) hasPreformattedAttrs() bool {
	return len(h.preformattedAttrs) > 0
}

func (h *commonHandler) lastPreformattedByte() byte {
	for i := len(h.preformattedAttrs) - 1; i >= 0; i-- {
		bs := h.preformattedAttrs[i].bytes
		if len(bs) > 0 {
			return bs[len(bs)-1]
		}
	}
	return 0
}

func (s *handleState) appendFields(ctx context.Context, fields []Field, isPreformat bool, isGroup bool) bool {
	nonEmpty := false
	for _, field := range fields {
		if s.appendField(ctx, field, isPreformat) {
			nonEmpty = true
		}
	}
	if isPreformat && !isGroup && nonEmpty && s.buf.Len() > 0 {
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
		// The value is resolved before calling Replacer, so the user doesn't have to.
		field = rep(ctx, gs, field)
	}
	return s.appendFieldValue(ctx, field, isPreformat)
}

// appendFieldValue appends a field after any replacement has been applied.
func (s *handleState) appendFieldValue(ctx context.Context, field Field, isPreformat bool) bool {
	// Elide empty Attrs.
	if field.isEmpty() {
		return false
	}
	// Valuer
	if v := field.Value; v.Kind() == KindValuer {
		s.appendKey(field.Key)
		if isPreformat {
			valuer := v.valuer()
			if valuer == nil {
				valuer = nilValuer
			}
			s.h.preformattedAttrs = append(s.h.preformattedAttrs, preformattedAttr{
				bytes:  *s.buf,
				valuer: valuer,
			})
			// Keep the stored bytes owned by the segment and reuse the slice header.
			*s.buf = nil
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
		pos := s.buf.Len()
		sep := s.sep
		prefixLen := len(*s.prefix)
		groupLen := 0
		if s.groups != nil {
			groupLen = len(*s.groups)
		}

		// Inline a group with an empty key.
		if field.Key != "" {
			s.openGroup(field.Key)
		}

		if !s.appendFields(ctx, fs, isPreformat, true) {
			s.buf.SetLen(pos)
			s.sep = sep
			*s.prefix = (*s.prefix)[:prefixLen]
			if s.groups != nil {
				*s.groups = (*s.groups)[:groupLen]
			}
			return false
		}

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
	groups  *[]string      // pool-allocated slice of active groups, for Replacer
	message Field          // replaced built-in message, emitted after accumulated fields
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
		*s.groups = append(*s.groups, h.groups[:h.nOpenGroups]...)
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

func (s *handleState) openGroups() {
	for _, name := range s.h.groups[s.h.nOpenGroups:] {
		s.openGroup(name)
	}
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
	// Collect group names for Replacer.
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

func (s *handleState) appendTimestamp(t time.Time, layout string) {
	if layout == time.RFC3339 || layout == time.RFC3339Nano {
		if s.h.json {
			s.appendByte('"')
		}
		*s.buf = t.AppendFormat(*s.buf, layout)
		if s.h.json {
			s.appendByte('"')
		}
		return
	}
	formatted := buffer.New()
	defer formatted.Free()
	*formatted = t.AppendFormat(*formatted, layout)
	s.appendString(bytesToString(*formatted))
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

func (s *handleState) appendPreformattedAttrs(ctx context.Context) bool {
	if len(s.h.preformattedAttrs) == 0 {
		return false
	}
	for _, attr := range s.h.preformattedAttrs {
		if len(attr.bytes) > 0 {
			_, _ = s.buf.Write(attr.bytes)
		}
		if attr.valuer != nil {
			s.appendValue(resolvePreformattedValuer(ctx, attr.valuer))
		}
	}
	s.sep = s.h.attrSep()
	return true
}

var nilValuer = Valuer(func(context.Context) Value { return AnyValue(nil) })

func resolvePreformattedValuer(ctx context.Context, valuer Valuer) (rv Value) {
	defer func() {
		if recover() != nil {
			rv = AnyValue(fmt.Errorf("valuer panicked\n%s", stack(3, 5)))
		}
	}()
	return valuer(ctx).Resolve(ctx)
}

func (s *handleState) appendNonBuiltIns(ctx context.Context, kvs []any) {
	nOpenGroups := s.h.nOpenGroups
	s.appendPreformattedAttrs(ctx)
	messageAppended := !s.h.json || s.h.nOpenGroups == 0
	if messageAppended {
		s.appendMessage(ctx)
	}

	if len(kvs) > 0 {
		_, _ = s.prefix.WriteString(s.h.groupPrefix)
		pos := s.buf.Len()
		s.openGroups()
		nOpenGroups = len(s.h.groups)

		nonEmpty := false
		var a Field
		for len(kvs) > 0 {
			if attr, ok := kvs[0].(slog.Attr); ok {
				kvs = kvs[1:]
				if s.appendSlogAttr(ctx, attr) {
					nonEmpty = true
				}
				continue
			}
			a, kvs = kvsToField(kvs)
			if s.appendField(ctx, a, false) {
				nonEmpty = true
			}
		}
		if !nonEmpty {
			s.buf.SetLen(pos)
			nOpenGroups = s.h.nOpenGroups
		}
	}

	if s.h.json {
		for range s.h.groups[:nOpenGroups] {
			s.appendByte('}')
		}
		if !messageAppended {
			s.appendMessage(ctx)
		}
		s.appendByte('}')
	}
}

func (s *handleState) appendMessage(ctx context.Context) {
	if s.message.isEmpty() {
		return
	}
	groups := s.groups
	s.groups = nil
	s.appendFieldValue(ctx, s.message, false)
	s.groups = groups
	s.message = Field{}
}

func (h *commonHandler) newRecordState(ctx context.Context, level, msg string) handleState {
	state := h.newHandleState(buffer.New(), true, "")

	if h.json {
		state.appendByte('{')
	}

	// Built-in attributes. They are not in a group.
	stateGroups := state.groups
	state.groups = nil // Built-in fields are always outside user groups.
	levelField := h.replaceBuiltIn(ctx, String(LevelKey, level))
	msgField := Field{}
	if msg != "" {
		msgField = h.replaceBuiltIn(ctx, String(MessageKey, msg))
	}
	nameField := Field{}
	if h.opts.Name != "" {
		nameField = h.replaceBuiltIn(ctx, String(NameKey, h.opts.Name))
	}

	// Preserve the text handler's [name] prefix when logger remains a string.
	if !h.json && nameField.Key == NameKey && nameField.Value.Kind() == KindString {
		_, _ = state.buf.WriteString("[")
		_, _ = state.buf.WriteString(nameField.Value.str())
		_, _ = state.buf.WriteString("] ")
		nameField = Field{}
	}
	if !nameField.isEmpty() {
		state.appendFieldValue(ctx, nameField, false)
	}

	if !levelField.isEmpty() {
		if !h.json && levelField.Key == LevelKey {
			_, _ = state.buf.WriteString(state.sep)
			value := levelField.Value
			if value.Kind() == KindValuer {
				value = value.Resolve(ctx)
			}
			state.appendValue(value)
			state.sep = h.attrSep()
		} else {
			state.appendFieldValue(ctx, levelField, false)
		}
	}
	state.message = msgField

	state.groups = stateGroups // Restore groups passed to Replacer.
	return state
}

func (h *commonHandler) replaceBuiltIn(ctx context.Context, field Field) Field {
	if h.opts.Replacer == nil {
		return field
	}
	return h.opts.Replacer(ctx, nil, field)
}

func (h *commonHandler) writeRecord(w io.Writer, state *handleState) error {
	state.appendByte('\n')

	if w == nil || w == io.Discard || w == Discard {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	n, err := w.Write(*state.buf)
	if err == nil && n != len(*state.buf) {
		return io.ErrShortWrite
	}
	return err
}

func (h *commonHandler) handle(ctx context.Context, w io.Writer, level Level, msg string, kvs ...any) error {
	state := h.newRecordState(ctx, level.String(), msg)
	defer state.free()

	state.appendNonBuiltIns(ctx, kvs)
	return h.writeRecord(w, &state)
}
