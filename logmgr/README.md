# logmgr

[简体中文](./README.zh-CN.md)

`github.com/nexuer/log/logmgr` is the singleton application logger manager for
`github.com/nexuer/log`. It keeps one process-wide manager, organizes printers
by scope, applies shared configuration, and supports command-line overrides.

## Install

```sh
go get github.com/nexuer/log
```

## Basic Usage

```go
package main

import (
	"flag"

	"github.com/nexuer/log"
	"github.com/nexuer/log/logmgr"
)

func main() {
	m := logmgr.Init("server",
		logmgr.WithFields(log.String("service", "api")),
	)
	logmgr.AddFlags(flag.CommandLine)
	flag.Parse()

	m.MustAdd("worker")
	db := m.MustAddScope("db", logmgr.WithLevel(log.LevelWarn))
	db.MustAdd("mysql")

	m.Printer().Info("server started")
	m.Printer("worker").Infof("job %d started", 42)
	db.Printer().Warn("database latency is high")
	db.Printer("mysql").Error("query failed")
}
```

Text output looks like:

```text
[server] INFO service=api msg="server started"
[server.worker] INFO service=api msg="job 42 started"
[db] WARN msg="database latency is high"
[db.mysql] ERROR msg="query failed"
```

With JSON format enabled, the same records look like:

```json
{"name":"server","level":"INFO","service":"api","msg":"server started"}
{"name":"server.worker","level":"INFO","service":"api","msg":"job 42 started"}
{"name":"db","level":"WARN","msg":"database latency is high"}
{"name":"db.mysql","level":"ERROR","msg":"query failed"}
```

## Singleton Manager

`Init` installs the singleton manager and returns it. Its `name` is the default
scope name and must not be empty. `M()` returns the current singleton manager
and panics if `Init` has not been called.

```go
m := logmgr.Init("server")

m.MustAdd("worker")
logmgr.M().Printer("worker").Info("started")
```

The package-level API is intentionally limited to `Init` and `M`. Use the
returned manager, or call `logmgr.M()` when the manager is needed from another
package:

```go
logmgr.M().Printer().Info("server")
logmgr.M().MustAdd("worker")
logmgr.M().MustAddScope("db")
logmgr.M().Apply(logmgr.WithFormat(logmgr.JsonFormat))
```

## Scopes

A scope is a named configuration area. Printers in the same scope share the
same resolved configuration. Each scope has a default printer with the scope
name, and additional printers are named as `scope.printer`.

```go
m := logmgr.Init("server", logmgr.WithLevel(log.LevelInfo))

db := m.MustAddScope("db",
	logmgr.WithLevel(log.LevelWarn),
	logmgr.WithOutput(logmgr.StderrOutput),
)
db.MustAdd("mysql")

m.Printer().Info("server event")          // name: server
logmgr.M().Printer().Info("same printer") // name: server
db.Printer().Warn("db event")             // name: db
db.Printer("mysql").Error("mysql event")  // name: db.mysql
```

Managers can also return a sorted snapshot of registered scopes:

```go
for _, scope := range logmgr.M().Scopes() {
	fmt.Println(scope.Name())
}
```

Use the non-`Must` forms when duplicate registration should be handled
explicitly:

```go
if _, err := logmgr.M().AddScope("db"); err != nil {
	return err
}
if _, err := logmgr.M().Add("worker"); err != nil {
	return err
}
```

## Configuration

Configuration is resolved from defaults, code options, command-line flags, and
`--log-set`. Default-scope flags such as `--log-level` and `--log-format`
configure the default scope. `--log-set` has the highest priority: `key=value`
configures the default scope, and `scope.key=value` configures a named scope
when it is created. The default scope also has a name, so `server.level=debug`
applies to the default scope when it is named `server`.

`Apply` rebuilds the resolved config for one scope and reapplies it to printers
already created in that scope. Use it when changing level, format, output,
fields, or replacer at runtime. An empty `Apply()` call is a no-op.

`Manager.Apply` only updates the default scope. It does not update every scope
in the manager. Use `Scope.Apply` to change a named scope:

```go
logmgr.M().Apply(logmgr.WithOutput(logmgr.StdoutOutput))       // default scope
logmgr.M().Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
```

Available options:

```go
logmgr.WithLevel(log.LevelDebug)
logmgr.WithFormat(logmgr.TextFormat)
logmgr.WithOutput(logmgr.StdoutOutput)
logmgr.WithFileDir("log")
logmgr.WithFileSize(512)
logmgr.WithFileBackups(5)
logmgr.WithFileCompress(true)
logmgr.WithFields(log.String("service", "api"))
logmgr.WithReplacer(replacer)
```

## Command-line Overrides

Register and parse flags before `Init`. Parsed values are applied when the
manager or scope is created.

```go
logmgr.AddFlags(flag.CommandLine)
flag.Parse()

m := logmgr.Init("server")
```

Default-scope flags:

```sh
--log-level=info
--log-format=json
--log-output=stderr
--log-file-dir=log
--log-file-size=512
--log-file-backups=5
--log-file-compress=false
```

Dynamic overrides:

```sh
--log-set=level=debug
--log-set=server.level=warn
--log-set=db.level=warn
--log-set=db.format=json
--log-set=db.output=file
--log-set=db.file-dir=log/db
--log-set=db.file-size=256
--log-set=db.file-backups=5
--log-set=db.file-compress=false
```

`--log-set=key=value` configures the default scope. `--log-set=scope.key=value`
configures a named scope before it is registered in code.
