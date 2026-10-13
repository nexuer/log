# logmgr

[简体中文](./README.zh-CN.md) · [API reference](https://pkg.go.dev/github.com/nexuer/log/logmgr) · [log](../README.md)

Process-wide logger management for `log`. Scopes manage configuration;
printers identify components. Includes CLI overrides and rotating files.

## Quick start

```sh
go get github.com/nexuer/log
```

This service sends http and grpc logs to stderr and db logs to files:

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

	m := logmgr.Init("server",
		logmgr.WithOutput(logmgr.StderrOutput),
		logmgr.WithFields(log.DefaultFields...),
	)
	defer m.Close()

	db := m.MustAddScope("db",
		logmgr.WithOutput(logmgr.FileOutput),
		logmgr.WithFileDir("log/db"),
		logmgr.WithLevel(log.LevelWarn),
	)

	m.Printer("http").Info("listening on :8080")
	m.Printer("grpc").Info("listening on :9090")
	db.Printer("mysql").Warn("slow query")
}
```

Call `Init` once. Other packages can use `logmgr.M()` or receive a `log.Printer`.

## Scopes and printers

Printers in a scope share level, format, output, and rotation settings.
New scopes inherit `Init` options, then apply their own options.

```go
m.Printer()                    // server
m.Printer("http")              // server.http
m.Scope("db").Printer()        // db
m.Scope("db").Printer("mysql") // db.mysql
```

Register scopes with `AddScope`; duplicate names return an error.
`MustAddScope` panics on error. Printers are created and reused by name.
`Scopes()` lists registered scopes sorted by name.

Each printer writes to its own `<file-dir>/<full logger name>.log` file.
For the db scope above:

| Printer | File |
|---|---|
| `db.Printer()` | `log/db/db.log` |
| `db.Printer("mysql")` | `log/db/db.mysql.log` |

## CLI configuration

Register and parse flags before `Init`. Global flags affect every scope;
scoped `log-set` values take precedence.

```sh
app --log-format=json
app --log-format=json --log-set=db.format=text
app --log-level=info --log-set=db.level=warn
```

The first command uses JSON for all scopes; http and grpc still write to stderr,
and db still writes to files. The second keeps Text output for db.

| Flag | Default | Values / units |
|---|---|---|
| `--log-level` | `info` | debug, info, warn, error, fatal |
| `--log-format` | `text` | text, json |
| `--log-output` | `stderr` | stderr, stdout, file |
| `--log-file-dir` | `log` | File directory |
| `--log-file-size` | `512` | MB; 0 uses the default |
| `--log-file-backups` | `0` | Backup count; 0 means unlimited |
| `--log-file-compress` | `false` | gzip compression |

Defaults apply when an option has not been configured in code. CLI flags only
override explicitly supplied fields. `--log-set` accepts the fields above
without the `log-` prefix:

```sh
--log-set=format=json
--log-set=db.output=file
--log-set=db.file-dir=log/db
```

Configuration precedence, lowest to highest:

```text
Defaults → Init options → Scope options → Global flags → log-set=key=value → log-set=scope.key=value
```

## Runtime configuration

```go
m.Apply(logmgr.WithLevel(log.LevelWarn))              // Default scope
m.Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
```

Existing printers use the updated configuration. `Apply` can run concurrently
with logging. Explicit CLI fields retain precedence over `Apply`.

Fields and Replacer can also be configured through options:

```go
logmgr.WithFields(log.String("service", "api"))
logmgr.AppendFields(log.String("component", "db"))
logmgr.WithKeyValues("service", "api")
logmgr.WithReplacer(replacer)
```

See the [API reference](https://pkg.go.dev/github.com/nexuer/log/logmgr) for all options.
Call `m.Close()` on shutdown to close managed outputs.
