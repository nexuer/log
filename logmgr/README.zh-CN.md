# logmgr

[English](./README.md) · [API 文档](https://pkg.go.dev/github.com/nexuer/log/logmgr)

[log](../README.zh-CN.md) 的全局日志管理包，支持命名 scope、命令行配置和文件轮转。

## 背景

一个服务通常包含 HTTP、数据库、任务等模块。它们需要统一管理日志，
但输出要求可能不同：HTTP 写到终端，数据库写到文件，排查问题时只调整某个模块的日志级别。

logmgr 用 **scope 划分配置组**。每个 scope 管理一组 logger，组内共享日志级别、
格式、输出位置、公共字段和文件轮转设置；不同 scope 可以独立配置。
logger 则用于标识组内的组件，例如 `server.http`、`server.grpc`。

配置相同的组件放在同一个 scope，需要独立配置时再创建新 scope。
例如 HTTP 和 gRPC 使用默认的 `server` scope，数据库使用单独的 `db` scope。
文件输出时，每个 logger 按完整名称生成文件，共享的是输出目录和轮转配置。

## 特性

- **全局 API**：初始化一次，各模块按名称获取 scope 和 logger。
- **分组配置**：新 scope 继承初始化配置，也可单独覆盖。
- **动态调整**：通过 Apply 更新整个 scope，已缓存的 logger 自动生效。
- **命令行覆盖**：统一设置全部 scope，或单独调整指定 scope。
- **输出与轮转**：支持 Text、JSON，以及终端和自动轮转的文件输出。
- **资源管理**：Remove 删除指定 logger 或 scope，Close 在退出时释放全部资源。

## 使用

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

先调用一次 `Init`，再使用其他 API。Scope 名称不能为空或重复。

## Scope 与 Logger

同一 scope 下的 logger 共享配置，新 scope 继承 `Init` 的 options。

| API | 用途 |
|---|---|
| `Get()` / `Get(name)` | 获取默认或指定 scope，不存在时 panic |
| `Logger(name...)` | 获取或创建默认 scope 下的 logger |
| `Register(name, opts...)` | 注册 scope |
| `Lookup(name)` | 获取 scope 及是否存在 |
| `Scopes()` | 返回按名称排序的 scope 列表 |
| `scope.Logger(name...)` | 获取或创建 logger |
| `scope.Apply(opts...)` | 更新配置，已缓存的 logger 也会生效 |

Logger 名称相对于 scope：

```go
logmgr.Logger()                    // server
logmgr.Logger("http")              // server.http
logmgr.Get("db").Logger("mysql")   // db.mysql
```

文件输出路径为 `<file-dir>/<完整 logger 名称>.log`。
例如上面的 mysql logger 写入 `log/db/db.mysql.log`。
默认 scope 的配置也应用于 `log.Default()`。

`WithFields` / `AppendFields` 接收 `log.Field`，
`WithKeyValues` / `AppendKeyValues` 接收键值对。
完整选项见 [API 文档](https://pkg.go.dev/github.com/nexuer/log/logmgr)。

## 命令行配置

在 `Init` 前注册并解析 flags，只有显式传入的参数才会覆盖代码配置。

| 参数 | 默认值 |
|---|---|
| `--log-level` | `info` |
| `--log-format` | `text` |
| `--log-output` | `stderr` |
| `--log-file-dir` | `log` |
| `--log-file-size` | `512` MB；0 使用默认值 |
| `--log-file-backups` | `0`，不限制 |
| `--log-file-compress` | `false` |

使用 `--log-set` 设置全部或指定 scope：

```sh
app --log-format=json --log-set=db.level=warn
app --log-set=output=file --log-set=db.file-dir=log/db
```

配置优先级从低到高：

```text
默认值 → Init options → Scope options / Apply → 通用 flags → log-set=key=value → log-set=scope.key=value
```

## 释放资源

```go
logmgr.Remove("db", "mysql", "redis") // 删除指定 logger
logmgr.Remove("db", "")              // 删除 scope 的默认 logger
logmgr.Remove("db")                  // 删除整个 scope
```

Remove 关闭输出并删除注册项，目标不存在时忽略。
默认 scope 及其默认 logger 不允许删除。删除后不再使用旧 scope 或 logger。

程序退出时，先停止日志输出，再调用 `Close`。
Close 关闭全部输出并清空注册表，重复调用直接成功；关闭后不能再注册或重新初始化。
