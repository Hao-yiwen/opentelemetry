# OpenTelemetry Go 完整示例

一个演示 OpenTelemetry 三大支柱（Tracing、Metrics、Logging）的 Go 语言示例项目。

## 功能特性

- **Tracing**: 分布式追踪，支持嵌套 span 和跨服务 context propagation
- **Metrics**: Counter 和 Histogram 指标收集
- **Logging**: slog 日志与 trace context 自动关联
- **导出**: 使用 OTLP gRPC 协议导出到 Jaeger

## 项目结构

```
.
├── cmd/
│   ├── server/main.go      # HTTP 服务端
│   └── client/main.go      # HTTP 客户端
├── internal/
│   └── handler/handler.go  # HTTP 处理器（业务逻辑）
├── pkg/
│   └── otel/otel.go        # OpenTelemetry SDK 初始化
├── docker-compose.yaml     # Jaeger 容器配置
└── README.md
```

## 快速开始

### 1. 启动 Jaeger

```bash
docker-compose up -d
```

### 2. 启动服务端

```bash
go run cmd/server/main.go
```

服务端启动后监听 `:8080` 端口，提供以下接口：

| 接口 | 说明 |
|------|------|
| `GET /hello` | 返回问候消息，演示基本 span 和 metrics |
| `GET /rolldice` | 掷骰子，演示嵌套 span（模拟数据库调用）|
| `GET /health` | 健康检查（不记录 trace）|

### 3. 启动客户端

```bash
go run cmd/client/main.go
```

客户端会自动发送 5 轮请求，演示跨服务的分布式追踪。

### 4. 查看追踪数据

打开 Jaeger UI: http://localhost:16686

- 在 Service 下拉菜单选择 `otel-demo-server` 或 `otel-demo-client`
- 点击 "Find Traces" 查看追踪数据
- 点击具体 trace 查看调用链和 span 详情

## 手动测试

```bash
# 测试 /hello 接口
curl http://localhost:8080/hello

# 测试 /rolldice 接口
curl http://localhost:8080/rolldice
```

## 核心代码说明

### OpenTelemetry 初始化 (`pkg/otel/otel.go`)

统一初始化三大 Provider：

```go
// TracerProvider - 追踪
tracerProvider, _ := newTracerProvider(ctx, endpoint, res)
otel.SetTracerProvider(tracerProvider)

// MeterProvider - 指标
meterProvider, _ := newMeterProvider(ctx, endpoint, res)
otel.SetMeterProvider(meterProvider)

// LoggerProvider - 日志
loggerProvider, _ := newLoggerProvider(ctx, endpoint, res)
global.SetLoggerProvider(loggerProvider)
```

### 手动创建 Span (`internal/handler/handler.go`)

```go
ctx, span := tracer.Start(ctx, "operation.name",
    trace.WithAttributes(
        attribute.String("key", "value"),
    ),
)
defer span.End()

// 添加事件
span.AddEvent("event.name")

// 记录错误
span.RecordError(err)
```

### 记录 Metrics

```go
// Counter
requestCounter.Add(ctx, 1, metric.WithAttributes(...))

// Histogram
requestLatency.Record(ctx, duration, metric.WithAttributes(...))
```

### 日志与 Trace 关联

```go
// 使用 slog 的 InfoContext，自动关联 trace
slog.InfoContext(ctx, "message", slog.String("key", "value"))
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC 端点 |
| `SERVER_URL` | `http://localhost:8080` | 服务端地址（客户端使用）|

## 依赖版本

- Go 1.22+
- OpenTelemetry Go SDK v1.33.0
- OpenTelemetry Go Contrib v0.58.0
- Jaeger v1.64.0

## 参考资源

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [OpenTelemetry Go Contrib](https://github.com/open-telemetry/opentelemetry-go-contrib)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
