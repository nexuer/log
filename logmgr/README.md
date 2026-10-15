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

	logmgr.Init("server",
		logmgr.WithOutput(logmgr.StderrOutput),
		logmgr.WithFields(log.DefaultFields...),
	)
	defer logmgr.Close()

	db := logmgr.AddScope("db",
		logmgr.WithOutput(logmgr.FileOutput),
		logmgr.WithFileDir("log/db"),
		logmgr.WithLevel(log.LevelWarn),
	)

	logmgr.Printer("http").Info("listening on :8080")
	logmgr.Printer("grpc").Info("listening on :9090")
	db.Printer("mysql").Warn("slow query")
}
```

Call `Init` once. Other packages can use `logmgr.Printer` and `logmgr.Scope`,
or receive a `log.Printer`. All manager operations are package-level functions;
`Init` has no return value. The scope type is `LoggerScope`.

All scopes require a nonempty name, including the default scope created by
`Init`. For example, the default printer from `Init("server")` writes to `server.log`.

## Scopes and printers

Printers in a scope share level, format, output, and rotation settings.
New scopes inherit `Init` options, then apply their own options.

```go
logmgr.Printer()                    // server
logmgr.Printer("http")              // server.http
logmgr.Scope("db").Printer()        // db
logmgr.Scope("db").Printer("mysql") // db.mysql
```

Register scopes with `AddScope(name, opts...)`; empty or duplicate names panic.
`Scope(name)` only retrieves a registered scope;
it does not create scopes or change configuration, and panics if missing.
Use `AddScope(name, opts...)` to configure a new scope and `scope.Apply(opts...)`
to update an existing one. Printers are created and reused by name.
`logmgr.Scopes()` lists registered scopes sorted by name.

Printers write to `<file-dir>/<full logger name>.log`.
For the db scope above:

| Printer | File |
|---|---|
| `db.Printer()` | `log/db/db.log` |
| `db.Printer("mysql")` | `log/db/db.mysql.log` |

Printers in different scopes may write to the same file when their full logger
names and directories match. For example, the `mysql` printer in scope `db` and
the default printer in scope `db.mysql` both write to `db.mysql.log` in the same directory.

Use `LookupScope(name)` when a scope may be absent; it returns `(*LoggerScope, bool)`
and does not create or change configuration. Like `Scope`, it panics before
`Init` or after `Close`. Lookup does not prevent concurrent removal.

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
logmgr.DefaultScope().Apply(logmgr.WithLevel(log.LevelWarn)) // Default scope
logmgr.Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
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
Call `logmgr.Close()` on shutdown to close managed outputs.

## Releasing resources

```go
logmgr.Remove("db", "mysql", "redis") // Remove printers by scope-relative names.
logmgr.Remove("db", "")              // Remove db's default printer.
logmgr.Remove("db")                  // Remove the entire scope.
```

`Remove` unregisters resources, closes their outputs, and returns close errors.
Missing scopes or printers are ignored. The default scope and its default printer
cannot be removed; other printers within the default scope can be removed.
Held printers stop output after removal. After an entire scope is removed, its
old `Printer` and nonempty `Apply` calls panic. Recreating the same name does not
reactivate old references.

`logmgr.Close()` closes all managed outputs; repeated calls succeed. After closing,
scope access, printer creation, configuration changes, and another `Init` panic.
Held printers stop output.

Printer calls atomically load a configuration snapshot without a printer lock.
`Apply`, `Remove`, and `Close` do not wait for calls using an older snapshot.
Those calls may still attempt to write to the previous output after it is closed;
write failures are handled by `log.ErrorHandler` (or returned by `Printer.Write`).
Whether a closed writer rejects writes or reopens its output depends on that
writer. Stop logging goroutines before calling `Close` on shutdown.
