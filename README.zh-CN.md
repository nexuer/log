# log

[English](./README.md) · [API 文档](https://pkg.go.dev/github.com/nexuer/log) · [日志管理](./logmgr/README.zh-CN.md)

Go 结构化日志库，支持 Text、JSON、动态字段和 `log/slog` 接入。

## 安装

```sh
go get github.com/nexuer/log
```

## 快速开始

```go
package main

import (
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stderr)
	logger.Info("server started")
	logger.Infof("listening on %s", ":8080")
	logger.InfoS("request completed", "method", "GET", "status", 200)
}
```

```text
INFO msg="server started"
INFO msg="listening on :8080"
INFO msg="request completed" method=GET status=200
```

`Info` 输出文本，`Infof` 格式化文本，`InfoS` 输出消息和字段。
Debug、Warn、Error、Fatal 也提供这三种方法。Fatal 写入日志后以状态码 1 退出。

## JSON 与日志级别

```go
logger := log.New(os.Stderr, log.Json(&log.HandlerOptions{Name: "server"}))
logger.SetLevel(log.LevelInfo)
logger.InfoS("ready", "port", 8080)
```

```json
{"logger":"server","level":"INFO","msg":"ready","port":8080}
```

默认使用 Text。`SetLevel` 设置最低输出级别。

## 字段

`With` 添加公共字段，日志调用添加本次字段。可以使用键值对或类型化字段：

```go
logger := log.New(os.Stderr).With("service", "api")
logger.InfoS("user login", log.String("user", "alice"), log.Int("attempt", 1))
logger.ErrorS("request failed", log.Err(err), "path", "/api")
```

`log.Err(nil)` 不输出字段。`WithFields` 接受 `[]log.Field`，
`log.Fields(...)` 将键值对转换为字段。也支持 `slog.Attr` 和 `WithGroup`。
重复 key 会保留；没有配对 value 的参数使用 `<BAD_KEY>`。

### 时间戳与 Caller

新建 Logger 不默认添加时间戳和调用位置，需要时使用 `DefaultFields`：

```go
logger := log.New(os.Stderr).WithFields(log.DefaultFields...)
```

也可以单独添加：

```go
logger := log.New(os.Stderr).With(
    "ts", log.DefaultTimestamp,
    "caller", log.DefaultCaller,
)
```

不要修改 `DefaultFields`。封装日志调用时，每层包装用 `AddCallerDepth` 补偿一层：

```go
func Info(ctx context.Context, logger *log.Logger, msg string) error {
    return logger.Log(log.AddCallerDepth(ctx, 1), log.LevelInfo, msg)
}
```

`Caller(depth)` 的深度从 Valuer 求值位置计算，通常直接使用 `DefaultCaller` 即可。

### Valuer 与 Context

`Valuer` 在写日志时求值，适合从 ctx 获取请求字段：

```go
type traceKey struct{}

logger := log.New(os.Stderr).WithFields(
    log.Dynamic("trace_id", func(ctx context.Context) log.Value {
        return log.AnyValue(ctx.Value(traceKey{}))
    }),
)
ctx := context.WithValue(context.Background(), traceKey{}, "abc123")
_ = logger.Log(ctx, log.LevelInfo, "request completed")
```

`Log(ctx, ...)` 使用传入的 ctx，并累加调用 ctx 和 logger context 的 caller depth。
其他日志方法使用 `WithContext(ctx)` 设置的 context。ctx 键值由应用显式选择输出。

## Printer 与全局日志

业务只需要文本日志时，可以传入 `log.Printer`：

```go
printer := log.NewPrinter(logger)
printer.Info("server started")
printer.Infof("listening on %s", ":8080")
_, _ = printer.Write([]byte("message"))
```

Printer 提供 Debug、Info、Warn、Error 的文本和格式化方法，以及 `io.Writer`。
需要结构化字段时使用 Logger。

包级函数使用全局 Logger，默认输出到 stderr，带时间戳和 Caller：

```go
log.Info("server started")
log.SetDefault(log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...))
log.InfoS("ready", "port", 8080)
```

## 接入 log/slog

```go
native := log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...)
logger := slog.New(log.NewSlogHandler(native))
logger.Info("ready", "port", 8080)
```

`NewSlogHandler` 使用创建时的 Logger 配置，后续修改 Logger 不会同步。
时间戳和 Caller 仍通过动态字段配置，不使用 `slog.Record.Time` 或 `PC`。
字段转换、重命名和删除可通过 `HandlerOptions.Replacer` 配置。

## 日志管理

服务中有多个组件、文件输出或命令行配置时，使用 [logmgr](./logmgr/README.zh-CN.md)。

Logger 的 `SetLevel`、`SetOutput`、`SetHandler` 应在并发使用前设置。
运行时调整托管日志可使用 logmgr 的 `Apply`。

## 性能测试

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem
```

测试场景和结果见 [benchmarks](./benchmarks/README.md)。
