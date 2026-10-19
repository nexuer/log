# logmgr

[简体中文](./README.zh-CN.md) · [API reference](https://pkg.go.dev/github.com/nexuer/log/logmgr)

Process-wide logging configuration for [log](../README.md), with named scopes,
command-line options, and rotating files.

## Background

A service often has HTTP, database, and worker components. They need centrally
managed logging, but may have different requirements: HTTP logs go to the terminal,
database logs go to files, and troubleshooting changes the level of just one group.

logmgr uses a **scope as a configuration group**. Each scope manages loggers that
share a level, format, output target, common fields, and rotation settings.
Different scopes can be configured independently. Logger names identify components
within a group, such as `server.http` and `server.grpc`.

Keep components with the same configuration in one scope; create another scope
when independent configuration is needed. For example, HTTP and gRPC can use the
default `server` scope, while the database uses a separate `db` scope.
With file output, each logger uses its full name as the filename; the directory
and rotation settings are shared.

## Features

- **Global API**: initialize once and retrieve scopes and loggers by name.
- **Grouped configuration**: new scopes inherit initialization options and can override them.
- **Runtime updates**: Apply updates a whole scope, including cached loggers.
- **CLI overrides**: configure all scopes or target a specific scope.
- **Output and rotation**: Text and JSON formats, terminal output, and rotating files.
- **Resource management**: Remove selected loggers or scopes; Close releases all resources on shutdown.

## Usage

```go
package main

import (
    "flag"

    "github.com/nexuer/log"
    "github.com/nexuer/log/logmgr"
)

func main() {
    logmgr.AddFlags(flag.CommandLine)
    flag.Parse()

    logmgr.Init("server", logmgr.WithFields(log.DefaultFields...))
    defer logmgr.Close()

    http := logmgr.Logger("http")
    http.Info("listening on :8080")

    db := logmgr.Register("db",
        logmgr.WithOutput(logmgr.FileOutput),
        logmgr.WithFileDir("log/db"),
    )
    db.Logger("mysql").Warn("slow query")

    db.Apply(logmgr.WithLevel(log.LevelError))
}
```

Call `Init` once before using other APIs. Scope names must be nonempty and unique.

## Scopes and loggers

A scope shares configuration among its loggers. New scopes inherit `Init` options.

| API | Purpose |
|---|---|
| `Get()` / `Get(name)` | Get the default or a named scope; panic if missing |
| `Logger(name...)` | Get or create a logger in the default scope |
| `Register(name, opts...)` | Create a scope |
| `Lookup(name)` | Get a scope and whether it exists |
| `Scopes()` | List scopes sorted by name |
| `scope.Logger(name...)` | Get or create a logger |
| `scope.Apply(opts...)` | Update configuration, including cached loggers |

Logger names are relative to the scope:

```go
logmgr.Logger()                    // server
logmgr.Logger("http")              // server.http
logmgr.Get("db").Logger("mysql")   // db.mysql
```

File output uses `<file-dir>/<full logger name>.log`.
For example, the mysql logger above writes to `log/db/db.mysql.log`.
The default scope also configures `log.Default()`.

Use `WithFields` / `AppendFields` for `log.Field` values and
`WithKeyValues` / `AppendKeyValues` for key-value pairs.
See the [API reference](https://pkg.go.dev/github.com/nexuer/log/logmgr) for all options.

## Command-line configuration

Register and parse flags before `Init`. Only explicitly supplied flags override code options.

| Flag | Default |
|---|---|
| `--log-level` | `info` |
| `--log-format` | `text` |
| `--log-output` | `stderr` |
| `--log-file-dir` | `log` |
| `--log-file-size` | `512` MB; 0 uses the default |
| `--log-file-backups` | `0` (unlimited) |
| `--log-file-compress` | `false` |

Use `--log-set` for global or scope-specific overrides:

```sh
app --log-format=json --log-set=db.level=warn
app --log-set=output=file --log-set=db.file-dir=log/db
```

Precedence, lowest to highest:

```text
Defaults → Init options → Scope options / Apply → Global flags → log-set=key=value → log-set=scope.key=value
```

## Releasing resources

```go
logmgr.Remove("db", "mysql", "redis") // Remove selected loggers.
logmgr.Remove("db", "")              // Remove the scope's default logger.
logmgr.Remove("db")                  // Remove the entire scope.
```

Remove closes outputs and deletes registrations. Missing targets are ignored.
The default scope and its default logger are protected.
Do not continue using removed scopes or loggers.

Stop logging before calling `Close` on shutdown. It closes all outputs and clears
registrations. Repeated calls succeed; registration and reinitialization are no
longer allowed.
