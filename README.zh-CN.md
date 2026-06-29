# log

[English](./README.md)

`github.com/nexuer/log` 是参考 Go 标准库 `log/slog` 实现的结构化日志库。`slog`
为结构化日志提供了很好的基础，但它的 `With` 方法不支持字段延迟加载，会导致动态值被提前求值。
当日志属性需要在运行时计算时，例如时间戳、请求 ID 或指标数据，这会降低灵活性和效率。
为了解决这个问题，我们重新实现了这个库。

核心包只保留 logger、handler、field、value 和动态字段能力；应用级日志统一管理放在
`logmgr` 子包中。

## 安装

```sh
go get github.com/nexuer/log
```

## 基础用法

```go
package main

import (
	"os"

	"github.com/nexuer/log"
)

func main() {
	logger := log.New(os.Stdout).With(
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	)

	logger.InfoS("starting server", "addr", ":8080")
	logger.ErrorS(nil, "request failed", "path", "/api")
}
```

Text 输出：

```text
INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:12 msg="starting server" addr=:8080
```

## Text 输出

```go
logger := log.New(os.Stdout, log.Text(&log.HandlerOptions{Name: "server"})).With(
	"ts", log.DefaultTimestamp,
	"caller", log.DefaultCaller,
)

logger.InfoS("starting server", "addr", ":8080")
```

输出：

```text
[server] INFO ts=2026-06-26T17:30:00+08:00 caller=cmd/server.go:12 msg="starting server" addr=:8080
```

## JSON 输出

```go
logger := log.New(os.Stdout, log.Json(&log.HandlerOptions{Name: "server"})).With(
	"service", "api",
	"ts", log.DefaultTimestamp,
)

logger.InfoS("ready", "port", 8080)
```

输出：

```json
{"level":"INFO","logger":"server","service":"api","ts":"2026-06-26T17:30:00+08:00","msg":"ready","port":8080}
```

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

`Fields` 可以把键值对转换成可复用字段：

```go
fields := log.Fields("service", "api", "region", "cn")
logger := log.New(os.Stdout).WithFields(fields...)
```

## 动态字段

`Valuer` 会在真正写日志时才求值：

```go
logger := log.New(os.Stdout).With(
	"ts", log.Timestamp(time.RFC3339),
	"caller", log.Caller(1),
)
```

适合时间戳、调用位置、请求上下文等不应该在 `With` 时提前计算的字段。

## 全局 Logger

```go
log.SetDefault(log.New(os.Stdout).With(log.DefaultFields...))

log.InfoS("server started", "addr", ":8080")
log.ErrorS(err, "request failed", "path", "/api")
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
