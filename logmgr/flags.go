package logmgr

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type flagValue struct {
	typ string
	set func(string) error
}

func (v flagValue) Set(s string) error {
	return v.set(s)
}

func (v flagValue) String() string {
	return ""
}

func (v flagValue) Type() string {
	return v.typ
}

type boolFlagValue struct {
	set func(string) error
}

func (v boolFlagValue) Set(s string) error {
	return v.set(s)
}

func (v boolFlagValue) String() string {
	return ""
}

func (v boolFlagValue) Type() string {
	return "bool"
}

func (v boolFlagValue) IsBoolFlag() bool {
	return true
}

type flags struct {
	config *config
	set    map[string]*config
}

func newFlags() *flags {
	return &flags{
		config: new(config),
		set:    make(map[string]*config),
	}
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", s)
	}
}

// AddFlags registers global log flags and dynamic log-set flags.
func (f *flags) AddFlags(fs *flag.FlagSet) {
	if f.config == nil {
		f.config = &config{}
	}
	fs.Var(
		flagValue{
			typ: "level",
			set: func(s string) error {
				return parseConfigField(f.config, "level", s)
			},
		},
		"log-level",
		fmt.Sprintf("Set log `level`. One of: debug, info, warn, error, fatal (default %q)",
			strings.ToLower(defaultLevel.String())),
	)
	fs.Var(
		flagValue{
			typ: "output",
			set: func(s string) error {
				return parseConfigField(f.config, "output", s)
			},
		},
		"log-output",
		fmt.Sprintf("Set log `output`. One of: stderr, stdout, file (default %q)", defaultOutput),
	)
	fs.Var(
		flagValue{
			typ: "dir",
			set: func(s string) error {
				return parseConfigField(f.config, "file-dir", s)
			},
		},
		"log-file-dir",
		fmt.Sprintf("Directory `dir` to store log files (default %q)", defaultFileDir),
	)
	fs.Var(
		flagValue{
			typ: "format",
			set: func(s string) error {
				return parseConfigField(f.config, "format", s)
			},
		},
		"log-format",
		fmt.Sprintf("Set log `format`. One of: text, json (default %q)", defaultFormat),
	)
	fs.Var(
		flagValue{
			typ: "MB",
			set: func(s string) error {
				if _, err := strconv.ParseInt(s, 10, 64); err != nil {
					return fmt.Errorf("invalid log file size %q: %w", s, err)
				}
				return parseConfigField(f.config, "file-size", s)
			},
		},
		"log-file-size",
		fmt.Sprintf("Maximum log file size in `MB`, 0 means the default value (default %d MB)", defaultFileSize),
	)
	fs.Var(
		flagValue{
			typ: "count",
			set: func(s string) error {
				if _, err := strconv.ParseInt(s, 10, 64); err != nil {
					return fmt.Errorf("invalid log file backups %q: %w", s, err)
				}
				return parseConfigField(f.config, "file-backups", s)
			},
		},
		"log-file-backups",
		fmt.Sprintf("Maximum backup `count` to retain, 0 means unlimited (default %d)", defaultFileBackups),
	)
	fs.Var(
		boolFlagValue{
			set: func(s string) error {
				if _, err := parseBool(s); err != nil {
					return err
				}
				return parseConfigField(f.config, "file-compress", s)
			},
		},
		"log-file-compress",
		fmt.Sprintf("Enable gzip compression for rotated log files (default %t)", defaultFileCompress),
	)
	fs.Var(
		flagValue{
			typ: "key=value",
			set: func(s string) error {
				scope, key, value, err := splitSetFlag(s)
				if err != nil {
					return err
				}
				cfg := f.set[scope]
				if cfg == nil {
					cfg = &config{}
				}
				if err := parseConfigField(cfg, key, value); err != nil {
					return fmt.Errorf("invalid log-set %q: %w", s, err)
				}
				f.set[scope] = cfg
				return nil
			},
		},
		"log-set",
		"Set log config `key=value` or `scope.key=value`. Example: --log-set=db.level=warn",
	)
}

func splitSetFlag(raw string) (scope, key, value string, err error) {
	left, value, ok := strings.Cut(raw, "=")
	if !ok {
		return "", "", "", fmt.Errorf("missing '='")
	}
	scope, key, ok = strings.Cut(left, ".")
	if !ok {
		key = scope
		scope = ""
	}
	if key == "" {
		return "", "", "", fmt.Errorf("want key=value or scope.key=value")
	}
	return scope, key, value, nil
}
