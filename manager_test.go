package log

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestInitManager(t *testing.T) {
	logger := New(os.Stderr).With(DefaultFields...)
	logger.Info("hello world")
	Info("global logger: hello world")
	Default().Info("default logger: hello world")
	SetDefault(logger)
	Info("new global logger: hello world")

	InitManager("server", DefaultFields...)

	Info("hello world")
	M().Logger().Info("hello world")

	M().Add("grpcer")
	M().Logger("grpcer").Info("hello grpc")

	M().AddWithSuffix("_grpc")
	M().Logger("server_grpc").Info("hello server_grpc")
}

func TestInitManagerJson(t *testing.T) {
	logger := New(os.Stderr, Json()).With(DefaultFields...)
	logger.Info("hello world")
	Info("global logger: hello world")

	m := InitManager("server", DefaultFields...)
	m.Apply(Config{Format: JsonFormat})

	Info("global logger: hello world")
	M().Logger().Info("hello world")

	M().Add("grpcer")
	M().Logger("grpcer").Info("hello grpc")

	M().AddWithSuffix("_grpc")
	M().Logger("server_grpc").Info("hello server_grpc")
}

func TestMergeConfig(t *testing.T) {
	cfg := mergeConfig()
	fmt.Printf("default: %+v\n", cfg)
	cfg = mergeConfig(Config{
		Format: JsonFormat,
		Level:  LevelDebug,
		Output: StdoutOutput,
		File: FileConfig{
			Dir:      "testdata",
			Size:     1024,
			Backups:  10,
			Compress: true,
		},
		Replacer: func(ctx context.Context, groups []string, field Field) Field {
			return field
		},
	})
	fmt.Printf("merge: %+v\n", cfg)

	levelFlag = "error"
	formatFlag = "text"
	outputFlag = "file"
	dirFlag = "dir"
	maxSizeFlag = 256
	maxBackupsFlag = 5
	flag := false
	compressFlag = &flag
	cfg = mergeConfig(Config{
		Format: JsonFormat,
		Level:  LevelDebug,
		Output: StdoutOutput,
		File: FileConfig{
			Dir:      "testdata",
			Size:     1024,
			Backups:  10,
			Compress: true,
		},
		Replacer: func(ctx context.Context, groups []string, field Field) Field {
			return field
		},
	})
	fmt.Printf("flag: %+v\n", cfg)
}
