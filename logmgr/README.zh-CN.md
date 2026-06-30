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

`Init` 安装单例 manager，并返回这个 manager。`name` 是默认 scope 名，不能为空。`M()`
返回当前单例 manager；如果还没有调用 `Init`，`M()` 会 panic。

```go
m := logmgr.Init("server")

m.MustAdd("worker")
logmgr.M().Printer("worker").Info("started")
```

包级 API 有意只保留 `Init` 和 `M`。优先使用 `Init` 返回的 manager；如果需要在其他包中
访问当前 manager，再调用 `logmgr.M()`：

```go
logmgr.M().Printer().Info("server")
logmgr.M().MustAdd("worker")
logmgr.M().MustAddScope("db")
logmgr.M().Apply(logmgr.WithFormat(logmgr.JsonFormat))
```

## Scope

Scope 是一个命名配置区域。同一个 scope 下的 printer 共享同一套最终配置。每个 scope 都有一个
默认 printer，名称就是 scope 名；额外添加的 printer 名称为 `scope.printer`。

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

Manager 也可以返回当前已注册 scope 的有序快照：

```go
for _, scope := range logmgr.M().Scopes() {
	fmt.Println(scope.Name())
}
```

如果需要显式处理重复注册错误，可以使用非 `Must` 版本：

```go
if _, err := logmgr.M().AddScope("db"); err != nil {
	return err
}
if _, err := logmgr.M().Add("worker"); err != nil {
	return err
}
```

## 配置

最终配置由默认值、代码中的 option、命令行 flag 和 `--log-set` 合并得到。`--log-level`、
`--log-format` 这类 flag 作用于默认 scope。`--log-set` 的优先级最高：`key=value`
作用于默认 scope，`scope.key=value` 作用于指定的命名 scope。默认 scope 本身也有名字，
因此当默认 scope 名为 `server` 时，`server.level=debug` 也会作用于默认 scope。

`Apply` 会重新计算一个 scope 的最终配置，并应用到这个 scope 已创建的 printer 上，适合在
运行时调整 level、format、output、fields 或 replacer。空的 `Apply()` 不会产生任何效果。

`Manager.Apply` 只更新默认 scope，不会更新 manager 下的所有 scope。修改命名 scope 时，
请使用 `Scope.Apply`：

```go
logmgr.M().Apply(logmgr.WithOutput(logmgr.StdoutOutput))       // 默认 scope
logmgr.M().Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
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

在 `Init` 之前注册并解析 flags。解析后的值会在 manager 或 scope 创建时生效。

```go
logmgr.AddFlags(flag.CommandLine)
flag.Parse()

m := logmgr.Init("server")
```

默认 scope 配置：

```sh
--log-level=info
--log-format=json
--log-output=stderr
--log-file-dir=log
--log-file-size=512
--log-file-backups=5
--log-file-compress=false
```

动态覆盖配置：

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

`--log-set=key=value` 配置默认 scope；`--log-set=scope.key=value` 可以在命名 scope
注册到代码之前提前配置它。
