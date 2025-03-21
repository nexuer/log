package log

import (
	"testing"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		input Level
		want  string
	}{
		{
			input: LevelDebug,
			want:  "DEBUG",
		},
		{
			input: LevelInfo,
			want:  "INFO",
		},
		{
			input: LevelWarn,
			want:  "WARN",
		},
		{
			input: LevelError,
			want:  "ERROR",
		},
		{
			input: LevelFatal,
			want:  "FATAL",
		},

		{
			input: Level(LevelDebug - 1),
			want:  "DEBUG-1",
		},
		{
			input: Level(LevelDebug + 1),
			want:  "DEBUG+1",
		},
		{
			input: Level(LevelDebug + 4),
			want:  "INFO",
		},
		{
			input: Level(LevelInfo),
			want:  "INFO",
		},

		{
			input: Level(LevelInfo + 1),
			want:  "INFO+1",
		},
		{
			input: Level(LevelInfo + 4),
			want:  "WARN",
		},
		{
			input: Level(LevelWarn),
			want:  "WARN",
		},
		{
			input: Level(LevelWarn + 1),
			want:  "WARN+1",
		},
		{
			input: Level(LevelWarn + 4),
			want:  "ERROR",
		},
		{
			input: Level(LevelError),
			want:  "ERROR",
		},
		{
			input: Level(LevelError + 1),
			want:  "ERROR+1",
		},
		{
			input: Level(LevelError + 4),
			want:  "FATAL",
		},
		{
			input: Level(LevelFatal),
			want:  "FATAL",
		},
	}

	for i, tt := range tests {
		if got := tt.input.String(); got != tt.want {
			t.Errorf("#%d Level.String() = %v, want %v", i, got, tt.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{
			want:  LevelDebug,
			input: "debug",
		},
		{
			want:  LevelDebug,
			input: "DEBUG",
		},
		{
			want:  LevelInfo,
			input: "INFO",
		},
		{
			want:  LevelWarn,
			input: "WARN",
		},
		{
			want:  LevelError,
			input: "ERROR",
		},
		{
			want:  LevelFatal,
			input: "FATAL",
		},

		{
			want:  Level(LevelDebug - 1),
			input: "DEBUG-1",
		},
		{
			want:  Level(LevelDebug + 1),
			input: "DEBUG+1",
		},
		{
			want:  Level(LevelDebug + 4),
			input: "INFO",
		},
		{
			want:  Level(LevelInfo),
			input: "INFO",
		},

		{
			want:  Level(LevelInfo + 1),
			input: "INFO+1",
		},
		{
			want:  Level(LevelInfo + 4),
			input: "WARN",
		},
		{
			want:  Level(LevelWarn),
			input: "WARN",
		},
		{
			want:  Level(LevelWarn + 1),
			input: "WARN+1",
		},
		{
			want:  Level(LevelWarn + 4),
			input: "ERROR",
		},
		{
			want:  Level(LevelError),
			input: "ERROR",
		},
		{
			want:  Level(LevelError + 1),
			input: "ERROR+1",
		},
		{
			want:  Level(LevelError + 4),
			input: "FATAL",
		},
		{
			want:  Level(LevelFatal),
			input: "FATAL",
		},
	}

	for i, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.want {
			t.Errorf("#%d ParseLevel() = %v, want %v", i, got, tt.want)
		}
	}
}
