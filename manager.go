package log

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Format int

const (
	TextFormat Format = iota
	JsonFormat
)

type Output int

const (
	StderrOutput Output = iota
	StdoutOutput
	FileOutput
)

type Config struct {
	Format   Format
	Level    Level
	Output   Output
	File     FileConfig
	Replacer Replacer
}

type FileConfig struct {
	Dir      string
	Size     int64
	Backups  int64
	Compress bool
}

var defaultCfg = Config{
	Level:  LevelInfo,
	Format: TextFormat,
	Output: StderrOutput,
	File: FileConfig{
		Dir:      "log",
		Size:     512,
		Backups:  0,
		Compress: false,
	},
}

var (
	levelFlag      string
	outputFlag     string
	dirFlag        string
	formatFlag     string
	maxSizeFlag    int64
	maxBackupsFlag int64
	compressFlag   *bool
)

func AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&levelFlag, "log-level", "",
		`Set the log level. One of: ["debug", "info", "warn", "error", "fatal"] (default "info")`)
	fs.StringVar(&outputFlag, "log-output", "",
		`Set the log output. Permitted output: "stderr", "stdout" or "file" (default "stderr")`)
	fs.StringVar(&dirFlag, "log-dir", "",
		fmt.Sprintf(`Directory to store log files (default "%s")`, defaultCfg.File.Dir))
	fs.StringVar(&formatFlag, "log-format", "", `Set the log format. Permitted formats: "text" or "json" (default "json")`)

	fs.Int64Var(&maxSizeFlag, "log-max-size", 0,
		fmt.Sprintf(`Maximum size of each log file in MB, 0 means the default value (default %d MB)`,
			defaultCfg.File.Size))

	fs.Int64Var(&maxBackupsFlag, "log-max-backups", 0,
		fmt.Sprintf(`Maximum number of log file backups to retain, 0 means unlimited (default %d)`,
			defaultCfg.File.Backups))
	fs.BoolVar(compressFlag, "log-compress", defaultCfg.File.Compress,
		fmt.Sprintf(`Enable gzip compression for rotated log files (default %t)`,
			defaultCfg.File.Compress))
}

// mergeString
func mergeString(target *string, value string) {
	if value != "" {
		*target = value
	}
}

func mergeInt[T Level | Format | Output | int | int64](target *T, value T) {
	if value > 0 {
		*target = value
	}
}

func mergeAlways[T Level | Format | Output](target *T, value T) {
	*target = value
}

func mergeConfig(config ...Config) Config {
	finalCfg := defaultCfg

	if len(config) > 0 {
		cfg := config[0]
		if cfg.Replacer != nil {
			finalCfg.Replacer = cfg.Replacer
		}

		mergeAlways(&finalCfg.Level, cfg.Level)
		mergeAlways(&finalCfg.Format, cfg.Format)
		mergeAlways(&finalCfg.Output, cfg.Output)
		mergeString(&finalCfg.File.Dir, cfg.File.Dir)
		mergeInt(&finalCfg.File.Size, cfg.File.Size)
		mergeInt(&finalCfg.File.Backups, cfg.File.Backups)

		if cfg.File.Compress {
			finalCfg.File.Compress = true
		}
	}

	// Apply priority flags
	if levelFlag != "" {
		finalCfg.Level = ParseLevel(levelFlag)
	}

	if outputFlag != "" {
		switch strings.ToLower(outputFlag) {
		case "stdout":
			finalCfg.Output = StdoutOutput
		case "file":
			finalCfg.Output = FileOutput
		default:
			finalCfg.Output = StderrOutput
		}
	}

	if formatFlag != "" {
		switch strings.ToLower(formatFlag) {
		case "json":
			finalCfg.Format = JsonFormat
		default:
			finalCfg.Format = TextFormat
		}
	}

	mergeString(&finalCfg.File.Dir, dirFlag)
	mergeInt(&finalCfg.File.Size, maxSizeFlag)
	mergeInt(&finalCfg.File.Backups, maxBackupsFlag)

	if compressFlag != nil {
		finalCfg.File.Compress = *compressFlag
	}

	return finalCfg
}

type logChannel struct {
	logger *Logger
	fields []Field
}

type Manager struct {
	cfg    Config
	name   string
	main   *logChannel
	others map[string]*logChannel
	mu     sync.Mutex
}

func NewManager(name string, kvs ...any) *Manager {
	m := &Manager{
		cfg:  mergeConfig(),
		name: name,
		main: &logChannel{
			fields: kvsToFieldSlice(kvs),
		},
		others: make(map[string]*logChannel),
	}

	m.initLogger(name, true, m.main.fields...)
	return m
}

func (m *Manager) fields(fs []Field) []Field {
	if len(fs) == 0 {
		return m.main.fields
	}
	length := len(fs) + len(m.main.fields)
	nkvs := make([]Field, 0, length)
	nkvs = append(nkvs, m.main.fields...)
	nkvs = append(nkvs, fs...)
	return nkvs
}

func (m *Manager) visitAll(fn func(name string, l *Logger, fields []Field)) {
	if m.main != nil {
		fn(m.name, m.main.logger, m.main.fields)
	}
	for name, lk := range m.others {
		fn(name, lk.logger, lk.fields)
	}
}

func (m *Manager) Apply(cfg Config) Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = mergeConfig(cfg)

	m.visitAll(func(name string, l *Logger, fields []Field) {
		m.set(name, l, fields)
	})

	return m.cfg
}

func (m *Manager) handler(name string) Handler {
	ho := &HandlerOptions{Name: name, Replacer: m.cfg.Replacer}
	switch m.cfg.Format {
	case JsonFormat:
		return Json(ho)
	default:
		return Text(ho)
	}
}

func (m *Manager) set(name string, l *Logger, fields []Field) {
	l.SetLevel(m.cfg.Level)
	handler := m.handler(name)
	if m.isMain(name) {
		handler = handler.WithFields(l.ctx, fields...)
	} else {
		handler = handler.WithFields(l.ctx, m.fields(fields)...)
	}
	l.SetHandler(handler)
	w, newPath := m.writer(name, l)
	if newPath != "" {
		l.Infof("redirecting log output to file %q", newPath)
	}
	l.SetOutput(w)
	if m.isMain(name) {
		SetDefault(l)
	}
}

func (m *Manager) writer(name string, l *Logger) (io.Writer, string) {
	switch m.cfg.Output {
	case FileOutput:
		path := filepath.Join(m.cfg.File.Dir, name+".log")
		if f, ok := l.Writer().(*lumberjack.Logger); ok && f.Filename == path {
			return f, ""
		}
		return FileWriter(path, m.cfg.File.Size, m.cfg.File.Backups), path
	case StdoutOutput:
		return os.Stdout, ""
	default:
		return os.Stderr, ""
	}
}

func (m *Manager) initLogger(name string, main bool, fields ...Field) *Logger {
	l := New(os.Stderr)

	m.set(name, l, fields)

	if main {
		m.main.logger = l
		return l
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.others[name] = &logChannel{
		fields: fields,
		logger: l,
	}
	return l
}

func (m *Manager) Add(name string, kvs ...any) (*Logger, error) {
	if name == m.name {
		return nil, fmt.Errorf(`log: %q logger already exists`, name)
	}
	if _, ok := m.others[name]; ok {
		return nil, fmt.Errorf(`log: %q logger already exists`, name)
	}
	return m.initLogger(name, false, kvsToFieldSlice(kvs)...), nil
}

func (m *Manager) AddWithSuffix(suffix string, kvs ...any) (*Logger, error) {
	return m.Add(m.name+suffix, kvs...)
}

func (m *Manager) isMain(name string) bool {
	return m.name == name
}

func (m *Manager) Logger(name ...string) *Logger {
	if len(name) == 0 || m.isMain(name[0]) {
		return m.main.logger
	}

	if m.others[name[0]] != nil {
		return m.others[name[0]].logger
	}

	return m.main.logger
}

func (m *Manager) Close() error {
	var errs []error
	m.visitAll(func(name string, l *Logger, fields []Field) {
		if err := l.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	})
	return errors.Join(errs...)
}
