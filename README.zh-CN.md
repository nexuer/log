# log

[English](./README.md)

`github.com/nexuer/log` 是一个参考 Go 标准库 `log/slog` 设计的轻量结构化日志库。
它保留了常见的 print、printf 和 structured 三种日志方法，同时支持 `With` 字段的延迟求值。

应用级日志统一管理放在 `logmgr` 子包中。

## 安装

```sh
go get github.com/nexuer/log
```

## 快速开始

```go
package main

import (
	"errors"
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stdout).With(
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	)

	logger.Info("server starting")
	logger.Infof("listening on %s", ":8080")
	logger.InfoS("server started", "addr", ":8080")

	err := errors.New("timeout")
	logger.ErrorS("request failed", log.Err(err), "path", "/api")
}
```

Text 输出：

```text
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:15 msg="server starting"
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:16 msg="listening on :8080"
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:17 msg="server started" addr=:8080
ERROR ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:20 msg="request failed" err=timeout path=/api
```

## 日志方法

每个日志级别都有三种方法：

```go
logger.Info(args ...any)                 // fmt.Sprint 风格
logger.Infof(format string, args ...any) // fmt.Sprintf 风格
logger.InfoS(msg string, kvs ...any)     // 结构化消息和字段
```

`Info` 用于普通文本，`Infof` 用于格式化文本，`InfoS` 用于输出结构化字段：

```go
logger.Info("retry ", attempt, "/", max)
logger.Infof("retry %d/%d", attempt, max)
logger.InfoS("retry", "attempt", attempt, "max", max)
```

`Info(args...)` 不会把键值对解释成字段。需要结构化输出时，请使用 `S` 方法。

## Handler

默认 handler 是 text：

```go
logger := log.New(os.Stdout, log.Text(&log.HandlerOptions{Name: "server"}))
logger.InfoS("ready", "addr", ":8080")
```

输出：

```text
[server] INFO msg=ready addr=:8080
```

使用 `Json` 输出 JSON：

```go
logger := log.New(os.Stdout, log.Json(&log.HandlerOptions{Name: "server"})).With(
	"service", "api",
	"ts", log.DefaultTimestamp,
)

logger.InfoS("ready", "port", 8080)
```

输出：

```json
{"logger":"server","level":"INFO","service":"api","ts":"2026-06-26T17:30:00+08:00","msg":"ready","port":8080}
```

### slog Handler

可以在标准库 `log/slog` API 后使用 Nexuer handler：

```go
native := log.New(os.Stdout, log.Json(&log.HandlerOptions{Name: "server"})).
	SetLevel(log.LevelInfo)
logger := slog.New(log.NewSlogHandler(native))
logger.Info("ready", "port", 8080)
```

`NewSlogHandler` 会取得 Logger 当前输出、级别、handler、字段、group 和 context
的快照；之后对 Logger 的修改不会影响已经返回的 handler。它只输出 `level`、`msg`
和用户字段。
`slog.Record.Time` 和 `slog.Record.PC` 始终会被忽略。
`HandlerOptions.Replacer` 可以转换、重命名或删除用户字段，以及 `level`、`msg`、
`logger` 三个内置字段。返回空 `Field` 会删除字段。包括内置 key 在内，重复 key 仍然允许。

## 字段

推荐优先使用类型化字段：

```go
logger.InfoS("user login",
	log.String("user", "alice"),
	log.Int("attempt", 1),
	log.Bool("ok", true),
)
```

也可以直接使用键值对：

```go
logger.InfoS("user login", "user", "alice", "attempt", 1)
```

`With`、`Fields` 和结构化日志方法可以在同一个参数列表中混用类型化 `Field`、
`slog.Attr`、普通键值对、group 和延迟求值的 `Valuer`，输出顺序与参数顺序一致。
没有配对 value 的参数会使用 `<BAD_KEY>` 作为 key 输出。

每条记录按 logger name、level、`With` 累计字段、msg、本次日志调用字段的顺序输出。
如果 JSON group 同时包含累计字段和本次调用字段，会保持为单一对象，msg 放在该对象之后。

允许重复 key，包括 `level`、`msg` 和 `logger`。Text 会保留每一次出现；JSON
因此可能包含重复的对象成员。很多 JSON 解析器会保留最后一个值，但这取决于解析器实现；
需要跨系统保持唯一语义时应避免重复 key。

使用 `Err` 输出标准错误字段。nil error 会返回空字段，不会被输出：

```go
logger.ErrorS("request failed", log.Err(err), "path", "/api")
```

`Fields` 可以把键值对转换成可复用字段：

```go
fields := log.Fields("service", "api", "region", "cn")
logger := log.New(os.Stdout).WithFields(fields...)
```

## 动态字段

`Valuer` 会在真正写日志时才求值：

```go
logger := log.New(os.Stdout).WithFields(
	log.Dynamic("ts", log.Timestamp(time.RFC3339)),
)
```

Timestamp 和 Caller 都需要显式启用。使用 `DefaultFields` 可以添加标准时间戳，以及
已经按 Logger API 校准过的 Caller：

```go
logger := log.New(os.Stdout).WithFields(log.DefaultFields...)
```

`DefaultFields` 是包提供的只读模板，不能修改，也不能并发修改。

`Caller(depth)` 是底层构造函数。`depth` 从动态值实际求值的位置开始计算栈帧，并非
相对于业务代码调用 Logger 的位置，因此不同日志调用链需要不同的基础值。普通 Logger
用户应直接使用 `DefaultFields`。封装日志调用的包应通过 `AddCallerDepth` 为每一层包装
累加一层：

```go
func Info(ctx context.Context, logger *log.Logger, msg string) {
	logger.Log(log.AddCallerDepth(ctx, 1), log.LevelInfo, msg)
}
```

适合时间戳、调用位置、请求上下文等不应该在 `With` 时提前计算的字段。

## Printer

`Printer` 是一个受限包装器，适合只允许输出普通日志文本的代码。它只暴露 print、printf
和 `io.Writer` 风格的方法，不暴露结构化 `S` 方法、`With`、输出修改、handler 修改、
`Close` 或 `Fatal`。

```go
printer := log.NewPrinter(logger)

printer.Info("server started")
printer.Infof("listening on %s", ":8080")
_, _ = printer.Write([]byte("message from io.Writer"))
```

需要结构化字段时，请直接使用 `Logger`。

## 全局 Logger

```go
log.SetDefault(log.New(os.Stdout).WithFields(log.DefaultFields...))

log.Info("server starting")
log.InfoS("server started", "addr", ":8080")
log.ErrorS("request failed", log.Err(err), "path", "/api")
```

## Fatal

`Fatal`、`Fatalf` 和 `FatalS` 会写入 fatal 级别日志，然后以状态码 `1` 退出进程。

```go
logger.FatalS("listen failed", log.Err(err), "addr", addr)
```

## 日志管理

如果应用需要多个日志实例、统一配置、命令行覆盖或按 scope 分组配置，请使用
`github.com/nexuer/log/logmgr`。

参考 [logmgr/README.zh-CN.md](./logmgr/README.zh-CN.md)。

## 性能测试

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem
```

完整场景说明和最新原始输出在 [benchmarks/README.md](./benchmarks/README.md)。
