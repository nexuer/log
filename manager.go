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

const (
	defaultMaxSize    int64 = 1024
	defaultMaxBackups int64 = 0
	defaultOutput           = "stderr"
	defaultFormat           = "text"
	defaultLevel            = "info"
	defaultDir              = "log"
)

type Options struct {
	Format     string
	Level      string
	Output     string
	Dir        string
	MaxSize    int64
	MaxBackups int64
}

func (o *Options) Copy() *Options {
	return &Options{
		Format:     o.Output,
		Level:      o.Level,
		Output:     o.Output,
		Dir:        o.Dir,
		MaxSize:    o.MaxSize,
		MaxBackups: o.MaxBackups,
	}
}

var priorityFlag = new(Options)

func AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&priorityFlag.Level, "log-level", "",
		fmt.Sprintf(`Set the log level. One of: ["debug", "info", "warn", "error", "fatal"] (default "%s")`, defaultLevel))
	fs.StringVar(&priorityFlag.Output, "log-output", "",
		fmt.Sprintf(`Set the log output. Permitted output: "stdout", "stderr" or "file" (default "%s")`, defaultOutput))
	fs.StringVar(&priorityFlag.Dir, "log-dir", "",
		fmt.Sprintf(`Directory to store log files (default "%s")`, defaultDir))
	fs.StringVar(&priorityFlag.Format, "log-format", "",
		fmt.Sprintf(`Set the log format. Permitted formats: "text" or "json" (default "%s")`, defaultFormat))

	fs.Int64Var(&priorityFlag.MaxSize, "log-max-size", 0,
		fmt.Sprintf(`Maximum size of each log file in MB, 0 means the default value (default %d MB)`, defaultMaxSize))

	fs.Int64Var(&priorityFlag.MaxBackups, "log-max-backups", 0,
		fmt.Sprintf(`Maximum number of log file backups to retain, 0 means unlimited (default %d)`, defaultMaxBackups))
}

func file(path string, size int64, backups int64, compress ...bool) io.Writer {
	w := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    int(size),
		MaxBackups: int(backups),
		LocalTime:  true,
	}
	if len(compress) > 0 && compress[0] {
		w.Compress = true
	}
	return w
}

func mergeOptions(opts *Options) *Options {
	if opts == nil {
		opts = &Options{
			Output:     defaultOutput,
			Format:     defaultFormat,
			Level:      defaultLevel,
			Dir:        defaultDir,
			MaxSize:    defaultMaxSize,
			MaxBackups: defaultMaxBackups,
		}
	}

	// Apply default
	if opts.Output == "" {
		opts.Output = defaultOutput
	}
	if opts.Format == "" {
		opts.Format = defaultFormat
	}
	if opts.Level == "" {
		opts.Level = defaultLevel
	}
	if opts.Dir == "" {
		opts.Dir = defaultDir
	}
	if opts.MaxSize == 0 {
		opts.MaxSize = defaultMaxSize
	}
	if opts.MaxBackups == 0 {
		opts.MaxBackups = defaultMaxBackups
	}

	// Apply priority flags
	if priorityFlag.Output != "" {
		opts.Output = priorityFlag.Output
	}

	if priorityFlag.Dir != "" {
		opts.Dir = priorityFlag.Dir
	}

	if priorityFlag.Format != "" {
		opts.Format = priorityFlag.Format
	}

	if priorityFlag.Level != "" {
		opts.Level = priorityFlag.Level
	}

	if priorityFlag.MaxSize != 0 {
		opts.MaxSize = priorityFlag.MaxSize
	}

	if priorityFlag.MaxBackups != 0 {
		opts.MaxBackups = priorityFlag.MaxBackups
	}

	return opts
}

type loggerWithKvs struct {
	kvs    []any
	logger *Logger
}

type Manager struct {
	mu     sync.Mutex
	opts   *Options
	name   string
	main   *loggerWithKvs
	others map[string]*loggerWithKvs
}

func NewManager(name string, kvs ...any) *Manager {
	opts := mergeOptions(nil)
	m := &Manager{
		opts: opts,
		name: name,
		main: &loggerWithKvs{
			kvs: kvs,
		},
		others: make(map[string]*loggerWithKvs),
	}

	m.initLogger(name, true)
	return m
}

func (m *Manager) kvs(kvs []any) []any {
	if len(kvs) == 0 {
		return m.main.kvs
	}
	length := len(kvs) + len(m.main.kvs)
	nkvs := make([]any, 0, length)
	nkvs = append(nkvs, m.main.kvs...)
	nkvs = append(nkvs, kvs...)
	return nkvs
}

func (m *Manager) visitAll(fn func(name string, l *Logger, kvs []any)) {
	if m.main != nil {
		fn(m.name, m.main.logger, nil)
	}
	for name, lk := range m.others {
		fn(name, lk.logger, lk.kvs)
	}
}

func (m *Manager) Apply(opts *Options) *Options {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opts = mergeOptions(opts)

	m.visitAll(func(name string, l *Logger, kvs []any) {
		m.set(name, l, kvs)
	})

	return m.opts.Copy()
}

func (m *Manager) writer(name string, l *Logger) (io.Writer, string) {
	switch strings.ToLower(m.opts.Output) {
	case "file":
		path := filepath.Join(m.opts.Dir, name+".log")
		if f, ok := l.Writer().(*lumberjack.Logger); ok && f.Filename == path {
			return f, ""
		}
		return file(path, m.opts.MaxSize, m.opts.MaxBackups), path
	case "stdout":
		return os.Stdout, ""
	default:
		return os.Stderr, ""
	}
}

func (m *Manager) handler(name string, kvs []any) Handler {
	switch strings.ToLower(m.opts.Format) {
	case "json":
		return Json(name).With(m.kvs(kvs)...)
	default:
		return Text(name).With(m.kvs(kvs)...)
	}
}

func (m *Manager) level() Level {
	return ParseLevel(m.opts.Level)
}

func (m *Manager) set(name string, l *Logger, kvs []any) {
	l.SetLevel(m.level())
	l.SetHandler(m.handler(name, kvs))
	w, newPath := m.writer(name, l)
	if newPath != "" {
		l.Infof("redirecting log output to file %q", newPath)
	}
	l.SetOutput(w)
}

func (m *Manager) initLogger(name string, main bool, kvs ...any) *Logger {
	logger := New(os.Stderr)

	m.set(name, logger, kvs)

	if main {
		m.main.logger = logger
		return logger
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.others[name] = &loggerWithKvs{
		kvs:    kvs,
		logger: logger,
	}
	return logger
}

func (m *Manager) Add(name string, kvs ...any) (*Logger, error) {
	if name == m.name {
		return nil, fmt.Errorf(`log: %q logger already exists`, name)
	}
	if _, ok := m.others[name]; ok {
		return nil, fmt.Errorf(`log: %q logger already exists`, name)
	}
	return m.initLogger(name, false, kvs...), nil
}

func (m *Manager) AddWithSuffix(suffix string, kvs ...any) (*Logger, error) {
	return m.Add(m.name+suffix, kvs...)
}

func (m *Manager) Logger(name ...string) *Logger {
	if len(name) == 0 || name[0] == m.name {
		return m.main.logger
	}

	if m.others[name[0]] != nil {
		return m.others[name[0]].logger
	}

	return m.main.logger
}

func (m *Manager) Close() error {
	var errs error
	m.visitAll(func(name string, l *Logger, kvs []any) {
		if err := l.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = errors.Join(errs, err)
		}
	})
	return errs
}
