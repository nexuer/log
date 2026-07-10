package log

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
)

func slogAttrToField(attr slog.Attr) Field {
	attr.Value = attr.Value.Resolve()
	return slogResolvedAttrToField(attr)
}

func slogResolvedAttrToField(attr slog.Attr) Field {
	if attr.Equal(slog.Attr{}) {
		return Field{}
	}
	return Field{
		Key:   attr.Key,
		Value: slogValueToValue(attr.Value),
	}
}

func slogValueToValue(value slog.Value) Value {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return StringValue(value.String())
	case slog.KindInt64:
		return Int64Value(value.Int64())
	case slog.KindUint64:
		return Uint64Value(value.Uint64())
	case slog.KindFloat64:
		return Float64Value(value.Float64())
	case slog.KindBool:
		return BoolValue(value.Bool())
	case slog.KindDuration:
		return DurationValue(value.Duration())
	case slog.KindTime:
		return TimeValue(value.Time())
	case slog.KindGroup:
		attrs := value.Group()
		fields := make([]Field, 0, len(attrs))
		for _, attr := range attrs {
			if field := slogAttrToField(attr); !field.isEmpty() {
				fields = append(fields, field)
			}
		}
		return GroupValue(fields...)
	case slog.KindLogValuer:
		return AnyValue(value.LogValuer())
	default:
		return AnyValue(value.Any())
	}
}

func (s *handleState) appendSlogAttr(ctx context.Context, attr slog.Attr) bool {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return false
	}
	if s.h.opts.Replacer != nil && attr.Value.Kind() != slog.KindGroup {
		return s.appendField(ctx, slogResolvedAttrToField(attr), false)
	}

	if attr.Value.Kind() == slog.KindGroup {
		attrs := attr.Value.Group()
		if len(attrs) == 0 {
			return false
		}

		pos := s.buf.Len()
		sep := s.sep
		prefixLen := len(*s.prefix)
		groupLen := 0
		if s.groups != nil {
			groupLen = len(*s.groups)
		}

		if attr.Key != "" {
			s.openGroup(attr.Key)
		}

		nonEmpty := false
		for _, child := range attrs {
			if s.appendSlogAttr(ctx, child) {
				nonEmpty = true
			}
		}
		if !nonEmpty {
			s.buf.SetLen(pos)
			s.sep = sep
			*s.prefix = (*s.prefix)[:prefixLen]
			if s.groups != nil {
				*s.groups = (*s.groups)[:groupLen]
			}
			return false
		}

		if attr.Key != "" {
			s.closeGroup(attr.Key)
		}
		return true
	}

	s.appendKey(attr.Key)
	s.appendSlogValue(attr.Value)
	return true
}

func (s *handleState) appendSlogValue(value slog.Value) {
	switch value.Kind() {
	case slog.KindString:
		s.appendString(value.String())
	case slog.KindInt64:
		*s.buf = strconv.AppendInt(*s.buf, value.Int64(), 10)
	case slog.KindUint64:
		*s.buf = strconv.AppendUint(*s.buf, value.Uint64(), 10)
	case slog.KindFloat64:
		s.appendValue(Float64Value(value.Float64()))
	case slog.KindBool:
		*s.buf = strconv.AppendBool(*s.buf, value.Bool())
	case slog.KindDuration:
		if s.h.json {
			*s.buf = strconv.AppendInt(*s.buf, int64(value.Duration()), 10)
		} else {
			*s.buf = append(*s.buf, value.Duration().String()...)
		}
	case slog.KindTime:
		s.appendTime(value.Time())
	case slog.KindAny:
		s.appendValue(AnyValue(value.Any()))
	case slog.KindLogValuer:
		s.appendValue(AnyValue(value.LogValuer()))
	default:
		s.appendError(fmt.Errorf("bad slog kind: %s", value.Kind()))
	}
}
