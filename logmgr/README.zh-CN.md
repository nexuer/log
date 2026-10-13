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

`Init` 只调用一次。其他包可以通过 `logmgr.M()` 获取 manager，或接收 `log.Printer`。

## Scope 与 Printer

同一 scope 下的 printer 共享级别、格式、输出和轮转配置。
新 scope 继承 `Init` 的 options，再应用自己的 options。

```go
m.Printer()                    // server
m.Printer("http")              // server.http
m.Scope("db").Printer()        // db
m.Scope("db").Printer("mysql") // db.mysql
```

Scope 需要先用 `AddScope` 注册；重复注册返回错误，`MustAddScope` 遇到错误会 panic。
Printer 按名称创建并复用。`Scopes()` 返回按名称排序的 scope 列表。

文件路径为 `<file-dir>/<完整 logger 名称>.log`，每个 printer 单独写文件。
上面的 db scope 对应：

| Printer | 文件 |
|---|---|
| `db.Printer()` | `log/db/db.log` |
| `db.Printer("mysql")` | `log/db/db.mysql.log` |

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
m.Apply(logmgr.WithLevel(log.LevelWarn))            // 默认 scope
m.Scope("db").Apply(logmgr.WithLevel(log.LevelError)) // db scope
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
退出时调用 `m.Close()` 关闭托管输出。
