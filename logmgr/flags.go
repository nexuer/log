package logmgr

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type flagValue func(string) error

func (v flagValue) Set(s string) error {
	return v(s)
}

func (v flagValue) String() string {
	return ""
}

type boolFlagValue func(string) error

func (v boolFlagValue) Set(s string) error {
	return v(s)
}

func (v boolFlagValue) String() string {
	return ""
}

func (v boolFlagValue) IsBoolFlag() bool {
	return true
}

type flags struct {
	config *config
	scopes map[string]*config
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

// AddFlags registers global log flags and dynamic scope flags.
func (f *flags) AddFlags(fs *flag.FlagSet) {
	if f.config == nil {
		f.config = &config{}
	}
	fs.Var(
		flagValue(func(s string) error {
			return parseConfigField(f.config, "level", s)
		}),
		"log-level",
		fmt.Sprintf(`Set the log level. One of: ["debug", "info", "warn", "error", "fatal"] (default "%s")`, defaultLevel),
	)
	fs.Var(
		flagValue(func(s string) error {
			return parseConfigField(f.config, "output", s)
		}),
		"log-output",
		fmt.Sprintf(`Set the log output. Permitted output: "stderr", "stdout" or "file" (default "%s")`, defaultOutput),
	)
	fs.Var(
		flagValue(func(s string) error {
			return parseConfigField(f.config, "file-dir", s)
		}),
		"log-file-dir",
		fmt.Sprintf(`Directory to store log files (default "%s")`, defaultFileDir),
	)
	fs.Var(
		flagValue(func(s string) error {
			return parseConfigField(f.config, "format", s)
		}),
		"log-format",
		fmt.Sprintf(`Set the log format. Permitted formats: "text" or "json" (default "%s")`, defaultFormat),
	)
	fs.Var(
		flagValue(func(s string) error {
			if _, err := strconv.ParseInt(s, 10, 64); err != nil {
				return fmt.Errorf("invalid log file size %q: %w", s, err)
			}
			return parseConfigField(f.config, "file-size", s)
		}),
		"log-file-size",
		fmt.Sprintf(`Maximum size of each log file in MB, 0 means the default value (default %d MB)`, defaultFileSize),
	)
	fs.Var(
		flagValue(func(s string) error {
			if _, err := strconv.ParseInt(s, 10, 64); err != nil {
				return fmt.Errorf("invalid log file backups %q: %w", s, err)
			}
			return parseConfigField(f.config, "file-backups", s)
		}),
		"log-file-backups",
		fmt.Sprintf(`Maximum number of log file backups to retain, 0 means unlimited (default %d)`, defaultFileBackups),
	)
	fs.Var(
		boolFlagValue(func(s string) error {
			if _, err := parseBool(s); err != nil {
				return err
			}
			return parseConfigField(f.config, "file-compress", s)
		}),
		"log-file-compress",
		fmt.Sprintf("Enable gzip compression for rotated log files (default %t)", defaultFileCompress),
	)
	fs.Var(
		flagValue(func(s string) error {
			scope, key, value, err := splitScopeFlag(s)
			if err != nil {
				return err
			}
			cfg := f.scopes[scope]
			if cfg == nil {
				cfg = &config{}
			}
			if err := parseConfigField(cfg, key, value); err != nil {
				return fmt.Errorf("invalid log-scope %q: %w", s, err)
			}
			f.scopes[scope] = cfg
			return nil
		}),
		"log-scope",
		`Set scope log config, format: scope.key=value. Example: --log-scope=db.level=warn`,
	)
}

func splitScopeFlag(raw string) (scope, key, value string, err error) {
	left, value, ok := strings.Cut(raw, "=")
	if !ok {
		return "", "", "", fmt.Errorf("missing '='")
	}
	scope, key, ok = strings.Cut(left, ".")
	if !ok || scope == "" || key == "" {
		return "", "", "", fmt.Errorf("want scope.key=value")
	}
	return scope, key, value, nil
}
