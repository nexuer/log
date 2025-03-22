package benchmarks

import (
	"io"

	"github.com/nexuer/log"
)

func newNexuerLogger() *log.Logger {
	return log.New(io.Discard, log.Json())
}

func newDisabledNexuerLogger() *log.Logger {
	return log.New(io.Discard, log.Json()).SetLevel(log.LevelError)
}

func fakeNexuerLogKvs(hasValuer ...bool) []any {
	if len(hasValuer) > 0 && hasValuer[0] {
		return append(log.DefaultFields, []any{
			"int", _tenInts[0],
			"ints", _tenInts,
			"string", _tenStrings[0],
			"strings", _tenStrings,
			"time", _tenTimes[0],
			"times", _tenTimes,
			"user1", _oneUser,
			"user2", _oneUser,
			"users", _tenUsers,
			"error", errExample,
		}...)
	}
	return []any{
		"int", _tenInts[0],
		"ints", _tenInts,
		"string", _tenStrings[0],
		"strings", _tenStrings,
		"time", _tenTimes[0],
		"times", _tenTimes,
		"user1", _oneUser,
		"user2", _oneUser,
		"users", _tenUsers,
		"error", errExample,
	}
}

func fakeNexuerLogFields() []log.Field {
	return []log.Field{
		log.Int("int", _tenInts[0]),
		log.Any("ints", _tenInts),
		log.String("string", _tenStrings[0]),
		log.Any("strings", _tenStrings),
		log.Time("time", _tenTimes[0]),
		log.Any("times", _tenTimes),
		log.Any("user1", _oneUser),
		log.Any("user2", _oneUser),
		log.Any("users", _tenUsers),
		log.Any("error", errExample),
	}
}
