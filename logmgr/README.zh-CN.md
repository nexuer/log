# logmgr

[English](./README.md) · [API 文档](https://pkg.go.dev/github.com/nexuer/log/logmgr) · [log](../README.zh-CN.md)

`log` 的进程级日志管理包。通过 scope 管理配置，通过 printer 区分组件，支持命令行覆盖和文件轮转。

## 快速开始

```sh
go get github.com/nexuer/log
```

下面的服务将 http、grpc 日志输出到 stderr，db 日志写入文件：

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

`Init` 只调用一次且不返回值。其他包可以直接调用 `logmgr.Printer`、`logmgr.Scope`，
或接收 `log.Printer`。管理操作均为包级函数，scope 类型为 `LoggerScope`。

所有 scope 都必须有非空名称，包括 `Init` 创建的默认 scope。
例如 `Init("server")` 的默认 printer 写入 `server.log`。

## Scope 与 Printer

同一 scope 下的 printer 共享级别、格式、输出和轮转配置。
新 scope 继承 `Init` 的 options，再应用自己的 options。

```go
logmgr.Printer()                    // server
logmgr.Printer("http")              // server.http
logmgr.Scope("db").Printer()        // db
logmgr.Scope("db").Printer("mysql") // db.mysql
```

Scope 需要先用 `AddScope(name, opts...)` 注册；空名称或重复注册会 panic。
`Scope(name)` 只获取已有 scope，不创建、不修改配置；不存在时 panic。
已有 scope 的配置通过 `scope.Apply(opts...)` 修改。
Printer 按名称创建并复用。`logmgr.Scopes()` 返回按名称排序的 scope 列表。

文件路径为 `<file-dir>/<完整 logger 名称>.log`，不同完整名称的 printer 分别写文件。
上面的 db scope 对应：

| Printer | 文件 |
|---|---|
| `db.Printer()` | `log/db/db.log` |
| `db.Printer("mysql")` | `log/db/db.mysql.log` |

完整 logger 名称和目录相同时，允许不同 scope 的 printer 写入同一个文件。
例如 `db` scope 的 `mysql` printer 与 `db.mysql` scope 的默认 printer，
在同一目录下都写入 `db.mysql.log`。

不确定 scope 是否存在时，使用 `LookupScope(name)`，返回 `(*LoggerScope, bool)`，
不创建、不修改配置。与 `Scope` 一样，未初始化或关闭后调用会 panic。
查询成功不会阻止其他 goroutine 随后删除 scope。

## 命令行配置

在 `Init` 前注册并解析 flags。通用参数影响所有 scope，指定 scope 的 `log-set` 优先。

```sh
app --log-format=json
app --log-format=json --log-set=db.format=text
app --log-level=info --log-set=db.level=warn
```

第一条让所有 scope 使用 JSON，http、grpc 仍写 stderr，db 仍写文件。
第二条让 db 单独使用 Text。

| 参数 | 默认值 | 可选值 / 单位 |
|---|---|---|
| `--log-level` | `info` | debug、info、warn、error、fatal |
| `--log-format` | `text` | text、json |
| `--log-output` | `stderr` | stderr、stdout、file |
| `--log-file-dir` | `log` | 文件目录 |
| `--log-file-size` | `512` | MB；0 使用默认值 |
| `--log-file-backups` | `0` | 备份数；0 不限制 |
| `--log-file-compress` | `false` | gzip 压缩 |

默认值在代码未配置该项时使用。没有显式传入的 CLI 参数不会覆盖代码配置。
`--log-set` 支持上述字段去掉 `log-` 前缀后的名称：

```sh
--log-set=format=json
--log-set=db.output=file
--log-set=db.file-dir=log/db
```

配置优先级从低到高：

```text
默认值 → Init options → Scope options → 通用 flags → log-set=key=value → log-set=scope.key=value
```

## 运行时调整

```go
logmgr.DefaultScope().Apply(logmgr.WithLevel(log.LevelWarn)) // 默认 scope
logmgr.Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
```

已获取的 printer 会使用新配置，`Apply` 可以与日志写入并发调用。
显式配置的 CLI 字段仍优先于 `Apply`。

字段和 Replacer 也可以通过 options 配置：

```go
logmgr.WithFields(log.String("service", "api"))
logmgr.AppendFields(log.String("component", "db"))
logmgr.WithKeyValues("service", "api")
logmgr.WithReplacer(replacer)
```

完整 options 见 [API 文档](https://pkg.go.dev/github.com/nexuer/log/logmgr)。
退出时调用 `logmgr.Close()` 关闭托管输出。

## 释放资源

```go
logmgr.Remove("db", "mysql", "redis") // 删除指定 printer，名称相对于 scope
logmgr.Remove("db", "")              // 删除 db scope 的默认 printer
logmgr.Remove("db")                  // 删除整个 scope
```

`Remove` 删除注册项并关闭输出，返回关闭错误；目标不存在时直接成功。
默认 scope 及其默认 printer 不允许删除，默认 scope 下的其他 printer 可以删除。
已持有的 printer 在删除后停止输出；整个 scope 被删除后，旧 scope 的 `Printer`
和带 options 的 `Apply` 调用会 panic。重新创建同名对象不会恢复旧引用。

`logmgr.Close()` 关闭全部托管输出，重复调用直接成功。关闭后不能再获取 scope、
创建 printer、修改配置或重新 `Init`；已持有的 printer 停止输出。

printer 调用通过原子读取获取配置快照，不使用 printer 层的锁。
`Apply`、`Remove`、`Close` 不等待使用旧快照的调用结束。旧调用仍可能尝试写入
已经关闭的输出；写入错误交给 `log.ErrorHandler` 处理（`Printer.Write` 返回错误）。
关闭后拒绝写入还是重新打开输出，由底层 writer 决定。退出时先停止日志生产
goroutine，再调用 `Close`。
