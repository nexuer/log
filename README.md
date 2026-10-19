# log

[简体中文](./README.zh-CN.md) · [API reference](https://pkg.go.dev/github.com/nexuer/log) · [logmgr](./logmgr/README.md)

A Go logging library with plain, formatted, and structured logging, Text and JSON output, and Context-aware dynamic fields.

Use it when you want distinct message and structured logging APIs, centrally owned
common fields, and fields resolved from Context when records are written.
Call its APIs directly or integrate through a `log/slog` Handler.

## Features

- **Explicit calls**: use `Info` for messages, `Infof` for formatting, and `InfoS` for fields on a record.
- **Context-aware dynamic fields**: resolve fields at write time and read Context directly for values such as trace IDs; opt into timestamps and caller information.
- **Flexible structured output**: Text, JSON, key-value pairs, typed fields, groups, reusable common fields, and a Replacer to transform, mask, or filter fields.
- **slog integration**: keep standard `log/slog` calls while using this library's output and field capabilities.
- **Printer interface**: give callers plain and formatted logging with the Logger's fields and output settings, without methods to add fields, change configuration, or close resources.

## Installation

```sh
go get github.com/nexuer/log
```

## Usage

### Messages and structured logging

```go
logger := log.New(os.Stderr)

logger.Info("server started")
logger.Infof("listening on port %d", 8080)
logger.InfoS("request completed", "method", "GET", "status", 200)
```

Text is the default format. Debug, Info, Warn, Error, and Fatal have plain, formatted, and structured forms. Fatal logs and exits the process.

### Format, level, and common fields

```go
logger := log.New(os.Stderr, log.Json()).
    SetLevel(log.LevelInfo).
    With("service", "api").
    WithFields(log.DefaultFields...)

logger.InfoS("request completed", log.String("path", "/users"), log.Int("status", 200))
```

`DefaultFields` adds timestamps and caller information; new Loggers omit them by default.
Use `With` for key-value pairs, `WithFields` for typed fields, and `WithGroup` for groups.
`log.Err(err)` adds an error field and omits nil errors.

### Printer

Printer separates configuration from logging: the owner configures fields and output;
callers write log messages without methods to add or override configured fields,
change configuration, or close resources.

```go
logger := log.New(os.Stderr, log.Json()).With("service", "api")
printer := log.NewPrinter(logger)
printer.Info("server started")
printer.Infof("listening on port %d", 8080)
```

Every record above includes `service=api`. Printer supports Debug, Info, Warn, and
Error with formatted variants, and can also be used as an `io.Writer`.

### Global logging

```go
log.SetDefault(logger)
log.Info("server started")
log.InfoS("request completed", "status", 200)
```

The default global Logger writes to stderr with timestamps and caller information.

### Dynamic fields and Context

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

Dynamic fields resolve when records are written. Other logging methods use the
Context set by `WithContext(ctx)`. Logging wrappers can use
`log.AddCallerDepth(ctx, 1)` to adjust caller information.

### log/slog integration

```go
native := log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...)
logger := slog.New(log.NewSlogHandler(native))
logger.Info("request completed", "status", 200)
```

The Handler uses the Logger's configuration at creation. Configure timestamps and
caller information through fields. Use `log.HandlerOptions.Replacer` to transform,
mask, or filter fields.

Configure a Logger before concurrent logging. For central management, see [logmgr](./logmgr/README.md).

## Benchmarks

Run the separate benchmarks module:

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem -count=5
```

Compare with slog, zap, zerolog, phuslu/log, and logrus:

```sh
go test -run '^$' -bench='^BenchmarkComparison(EncodeOnly|WritePath)$' -benchmem -count=5
```

Scenarios cover Text / JSON, field encoding, disabled levels, and serial and parallel
output. EncodeOnly measures encoding; WritePath includes the writer path. Neither
measures actual disk performance.

See [benchmarks](./benchmarks/README.md) for the environment, full results, and more commands.
