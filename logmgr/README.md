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
	m.AddFlags(flag.CommandLine)
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

`Init` installs the singleton manager and returns it. `M()` returns the current
singleton manager and panics if `Init` has not been called.

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

Configuration is resolved from defaults, code options, and command-line flag
values. Default-scope flags such as `--log-level` and `--log-format` configure
the default scope. Scope flags such as `--log-scope=db.level=warn` configure
the named scope when it is created, and override code options for that scope.
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

Register flags after `Init` and before `flag.Parse`.

```go
m := logmgr.Init("server")
m.AddFlags(flag.CommandLine)
flag.Parse()
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

Named-scope flags:

```sh
--log-scope=db.level=warn
--log-scope=db.format=json
--log-scope=db.output=file
--log-scope=db.file-dir=log/db
--log-scope=db.file-size=256
--log-scope=db.file-backups=5
--log-scope=db.file-compress=false
```

Scope flags use `scope.key=value`, so scopes can be configured before they are
registered in code.
