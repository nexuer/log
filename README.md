# log

[简体中文](./README.zh-CN.md)

This structured logging library is developed with reference to Go's standard `log/slog`
package. Although `slog` provides a solid foundation for structured logging, it does not
support delayed field loading with the `With` method, resulting in premature evaluation of
dynamic values. This reduces flexibility and efficiency in scenarios where log attributes,
such as timestamps, request IDs, or metrics, require runtime computation. To address this,
we rewrote this library.

The core package focuses on logger, handler, fields, values, and dynamic field evaluation.
Application-level logger management lives in the separate `logmgr` subpackage.

## Install

```sh
go get github.com/nexuer/log
```

## Basic Usage

```go
package main

import (
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stdout).With(
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	)

	logger.InfoS("starting server", "addr", ":8080")
	logger.ErrorS(nil, "request failed", "path", "/api")
}
```

Text output:

```text
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:12 msg="starting server" addr=:8080
```

## Text Handler

```go
logger := log.New(os.Stdout, log.Text(&log.HandlerOptions{Name: "server"})).With(
	"ts", log.DefaultTimestamp,
	"caller", log.DefaultCaller,
)

logger.InfoS("starting server", "addr", ":8080")
```

Output:

```text
[server] INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:12 msg="starting server" addr=:8080
```

## JSON Handler

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

## Fields

Use typed helpers when possible:

```go
logger.InfoS("user login",
	log.String("user", "alice"),
	log.Int("attempt", 1),
	log.Bool("ok", true),
)
```

Key-value pairs are also supported:

```go
logger.InfoS("user login", "user", "alice", "attempt", 1)
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

This is useful for timestamps, caller data, request-scoped values, and other values that
should not be precomputed when `With` is called.

## Readonly Logger

`Readonly` is a restricted wrapper for code that should only emit messages. It does not
expose structured field logging, `With`, output changes, handler changes, `Close`, or `Fatal`.

```go
readonly := log.NewReadonly(logger)

readonly.Info("server started")
readonly.Infof("listening on %s", ":8080")
_, _ = readonly.Write([]byte("message from io.Writer"))
```

## Global Logger

```go
log.SetDefault(log.New(os.Stdout).With(log.DefaultFields...))

log.InfoS("server started", "addr", ":8080")
log.ErrorS(err, "request failed", "path", "/api")
```

## Manager

Use `github.com/nexuer/log/logmgr` when an application needs multiple logger instances,
shared configuration, command-line flags, or scope-level configuration.

See [logmgr/README.md](./logmgr/README.md).

## Benchmarks

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem
```

Full scenarios and latest raw output are in [benchmarks/README.md](./benchmarks/README.md).
