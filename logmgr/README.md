# logmgr

[简体中文](./README.zh-CN.md) · [API reference](https://pkg.go.dev/github.com/nexuer/log/logmgr) · [log](../README.md)

Global logging management for log, with shared configuration and outputs for multiple components.

Use scopes when modules need independent configuration or logs need coordinated
runtime updates, without managing configuration and output resources in every module.

## Features

- **Scope groups and inheritance**: loggers share configuration within a scope; new scopes inherit Init options before applying their own overrides.
- **Global access**: initialize once and retrieve scopes and loggers by name, reducing configuration passed between modules.
- **Runtime configuration**: Apply updates the entire scope, including cached Printers, without requiring callers to retrieve them again.
- **CLI overrides**: configure all scopes or target a specific scope.
- **Files and resources**: rotating files, removal by logger or scope, and central shutdown cleanup.

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
    http.Info("server started")

    db := logmgr.Register("db",
        logmgr.WithOutput(logmgr.FileOutput),
        logmgr.WithFileDir("log/db"),
    )
    mysql := db.Logger("mysql")
    mysql.Warn("slow query")

    db.Apply(logmgr.WithLevel(log.LevelError))
}
```

### Scopes and Printers

A scope is a configuration group for levels, formats, outputs, common fields, and
rotation settings. Keep components with the same configuration in one scope;
register another scope when separate configuration is needed.
New scopes inherit Init options before applying their own options.

Logger names identify components within a group:

```go
logmgr.Logger()                  // server
logmgr.Logger("http")            // server.http
logmgr.Get("db").Logger("mysql") // db.mysql
```

`Logger` returns a `log.Printer` for message output, without methods to add or
override configured fields, change configuration, or close resources.

Files use `<file-dir>/<full logger name>.log`. The mysql logger above writes to
`log/db/db.mysql.log`. The default scope also configures `log.Default()`.

### Common APIs

| API | Purpose |
|---|---|
| `Init(name, opts...)` | Initialize once and create the default scope |
| `Initialized()` | Whether initialization succeeded; stays true after Close |
| `Logger(name...)` | Get or create a Printer in the default scope |
| `Register(name, opts...)` | Register a scope with a nonempty, unique name |
| `Get(name...)` | Get a scope, default if omitted; panic if missing |
| `Lookup(name)` / `Scopes()` | Look up a scope / list scopes sorted by name |
| `scope.Logger(name...)` | Get or create a Printer; omit the name for the scope's default logger |
| `scope.Apply(opts...)` | Update configuration, including previously obtained Printers |

Use `WithFields` / `AppendFields` for common fields and
`WithKeyValues` / `AppendKeyValues` for key-value pairs. See the API reference for all options.

### Command-line configuration

Call AddFlags and parse arguments before Init. Supported flags:

| Flag | Default |
|---|---|
| `--log-level` | `info` |
| `--log-format` | `text` |
| `--log-output` | `stderr` |
| `--log-file-dir` | `log` |
| `--log-file-size` | `512` MB; 0 uses the default |
| `--log-file-backups` | `0` (unlimited) |
| `--log-file-compress` | `false` |

```sh
app --log-format=json --log-set=db.level=warn
app --log-set=output=file --log-set=db.file-dir=log/db
```

Only explicitly supplied flags override code configuration. Scope-specific
configuration takes precedence over global configuration; flags also take precedence over Apply.

### Releasing resources

```go
logmgr.Remove("db", "mysql") // Remove a logger and close its output.
logmgr.Remove("db", "")      // Remove db's default logger.
logmgr.Remove("db")          // Remove a scope and close its outputs.
```

Missing targets are ignored. The default scope and its default logger are protected.
Stop using removed objects.

Stop logging before calling Close on shutdown. It closes all outputs and clears
registrations. Repeated calls succeed; further registration and reinitialization are forbidden.
