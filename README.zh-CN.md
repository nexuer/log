# log

[English](./README.md) · [API 文档](https://pkg.go.dev/github.com/nexuer/log) · [logmgr](./logmgr/README.zh-CN.md)

Go 日志库，支持普通日志、格式化日志和结构化日志，可输出 Text 或 JSON。
支持公共字段、字段分组，以及在记录日志时从 Context 获取动态字段。

## 特性

- **日志方法清晰**：`Info` 记录普通日志，`Infof` 格式化日志内容，`InfoS` 为本条日志附加字段。
- **动态获取字段**：记录日志时从 Context 读取 trace ID 等信息，无需每次调用都手动传入。时间戳和调用位置也可以按需添加。
- **字段配置灵活**：支持键值对、带类型的字段、字段分组和可复用的公共字段，还可通过 Replacer 转换、脱敏或过滤字段。
- **支持 slog**：提供 `log/slog` Handler，可以继续使用标准库的日志方法，配合本库配置格式和字段。
- **Printer 接口**：向只需打印日志的代码提供普通和格式化日志方法，沿用 Logger 的字段与输出配置，不开放添加字段、修改配置或关闭资源的方法。

## 安装

```sh
go get github.com/nexuer/log
```

## 使用

### 记录日志

```go
logger := log.New(os.Stderr)

logger.Info("服务已启动")
logger.Infof("监听端口 %d", 8080)
logger.InfoS("请求完成", "method", "GET", "status", 200)
```

默认输出 Text 格式。Debug、Info、Warn、Error、Fatal 均提供上述三种日志方法；Fatal 记录日志后退出进程。

### 格式、级别与公共字段

```go
logger := log.New(os.Stderr, log.Json()).
    SetLevel(log.LevelInfo).
    With("service", "api").
    WithFields(log.DefaultFields...)

logger.InfoS("请求完成", log.String("path", "/users"), log.Int("status", 200))
```

新建 Logger 默认不带时间戳和调用位置，可通过 `DefaultFields` 一并添加。
`With` 通过键值对添加公共字段，`WithFields` 接收 `log.Field`，`WithGroup` 用于字段分组。
`log.Err(err)` 添加错误信息，err 为 nil 时不输出该字段。

### Printer

Printer 将日志配置与打印分开：创建方统一配置字段和输出，调用方只记录日志内容，不能添加或覆盖预设字段、修改配置或关闭资源。

```go
logger := log.New(os.Stderr, log.Json()).With("service", "api")
printer := log.NewPrinter(logger)
printer.Info("服务已启动")
printer.Infof("监听端口 %d", 8080)
```

上例的日志始终带有 `service=api`。Printer 支持 Debug、Info、Warn、Error 及格式化方法，也可作为 `io.Writer` 使用。

### 全局日志

```go
log.SetDefault(logger)
log.Info("服务已启动")
log.InfoS("请求完成", "status", 200)
```

全局日志默认写入 stderr，并带有时间戳和调用位置。

### 动态字段与 Context

```go
type traceKey struct{}

logger := log.New(os.Stderr).WithFields(
    log.Dynamic("trace_id", func(ctx context.Context) log.Value {
        return log.AnyValue(ctx.Value(traceKey{}))
    }),
)
ctx := context.WithValue(context.Background(), traceKey{}, "abc123")
_ = logger.Log(ctx, log.LevelInfo, "请求完成")
```

上例通过 `Log(ctx, ...)` 传入 Context，记录日志时才获取 trace ID。
其他日志方法使用 `WithContext(ctx)` 设置的 Context。
如果自行封装日志方法，可通过 `log.AddCallerDepth(ctx, 1)` 补偿包装层数，使调用位置指向业务代码。

### 接入 log/slog

```go
native := log.New(os.Stderr, log.Json()).WithFields(log.DefaultFields...)
logger := slog.New(log.NewSlogHandler(native))
logger.Info("请求完成", "status", 200)
```

Handler 沿用创建时的 Logger 配置，后续修改 Logger 不会同步到 Handler。
时间戳和调用位置仍通过本库的字段配置，字段处理可使用 `log.HandlerOptions.Replacer`。

直接使用 Logger 时，应先完成配置，再交给多个 goroutine 使用。
需要统一管理多个组件的日志时，可使用 [logmgr](./logmgr/README.zh-CN.md)。

## 基准测试

在独立的 benchmarks 模块中运行：

```sh
cd benchmarks
go test -run '^$' -bench=. -benchmem -count=5
```

与 slog、zap、zerolog、phuslu/log、logrus 对比：

```sh
go test -run '^$' -bench='^BenchmarkComparison(EncodeOnly|WritePath)$' -benchmem -count=5
```

测试涵盖 Text / JSON 编码、不同字段的开销、日志被级别过滤时的开销，以及单 goroutine 和多 goroutine 下的表现。
EncodeOnly 主要测量编码开销，WritePath 还包含调用 writer 和输出同步的开销，两者均不测量实际磁盘读写性能。

测试环境、完整结果与更多命令见 [benchmarks](./benchmarks/README.md)。
