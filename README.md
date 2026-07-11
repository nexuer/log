# log

[简体中文](./README.zh-CN.md)

`github.com/nexuer/log` is a small structured logging package inspired by Go's
standard `log/slog`. It keeps the familiar print, printf, and structured logging
methods while adding delayed field evaluation for values attached with `With`.

Application-level logger management lives in the separate `logmgr` subpackage.

## Install

```sh
go get github.com/nexuer/log
```

## Quick Start

```go
package main

import (
	"errors"
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stdout).With(
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	)

	logger.Info("server starting")
	logger.Infof("listening on %s", ":8080")
	logger.InfoS("server started", "addr", ":8080")

	err := errors.New("timeout")
	logger.ErrorS("request failed", log.Err(err), "path", "/api")
}
```

Text output:

```text
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:15 msg="server starting"
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:16 msg="listening on :8080"
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:17 msg="server started" addr=:8080
ERROR ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:20 msg="request failed" err=timeout path=/api
```

## Logging Methods

Each level has three method forms:

```go
logger.Info(args ...any)                 // fmt.Sprint-style message
logger.Infof(format string, args ...any) // fmt.Sprintf-style message
logger.InfoS(msg string, kvs ...any)     // structured message and fields
```

Use `Info` for plain text, `Infof` for formatted text, and `InfoS` when the
values should be emitted as fields:

```go
logger.Info("retry ", attempt, "/", max)
logger.Infof("retry %d/%d", attempt, max)
logger.InfoS("retry", "attempt", attempt, "max", max)
```

`Info(args...)` does not interpret key-value pairs as fields. For structured
output, use the `S` methods.

## Handlers

The default handler is text:

```go
logger := log.New(os.Stdout, log.Text(&log.HandlerOptions{Name: "server"}))
logger.InfoS("ready", "addr", ":8080")
```

Output:

```text
[server] INFO msg=ready addr=:8080
```

Use `Json` for JSON output:

```go
logger := log.New(os.Stdout, log.Json(&log.HandlerOptions{Name: "server"})).With(
	"service", "api",
	"ts", log.DefaultTimestamp,
)

logger.InfoS("ready", "port", 8080)
```

Output:

```json
{"level":"INFO","logger":"server","service":"api","ts":"2026-06-26T17:30:00+08:00","msg":"ready","port":8080}
```

### slog Handlers

Nexuer handlers can be used behind the standard `log/slog` API:

```go
native := log.New(os.Stdout, log.Json(&log.HandlerOptions{Name: "server"})).
	SetLevel(log.LevelInfo)
logger := slog.New(native.SlogHandler())
logger.Info("ready", "port", 8080)
```

`Logger.SlogHandler` preserves the Logger's current output, level, handler,
fields, and groups. It outputs `level`, `msg`, and user
attributes only. `slog.Record.Time` and `slog.Record.PC` are always ignored.
`HandlerOptions.Replacer` can transform, rename, or remove user attributes and
the built-in `level`, `msg`, and `logger` fields. Returning an empty `Field`
removes a field. Duplicate keys remain allowed, including built-in keys.

## Fields

Typed field helpers are preferred when possible:

```go
logger.InfoS("user login",
	log.String("user", "alice"),
	log.Int("attempt", 1),
	log.Bool("ok", true),
)
```

Plain key-value pairs are also supported:

```go
logger.InfoS("user login", "user", "alice", "attempt", 1)
```

`With`, `Fields`, and structured logging methods accept typed `Field` values,
`slog.Attr` values, ordinary key-value pairs, groups, and delayed `Valuer`
values in the same argument list. Their output order is preserved. An argument
without a matching value is emitted under `<BAD_KEY>`.

Duplicate keys are allowed, including `level`, `msg`, and `logger`. Text output
keeps every occurrence, and JSON output may therefore contain duplicate object
members. JSON consumers commonly keep the last value, but this is parser
dependent; avoid duplicates when interoperability requires one unambiguous
value.

Use `Err` for the standard error field. A nil error returns an empty field and
is not emitted:

```go
logger.ErrorS("request failed", log.Err(err), "path", "/api")
```

`Fields` converts key-value pairs to reusable fields:

```go
fields := log.Fields("service", "api", "region", "cn")
logger := log.New(os.Stdout).WithFields(fields...)
```

## Dynamic Fields

`Valuer` delays evaluation until the record is written:

```go
logger := log.New(os.Stdout).With(
	"ts", log.Timestamp(time.RFC3339),
	"caller", log.Caller(1),
)
```

Timestamp and caller fields are opt-in for a logger: add them with `With`, or
use `With(log.DefaultFields...)`. `DefaultFields` is a read-only package
template and must not be mutated or modified concurrently.

This is useful for timestamps, caller data, request-scoped values, and other
values that should not be computed when `With` is called.

## Printer

`Printer` is a restricted wrapper for code that should only emit plain log
messages. It exposes print, printf, and `io.Writer` style methods only. It does
not expose structured `S` methods, `With`, output changes, handler changes,
`Close`, or `Fatal`.

```go
printer := log.NewPrinter(logger)

printer.Info("server started")
printer.Infof("listening on %s", ":8080")
_, _ = printer.Write([]byte("message from io.Writer"))
```

Use `Logger` instead of `Printer` when structured fields are needed.

## Global Logger

```go
log.SetDefault(log.New(os.Stdout).With(log.DefaultFields...))

log.Info("server starting")
log.InfoS("server started", "addr", ":8080")
log.ErrorS("request failed", log.Err(err), "path", "/api")
```

## Fatal

`Fatal`, `Fatalf`, and `FatalS` write a fatal-level record and then exit with
status code `1`.

```go
logger.FatalS("listen failed", log.Err(err), "addr", addr)
```

## Manager

Use `github.com/nexuer/log/logmgr` when an application needs multiple logger
instances, shared configuration, command-line flags, or scope-level
configuration.

See [logmgr/README.md](./logmgr/README.md).

## Benchmarks

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem
```

Full scenarios and latest raw output are in [benchmarks/README.md](./benchmarks/README.md).
