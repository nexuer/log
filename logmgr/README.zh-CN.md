# logmgr

[English](./README.md)

`github.com/nexuer/log/logmgr` 是 `github.com/nexuer/log` 的单例应用级日志管理包。
它在进程内维护一个全局 manager，按 scope 管理 printer，统一应用配置，并支持命令行覆盖。

## 安装

```sh
go get github.com/nexuer/log
```

## 基础用法

```go
package main

import (
	"flag"

	"github.com/nexuer/log"
	"github.com/nexuer/log/logmgr"
)

func main() {
	logmgr.Init("server",
		logmgr.WithFields(log.String("service", "api")),
	)
	logmgr.AddFlags(flag.CommandLine)
	flag.Parse()

	logmgr.MustAdd("worker")
	db := logmgr.M().MustAddScope("db", logmgr.WithLevel(log.LevelWarn))
	db.MustAdd("mysql")

	logmgr.Printer().Info("server started")
	logmgr.Printer("worker").Infof("job %d started", 42)
	db.Printer().Warn("database latency is high")
	db.Printer("mysql").Error("query failed")
}
```

文本格式输出类似：

```text
[server] INFO service=api msg="server started"
[server.worker] INFO service=api msg="job 42 started"
[db] WARN msg="database latency is high"
[db.mysql] ERROR msg="query failed"
```

启用 JSON 格式后，同样的日志类似：

```json
{"name":"server","level":"INFO","service":"api","msg":"server started"}
{"name":"server.worker","level":"INFO","service":"api","msg":"job 42 started"}
{"name":"db","level":"WARN","msg":"database latency is high"}
{"name":"db.mysql","level":"ERROR","msg":"query failed"}
```

## 单例 Manager

`Init` 安装单例 manager，并返回这个 manager。`M()` 返回当前单例 manager；如果还没有调用
`Init`，`M()` 会 panic。

```go
m := logmgr.Init("server")

m.MustAdd("worker")
logmgr.M().Printer("worker").Info("started")
```

包级快捷函数都会转发到单例 manager：

```go
logmgr.Printer().Info("server")
logmgr.MustAdd("worker")
logmgr.MustAddScope("db")
logmgr.Apply(logmgr.WithFormat(logmgr.JsonFormat))
```

## Scope

Scope 是一个命名配置区域。同一个 scope 下的 printer 共享同一套最终配置。每个 scope 都有一个
默认 printer，名称就是 scope 名；额外添加的 printer 名称为 `scope.printer`。

```go
logmgr.Init("server", logmgr.WithLevel(log.LevelInfo))

db := logmgr.M().MustAddScope("db",
	logmgr.WithLevel(log.LevelWarn),
	logmgr.WithOutput(logmgr.StderrOutput),
)
db.MustAdd("mysql")

logmgr.Printer().Info("server event")      // name: server
logmgr.M().Printer().Info("same printer")  // name: server
db.Printer().Warn("db event")              // name: db
db.Printer("mysql").Error("mysql event")   // name: db.mysql
```

Manager 也可以返回当前已注册 scope 的有序快照：

```go
for _, scope := range logmgr.M().Scopes() {
	fmt.Println(scope.Name())
}
```

如果需要显式处理重复注册错误，可以使用非 `Must` 版本：

```go
if _, err := logmgr.AddScope("db"); err != nil {
	return err
}
if _, err := logmgr.Add("worker"); err != nil {
	return err
}
```

## 配置

配置由默认值、代码中的 option 和命令行 flag 共同决定。`--log-level`、`--log-format`
这类默认 scope flag 配置默认 scope；`--log-scope=db.level=warn` 这类命名 scope flag
会在对应 scope 创建时生效，并覆盖这个 scope 的代码 option。`Apply` 会重新计算某个 scope 的
最终配置，并应用到这个 scope 中已经创建的 printer。它适合运行时调整 level、format、output、
fields 或 replacer。空的 `Apply()` 是 no-op。

包级 `logmgr.Apply` 和 `logmgr.M().Apply` 只更新默认 scope，不会更新 manager 下的所有
scope。修改命名 scope 时，使用 `Scope.Apply`：

```go
logmgr.Apply(logmgr.WithFormat(logmgr.JsonFormat))              // 默认 scope
logmgr.M().Apply(logmgr.WithOutput(logmgr.StdoutOutput))        // 默认 scope
logmgr.M().Scope("db").Apply(logmgr.WithLevel(log.LevelError))  // db scope
```

可用配置项：

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

## 命令行覆盖

在 `Init` 之后、`flag.Parse` 之前注册 flags：

```go
logmgr.Init("server")
logmgr.AddFlags(flag.CommandLine)
flag.Parse()
```

默认 scope 配置：

```sh
--log-level=info
--log-format=json
--log-output=stderr
--log-dir=log
--log-max-size=512
--log-max-backups=5
--log-compress=false
```

命名 scope 配置：

```sh
--log-scope=db.level=warn
--log-scope=db.output=file
--log-scope=db.dir=log/db
--log-scope=http.level=debug
--log-scope=http.format=text
```

scope flag 使用 `scope.key=value`，因此可以在代码注册 scope 之前通过命令行提前配置。
