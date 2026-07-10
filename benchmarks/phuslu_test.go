package benchmarks

import (
	"io"
	"log/slog"

	phuslulog "github.com/phuslu/log"
)

func newPhusluLog() *phuslulog.Logger {
	return &phuslulog.Logger{
		Level:  phuslulog.DebugLevel,
		Writer: phuslulog.IOWriter{Writer: io.Discard},
	}
}

func newPhusluConsoleLog() *phuslulog.Logger {
	return &phuslulog.Logger{
		Level:  phuslulog.DebugLevel,
		Writer: &phuslulog.ConsoleWriter{Writer: io.Discard},
	}
}

func newDisabledPhusluLog() *phuslulog.Logger {
	logger := newPhusluLog()
	logger.Level = phuslulog.ErrorLevel
	return logger
}

func newPhusluSlog(fields ...slog.Attr) *slog.Logger {
	return slog.New(phuslulog.SlogNewJSONHandler(io.Discard, nil).WithAttrs(fields))
}

func newDisabledPhusluSlog(fields ...slog.Attr) *slog.Logger {
	return slog.New(phuslulog.SlogNewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}).WithAttrs(fields))
}

func (u *user) MarshalObject(e *phuslulog.Entry) {
	e.Str("name", u.Name).
		Str("email", u.Email).
		Int64("createdAt", u.CreatedAt.UnixNano())
}

func fakePhusluFields(e *phuslulog.Entry) *phuslulog.Entry {
	return e.
		Int("int", _tenInts[0]).
		Ints("ints", _tenInts).
		Str("string", _tenStrings[0]).
		Strs("strings", _tenStrings).
		Time("time", _tenTimes[0]).
		Times("times", _tenTimes).
		Object("user1", _oneUser).
		Object("user2", _oneUser).
		Objects("users", _tenUsers).
		Err(errExample)
}

func fakePhusluContext() phuslulog.Context {
	return fakePhusluFields(phuslulog.NewContext(nil)).Value()
}
