# logmgr

[English](./README.md) · [API 文档](https://pkg.go.dev/github.com/nexuer/log/logmgr) · [log](../README.zh-CN.md)

log 的全局日志管理包，用于统一配置各模块的日志、管理日志文件和释放资源。

不同模块可以使用各自的 scope，分别设置日志级别、格式和输出位置；
配置相同的模块也可以共用一个 scope，在运行时一起调整。

## 特性

- **分组配置**：同一 scope 下的 logger 共用配置。新 scope 默认继承 Init 的配置，也可以单独设置。
- **使用方便**：初始化一次，各模块即可按名称获取 scope 和 logger，无需层层传递配置。
- **运行时调整**：通过 Apply 修改整个 scope 的配置，之前获取并保存的 Printer 同样生效，无需重新获取。
- **命令行配置**：可以统一调整所有 scope，也可以单独设置某个 scope。
- **文件管理**：支持日志文件轮转，删除 logger 或 scope 时释放对应资源，程序退出时统一关闭。

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
    http.Info("服务已启动")

    db := logmgr.Register("db",
        logmgr.WithOutput(logmgr.FileOutput),
        logmgr.WithFileDir("log/db"),
    )
    mysql := db.Logger("mysql")
    mysql.Warn("查询较慢")

    db.Apply(logmgr.WithLevel(log.LevelError))
}
```

### Scope 与 Printer

Scope 将一组 logger 放在一起管理，统一设置日志级别、格式、输出位置、公共字段和文件轮转规则。
配置相同的组件共用 scope，需要独立配置时再注册新的 scope。
注册时会先继承 Init 的配置，再应用本次传入的配置选项。

通过 logger 名称区分同一 scope 下的组件：

```go
logmgr.Logger()                  // server
logmgr.Logger("http")            // server.http
logmgr.Get("db").Logger("mysql") // db.mysql
```

`Logger` 返回 `log.Printer`，业务代码只负责打印日志，不能通过它添加或覆盖预设字段、修改配置或关闭资源。

文件路径为 `<file-dir>/<完整 logger 名称>.log`。
示例中的 mysql 写入 `log/db/db.mysql.log`。默认 scope 的配置也会应用到 `log.Default()`。

### 常用 API

| API | 用途 |
|---|---|
| `Init(name, opts...)` | 初始化一次，创建默认 scope |
| `Initialized()` | 判断是否已成功初始化，Close 后仍为 true |
| `Logger(name...)` | 获取或创建默认 scope 下的 Printer |
| `Register(name, opts...)` | 注册 scope，名称不能为空或重复 |
| `Get(name...)` | 获取 scope，省略名称为默认 scope；不存在时 panic |
| `Lookup(name)` / `Scopes()` | 查询 scope / 按名称列出 scope |
| `scope.Logger(name...)` | 获取或创建 Printer，省略名称为 scope 的默认 logger |
| `scope.Apply(opts...)` | 修改配置，之前获取的 Printer 也会使用新配置 |

通过 `WithFields` 设置公共字段，`AppendFields` 追加公共字段。
如果使用键值对，则选择 `WithKeyValues` / `AppendKeyValues`。完整配置选项见 API 文档。

### 命令行配置

在 Init 前调用 AddFlags 注册命令行参数，并完成参数解析：

| 参数 | 默认值 |
|---|---|
| `--log-level` | `info` |
| `--log-format` | `text` |
| `--log-output` | `stderr` |
| `--log-file-dir` | `log` |
| `--log-file-size` | `512` MB；0 使用默认值 |
| `--log-file-backups` | `0`，不限制 |
| `--log-file-compress` | `false` |

```sh
app --log-format=json --log-set=db.level=warn
app --log-set=output=file --log-set=db.file-dir=log/db
```

只有明确传入的命令行参数才会覆盖代码中的配置，针对某个 scope 的参数优先于全局参数。
通过 Apply 调整配置时，命令行中已设置的选项仍然优先。

### 释放资源

```go
logmgr.Remove("db", "mysql") // 删除指定 logger 并关闭输出
logmgr.Remove("db", "")      // 删除 db 的默认 logger
logmgr.Remove("db")          // 删除整个 scope 并关闭输出
```

目标不存在时直接返回。默认 scope 及其默认 logger 不允许删除；删除其他 scope 或 logger 后，不应继续使用原来的引用。

程序退出时先停止日志输出，再调用 Close。Close 关闭全部输出并清空注册表，
重复调用不会报错；关闭后不能再注册 scope 或重新初始化。
