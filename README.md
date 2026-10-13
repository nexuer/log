# log

[简体中文](./README.zh-CN.md) · [API reference](https://pkg.go.dev/github.com/nexuer/log) · [Logger management](./logmgr/README.md)

Structured logging for Go with Text and JSON output, dynamic fields, and `log/slog` integration.

## Installation

```sh
go get github.com/nexuer/log
```

## Quick start

```go
package main

import (
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stderr)
	logger.Info("server started")
	logger.Infof("listening on %s", ":8080")
	logger.InfoS("request completed", "method", "GET", "status", 200)
}
```

```text
INFO msg="server started"
INFO msg="listening on :8080"
INFO msg="request completed" method=GET status=200
```

`Info` writes text, `Infof` formats text, and `InfoS` writes a message with fields.
Debug, Warn, Error, and Fatal provide the same three forms. Fatal logs and exits with status 1.

## JSON and log levels

```go
logger := log.New(os.Stderr, log.Json(&log.HandlerOptions{Name: "server"}))
logger.SetLevel(log.LevelInfo)
logger.InfoS("ready", "port", 8080)
```

```json
{"logger":"server","level":"INFO","msg":"ready","port":8080}
```

Text is the default format. `SetLevel` sets the minimum output level.

## Fields

Use `With` for shared fields and logging methods for fields on a single record.
Both key-value pairs and typed fields are supported:

```go
logger := log.New(os.Stderr).With("service", "api")
logger.InfoS("user login", log.String("user", "alice"), log.Int("attempt", 1))
logger.ErrorS("request failed", log.Err(err), "path", "/api")
```

`log.Err(nil)` produces no field. `WithFields` accepts `[]log.Field`, and
`log.Fields(...)` converts key-value pairs into fields. `slog.Attr` and
`WithGroup` are also supported. Duplicate keys are preserved; arguments
without a matching value use `<BAD_KEY>`.

### Timestamp and caller

New loggers have no timestamp or caller field. Add both with `DefaultFields`:

```go
logger := log.New(os.Stderr).WithFields(log.DefaultFields...)
```

Or add them individually:

```go
logger := log.New(os.Stderr).With(
    "ts", log.DefaultTimestamp,
    "caller", log.DefaultCaller,
)
```

Do not modify `DefaultFields`. Logging wrappers should adjust caller depth by
one for each wrapper frame:

```go
func Info(ctx context.Context, logger *log.Logger, msg string) error {
    return logger.Log(log.AddCallerDepth(ctx, 1), log.LevelInfo, msg)
}
```

`Caller(depth)` counts from where the Valuer is resolved. Use `DefaultCaller` for standard logging paths.

### Valuer and context

A `Valuer` runs when a record is written. Use it to read request fields from ctx:

```go
type traceKey struct{}

logger := log.New(os.Stderr).WithFields(
    log.Dynamic("trace_id", func(ctx context.Context) log.Value {
        return log.AnyValue(ctx.Value(traceKey{}))
    }),
)
ctx := context.WithValue(context.Background(), traceKey{}, "abc123")
_ = logger.Log(ctx, log.LevelInfo, "request completed")
```

`Log(ctx, ...)` uses the supplied ctx and adds caller depth from the call and
logger contexts. Other logging methods use the context set by `WithContext(ctx)`.
Applications explicitly select which context values to log.

## Printer and global logging

Pass a `log.Printer` to code that only needs text logging:

```go
printer := log.NewPrinter(logger)
printer.Info("server started")
printer.Infof("listening on %s", ":8080")
_, _ = printer.Write([]byte("message"))
```

Printer provides text and formatted methods for Debug, Info, Warn, and Error,
plus `io.Writer`. Use Logger for structured fields.

Package functions use the global Logger, which defaults to stderr with timestamp and caller fields:

```go
log.Info("server started")
log.SetDefault(log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...))
log.InfoS("ready", "port", 8080)
```

## log/slog integration

```go
native := log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...)
logger := slog.New(log.NewSlogHandler(native))
logger.Info("ready", "port", 8080)
```

`NewSlogHandler` uses a snapshot of the Logger's configuration. Later changes
to the Logger do not update it. Timestamp and caller use dynamic fields;
`slog.Record.Time` and `PC` are ignored. Use `HandlerOptions.Replacer` to
transform, rename, or remove fields.

## Logger management

Use [logmgr](./logmgr/README.md) for multiple components, file output, and CLI configuration.

Configure Logger's `SetLevel`, `SetOutput`, and `SetHandler` before concurrent use.
For runtime changes to managed logs, use logmgr's `Apply`.

## Benchmarks

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem
```

See [benchmarks](./benchmarks/README.md) for scenarios and results.
