package log

import (
	"fmt"
	"strconv"
	"strings"
)

// Level is a logger level.
type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)

func (l Level) Enable(level Level) bool {
	return level >= l
}

func (l Level) String() string {
	str := func(base string, val Level) string {
		if val == 0 {
			return base
		}
		return fmt.Sprintf("%s%+d", base, val)
	}

	switch {
	case l < LevelInfo:
		return str("DEBUG", l-LevelDebug)
	case l < LevelWarn:
		return str("INFO", l-LevelInfo)
	case l < LevelError:
		return str("WARN", l-LevelWarn)
	case l < LevelFatal:
		return str("ERROR", l-LevelError)
	default:
		return str("FATAL", l-LevelFatal)
	}
}

func ParseLevel(s string) Level {
	l := LevelInfo
	name := s
	offset := 0
	if i := strings.IndexAny(s, "+-"); i >= 0 {
		name = s[:i]
		offset, _ = strconv.Atoi(s[i:])
	}
	switch strings.ToUpper(name) {
	case "DEBUG":
		l = LevelDebug
	case "INFO":
		l = LevelInfo
	case "WARN":
		l = LevelWarn
	case "ERROR":
		l = LevelError
	case "FATAL":
		l = LevelFatal
	}
	return l + Level(offset)
}
