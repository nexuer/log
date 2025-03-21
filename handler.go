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
	key    string
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
	// groupPrefix is for the text handler only.
	// It holds the prefix for groups that were already pre-formatted.
	// A group will appear here when a call to WithGroup is followed by
	// a call to WithAttrs.
	groupPrefix string
	groups      []string // all groups started from WithGroup
	nOpenGroups int      // the number of groups opened in preformattedAttrs
	mu          *sync.Mutex
}

func newCommonHandler(json bool, opts HandlerOptions) *commonHandler {
	ch := &commonHandler{
		mu:   &sync.Mutex{},
		json: json,
	}
	//name = strings.TrimSpace(name)
	//if name == "" {
	//	return ch
	//}
	//if json {
	//	return ch.withFields([]Field{String(NameKey, name)}, true)
	//} else {
	//	ch.name = name + " | "
	//}
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

func (h *commonHandler) withFields(fields []Field, min bool) *commonHandler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(fields) == len(fields) {
		return h
	}
	h2 := h.clone()
	var state handleState
	state = h2.newHandleState(buffer.NewNonCap(), false, h.attrSep())
	//if min {
	//	state = h2.newHandleState(buffer.NewMin(), false, h.attrSep())
	//} else {
	//	state = h2.newHandleState(buffer.New(), false, h.attrSep())
	//}
	defer state.free()
	//pos := state.buf.Len()
	//state
	state.appendFields(context.Background(), "", fields, true)
	//state.appendFields2(context.Background(), fields, true)
	return h2
}

func (h *commonHandler) withFields2(fields []Field, min bool) *commonHandler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(fields) == len(fields) {
		return h
	}
	h2 := h.clone()
	var state handleState
	state = h2.newHandleState(buffer.NewNonCap(), false, h.attrSep())
	//if min {
	//	state = h2.newHandleState(buffer.NewMin(), false, h.attrSep())
	//} else {
	//	state = h2.newHandleState(buffer.New(), false, h.attrSep())
	//}
	defer state.free()
	//pos := state.buf.Len()
	//state
	//state.appendFields(context.Background(), "", fields, true)
	state.appendFields2(context.Background(), fields, true)
	return h2
}

func (s *handleState) appendFields2(ctx context.Context, fields []Field, isPreformat bool) bool {
	nonEmpty := false
	last := len(fields) - 1
	for i, field := range fields {
		if s.appendField(ctx, field, isPreformat, i == last) {
			nonEmpty = true
		}
	}
	return nonEmpty
}

func (s *handleState) appendField(ctx context.Context, field Field, isPreformat bool, isLast bool) bool {
	if rep := s.h.opts.Replacer; rep != nil && field.Value.Kind() != KindGroup {
		var gs []string
		if s.groups != nil {
			gs = *s.groups
		}
		// a.Value is resolved before calling ReplaceAttr, so the user doesn't have to.
		field = rep(gs, field)
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

			if !isLast {
				s.buf = buffer.NewNonCap()
			}
		} else {
			s.appendValue(v.Resolve(ctx))
		}
	}

	if field.Value.Kind() == KindGroup {

	} else {
		s.appendKey(field.Key)
		s.appendValue(field.Value)
	}

	return false
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
	freeBuf bool      // should buf be freed?
	sep     string    // separator to write before next key
	groups  *[]string // pool-allocated slice of active groups, for ReplaceAttr
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
		if needsQuoting(str) {
			*s.buf = strconv.AppendQuote(*s.buf, str)
		} else {
			_, _ = s.buf.WriteString(str)
		}
		//// text todo: needsQuoting
		//_, _ = s.buf.WriteString(str)
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
		a, kvs = kvsToField(kvs)
		s.appendFields(ctx, "", []Field{a}, false)
	}
}

func (h *commonHandler) handle(ctx context.Context, w io.Writer, level Level, msg string, fields ...any) error {
	state := h.newHandleState(buffer.New(), true, "")
	defer state.free()

	if h.json {
		state.appendByte('{')
	}

	//if len(h.name) > 0 {
	//	_, _ = state.buf.WriteString(h.name)
	//}

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
