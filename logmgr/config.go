package logmgr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nexuer/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Format controls the handler output encoding.
type Format int

func (f Format) String() string {
	switch f {
	case TextFormat:
		return "text"
	case JsonFormat:
		return "json"
	}
	return ""
}

const (
	// TextFormat writes human-readable text records.
	TextFormat Format = iota
	// JsonFormat writes JSON records.
	JsonFormat
)

// Output controls where log records are written.
type Output int

func (o Output) String() string {
	switch o {
	case StderrOutput:
		return "stderr"
	case StdoutOutput:
		return "stdout"
	case FileOutput:
		return "file"
	}
	return ""
}

const (
	// StderrOutput writes records to os.Stderr.
	StderrOutput Output = iota
	// StdoutOutput writes records to os.Stdout.
	StdoutOutput
	// FileOutput writes records to rotating log files.
	FileOutput
)

type config struct {
	// flags
	Format *Format
	Level  *log.Level
	Output *Output
	File   fileConfig

	Replacer log.Replacer
	Fields   []log.Field
}

func (c *config) handler(name string) log.Handler {
	opts := &log.HandlerOptions{Name: name, Replacer: c.Replacer}
	if *c.Format == JsonFormat {
		return log.Json(opts)
	}
	return log.Text(opts)
}

func (c *config) writer(name string, current io.Writer) (io.Writer, string) {
	switch *c.Output {
	case FileOutput:
		path := filepath.Join(*c.File.Dir, name+".log")
		if f, ok := current.(*lumberjack.Logger); ok && f.Filename == path {
			return current, ""
		}
		return log.FileWriter(path, *c.File.Size, *c.File.Backups, *c.File.Compress), path
	case StdoutOutput:
		return os.Stdout, ""
	default:
		return os.Stderr, ""
	}
}

type fileConfig struct {
	Dir      *string
	Size     *int64
	Backups  *int64
	Compress *bool
}

// Option changes manager or scope configuration.
type Option struct {
	apply func(*config)
}

// WithFormat sets the output format.
func WithFormat(v Format) Option {
	return Option{apply: func(c *config) {
		c.Format = &v
	}}
}

// WithLevel sets the minimum log level.
func WithLevel(v log.Level) Option {
	return Option{apply: func(c *config) {
		c.Level = &v
	}}
}

// WithFields sets fields that are included in every record.
func WithFields(v ...log.Field) Option {
	return Option{apply: func(c *config) {
		c.Fields = v
	}}
}

func With(v ...any) Option {
	return Option{apply: func(c *config) {
		c.Fields = log.Fields(v...)
	}}
}

// WithOutput sets the output target.
func WithOutput(v Output) Option {
	return Option{apply: func(c *config) {
		c.Output = &v
	}}
}

// WithFileDir sets the file output directory.
func WithFileDir(v string) Option {
	return Option{apply: func(c *config) {
		c.File.Dir = &v
	}}
}

// WithFileSize sets the file rotation size in MB.
func WithFileSize(v int64) Option {
	return Option{apply: func(c *config) {
		c.File.Size = &v
	}}
}

// WithFileBackups sets the number of retained rotated files.
func WithFileBackups(v int64) Option {
	return Option{apply: func(c *config) {
		c.File.Backups = &v
	}}
}

// WithFileCompress sets whether rotated files are compressed.
func WithFileCompress(v bool) Option {
	return Option{apply: func(c *config) {
		c.File.Compress = &v
	}}
}

// WithReplacer sets the field replacer.
func WithReplacer(v log.Replacer) Option {
	return Option{apply: func(c *config) {
		c.Replacer = v
	}}
}

func newConfig(opts []Option, flagsConfig *config) *config {
	next := &config{
		Level:  &defaultLevel,
		Format: &defaultFormat,
		Output: &defaultOutput,
		File: fileConfig{
			Dir:      &defaultFileDir,
			Size:     &defaultFileSize,
			Backups:  &defaultFileBackups,
			Compress: &defaultFileCompress,
		},
	}
	for _, opt := range opts {
		if opt.apply != nil {
			opt.apply(next)
		}
	}

	if flagsConfig != nil {
		if flagsConfig.Format != nil {
			next.Format = flagsConfig.Format
		}
		if flagsConfig.Level != nil {
			next.Level = flagsConfig.Level
		}
		if flagsConfig.Output != nil {
			next.Output = flagsConfig.Output
		}
		if flagsConfig.File.Dir != nil {
			next.File.Dir = flagsConfig.File.Dir
		}
		if flagsConfig.File.Size != nil {
			next.File.Size = flagsConfig.File.Size
		}
		if flagsConfig.File.Backups != nil {
			next.File.Backups = flagsConfig.File.Backups
		}
		if flagsConfig.File.Compress != nil {
			next.File.Compress = flagsConfig.File.Compress
		}
		if flagsConfig.Replacer != nil {
			next.Replacer = flagsConfig.Replacer
		}
		if len(flagsConfig.Fields) > 0 {
			next.Fields = append(next.Fields, flagsConfig.Fields...)
		}
	}
	return next
}

var (
	defaultLevel        = log.LevelInfo
	defaultFormat       = TextFormat
	defaultOutput       = StderrOutput
	defaultFileDir      = "log"
	defaultFileSize     = int64(512)
	defaultFileBackups  = int64(0)
	defaultFileCompress = false
)

// ParseFormat parses a log output format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "text":
		return TextFormat, nil
	case "json":
		return JsonFormat, nil
	default:
		return TextFormat, fmt.Errorf("unknown log format %q", s)
	}
}

// ParseOutput parses a log output target.
func ParseOutput(s string) (Output, error) {
	switch strings.ToLower(s) {
	case "stderr":
		return StderrOutput, nil
	case "stdout":
		return StdoutOutput, nil
	case "file":
		return FileOutput, nil
	default:
		return StderrOutput, fmt.Errorf("unknown log output %q", s)
	}
}

func parseConfigField(cfg *config, key, value string) error {
	switch key {
	case "level":
		v := log.ParseLevel(value)
		cfg.Level = &v
	case "format":
		v, err := ParseFormat(value)
		if err != nil {
			return err
		}
		cfg.Format = &v
	case "output":
		v, err := ParseOutput(value)
		if err != nil {
			return err
		}
		cfg.Output = &v
	case "file-dir":
		cfg.File.Dir = &value
	case "file-size":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid log file size %q: %w", value, err)
		}
		cfg.File.Size = &v
	case "file-backups":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid log file backups %q: %w", value, err)
		}
		cfg.File.Backups = &v
	case "file-compress":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid log file compress %q: %w", value, err)
		}
		cfg.File.Compress = &v
	default:
		return fmt.Errorf("unknown log config key %q", key)
	}
	return nil
}
