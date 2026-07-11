package log

import (
	"context"
	"io"
	"log/slog"
	"slices"
)

type slogAttrSegment struct {
	groups []string
	attrs  []slog.Attr
}

type slogHandler struct {
	base     *commonHandler
	common   *commonHandler
	w        io.Writer
	level    Level
	ctx      context.Context
	groups   []string
	segments []slogAttrSegment
	lazy     bool
}

// NewSlogHandler returns a slog.Handler snapshot of logger's current output,
// level, handler, accumulated fields, groups, and context. Later changes to
// logger do not affect the returned handler.
//
// If logger is nil, NewSlogHandler uses the package default Logger.
func NewSlogHandler(logger *Logger) slog.Handler {
	if logger == nil {
		logger = defaultLogger.Load()
		logger = logger.WithContext(adjustCallerDepth(logger.ctx, -1))
	}
	switch h := logger.handler.(type) {
	case *jsonHandler:
		return newBuiltinSlogHandler(h.handler, logger.Writer(), logger.level, logger.ctx)
	case *textHandler:
		return newBuiltinSlogHandler(h.handler, logger.Writer(), logger.level, logger.ctx)
	default:
		return &loggerSlogHandler{logger: logger.clone()}
	}
}

func newBuiltinSlogHandler(common *commonHandler, w io.Writer, level Level, ctx context.Context) *slogHandler {
	return &slogHandler{
		base:   common,
		common: common,
		w:      w,
		level:  level,
		ctx:    ctx,
		groups: slices.Clone(common.groups),
	}
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Enable(Level(level))
}

func (h *slogHandler) Handle(ctx context.Context, record slog.Record) error {
	// slog reaches the handler two frames sooner than Logger's convenience methods.
	ctx = adjustCallerDepth(mergeCallerDepth(ctx, h.ctx), -2)
	if h.lazy {
		return h.handleLazy(ctx, record)
	}
	return h.common.handleSlogRecord(ctx, h.w, record)
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	owned := slices.Clone(attrs)
	h2.segments = append(h2.segments, slogAttrSegment{
		groups: slices.Clone(h.groups),
		attrs:  owned,
	})

	if h2.lazy || hasSlogLogValuer(owned) {
		h2.lazy = true
		return h2
	}
	fields := make([]Field, 0, len(owned))
	for _, attr := range owned {
		if field := slogAttrToField(attr); !field.isEmpty() {
			fields = append(fields, field)
		}
	}
	h2.common = h.common.withFields(context.Background(), fields)
	return h2
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	if !h2.lazy {
		h2.common = h.common.withGroup(name)
	}
	return h2
}

func (h *slogHandler) clone() *slogHandler {
	return &slogHandler{
		base:     h.base,
		common:   h.common,
		w:        h.w,
		level:    h.level,
		ctx:      h.ctx,
		groups:   slices.Clone(h.groups),
		segments: slices.Clone(h.segments),
		lazy:     h.lazy,
	}
}

func hasSlogLogValuer(attrs []slog.Attr) bool {
	for _, attr := range attrs {
		switch attr.Value.Kind() {
		case slog.KindLogValuer:
			return true
		case slog.KindGroup:
			if hasSlogLogValuer(attr.Value.Group()) {
				return true
			}
		}
	}
	return false
}

func (h *commonHandler) handleSlogRecord(ctx context.Context, w io.Writer, record slog.Record) error {
	state := h.newRecordState(ctx, Level(record.Level).String(), record.Message)
	defer state.free()

	nOpenGroups := h.nOpenGroups
	state.appendPreformattedAttrs(ctx)
	messageAppended := !h.json || h.nOpenGroups == 0
	if messageAppended {
		state.appendMessage(ctx)
	}
	if record.NumAttrs() > 0 {
		_, _ = state.prefix.WriteString(h.groupPrefix)
		pos := state.buf.Len()
		state.openGroups()
		nOpenGroups = len(h.groups)
		nonEmpty := false
		record.Attrs(func(attr slog.Attr) bool {
			if state.appendSlogAttr(ctx, attr) {
				nonEmpty = true
			}
			return true
		})
		if !nonEmpty {
			state.buf.SetLen(pos)
			nOpenGroups = h.nOpenGroups
		}
	}

	if h.json {
		for range h.groups[:nOpenGroups] {
			state.appendByte('}')
		}
		if !messageAppended {
			state.appendMessage(ctx)
		}
		state.appendByte('}')
	}
	return h.writeRecord(w, &state)
}

func (h *slogHandler) handleLazy(ctx context.Context, record slog.Record) error {
	state := h.base.newRecordState(ctx, Level(record.Level).String(), record.Message)
	defer state.free()
	state.appendPreformattedAttrs(ctx)

	current := h.base.groups[:h.base.nOpenGroups]
	for _, segment := range h.segments {
		current = appendSlogAttrsAtPath(&state, ctx, current, segment.groups, segment.attrs)
	}
	messageAppended := !h.base.json || len(current) == 0
	if messageAppended {
		state.appendMessage(ctx)
	}

	pos := state.buf.Len()
	sep := state.sep
	transitionSlogGroups(&state, current, h.groups)
	nonEmpty := false
	record.Attrs(func(attr slog.Attr) bool {
		if state.appendSlogAttr(ctx, attr) {
			nonEmpty = true
		}
		return true
	})
	if nonEmpty {
		current = h.groups
	} else {
		transitionSlogGroups(&state, h.groups, current)
		state.buf.SetLen(pos)
		state.sep = sep
	}

	transitionSlogGroups(&state, current, nil)
	if h.base.json {
		if !messageAppended {
			state.appendMessage(ctx)
		}
		state.appendByte('}')
	}
	return h.base.writeRecord(h.w, &state)
}

func appendSlogAttrsAtPath(state *handleState, ctx context.Context, current, target []string, attrs []slog.Attr) []string {
	pos := state.buf.Len()
	sep := state.sep
	transitionSlogGroups(state, current, target)
	nonEmpty := false
	for _, attr := range attrs {
		if state.appendSlogAttr(ctx, attr) {
			nonEmpty = true
		}
	}
	if nonEmpty {
		return target
	}
	transitionSlogGroups(state, target, current)
	state.buf.SetLen(pos)
	state.sep = sep
	return current
}

func transitionSlogGroups(state *handleState, from, to []string) {
	common := 0
	for common < len(from) && common < len(to) && from[common] == to[common] {
		common++
	}
	for i := len(from) - 1; i >= common; i-- {
		state.closeGroup(from[i])
	}
	for _, group := range to[common:] {
		state.openGroup(group)
	}
}

var _ slog.Handler = (*slogHandler)(nil)

// loggerSlogHandler is the compatibility path for user-defined Handler
// implementations. Built-in JSON and text handlers use slogHandler above.
type loggerSlogHandler struct {
	logger   *Logger
	groups   []string
	segments []slogAttrSegment
}

func (h *loggerSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.logger.level.Enable(Level(level))
}

func (h *loggerSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.logger.handler == nil || !h.logger.level.Enable(Level(record.Level)) {
		return nil
	}
	ctx = adjustCallerDepth(mergeCallerDepth(ctx, h.logger.ctx), -2)
	handler := h.logger.handler
	nGroups := 0
	for _, segment := range h.segments {
		for _, group := range segment.groups[nGroups:] {
			handler = handler.WithGroup(group)
		}
		nGroups = len(segment.groups)
		fields := slogAttrsToFields(segment.attrs)
		if len(fields) > 0 {
			handler = handler.WithFields(ctx, fields...)
		}
	}
	for _, group := range h.groups[nGroups:] {
		handler = handler.WithGroup(group)
	}

	attrs := make([]any, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		if field := slogAttrToField(attr); !field.isEmpty() {
			attrs = append(attrs, field)
		}
		return true
	})
	return handler.Handle(ctx, h.logger.w, Level(record.Level), record.Message, attrs...)
}

func (h *loggerSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 || h.logger.handler == nil {
		return h
	}
	h2 := h.clone()
	h2.segments = append(h2.segments, slogAttrSegment{
		groups: slices.Clone(h.groups),
		attrs:  slices.Clone(attrs),
	})
	return h2
}

func (h *loggerSlogHandler) WithGroup(name string) slog.Handler {
	if name == "" || h.logger.handler == nil {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *loggerSlogHandler) clone() *loggerSlogHandler {
	return &loggerSlogHandler{
		logger:   h.logger,
		groups:   slices.Clone(h.groups),
		segments: slices.Clone(h.segments),
	}
}

func slogAttrsToFields(attrs []slog.Attr) []Field {
	fields := make([]Field, 0, len(attrs))
	for _, attr := range attrs {
		if field := slogAttrToField(attr); !field.isEmpty() {
			fields = append(fields, field)
		}
	}
	return fields
}

var _ slog.Handler = (*loggerSlogHandler)(nil)
