# OpenTelemetry Go 完整示例

一个演示 OpenTelemetry 三大支柱（Tracing、Metrics、Logging）的 Go 语言示例项目。

## 功能特性

- **Tracing**: 分布式追踪，支持嵌套 span 和跨服务 context propagation
- **Metrics**: Counter 和 Histogram 指标收集
- **Logging**: slog 日志与 trace context 自动关联
- **导出**: 使用 OTLP gRPC 经 OpenTelemetry Collector 转发到 Jaeger / Prometheus / Loki

## 环境要求

| 组件 | 版本要求 |
|------|----------|
| Go | **1.27.4+**（`go.mod` 已声明；首次构建会自动下载对应 toolchain）|
| Docker / Docker Compose | 用于启动 Collector、Jaeger、Loki、Prometheus、Grafana |
| 网络 | 本机可访问 `localhost` 上映射的端口（见下方）|

> 应用代码通过 **OTLP** 导出遥测数据，不再使用已废弃的 Jaeger exporter。

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
├── docker-compose.yaml     # Collector / Jaeger / Loki / Prometheus / Grafana
├── otel-collector-config.yaml
├── loki-config.yaml
├── prometheus.yaml
└── README.md
```

## 快速开始

### 1. 启动可观测性后端

```bash
docker compose down && docker compose up -d
```

启动后可用服务：

| 服务 | 地址 | 说明 |
|------|------|------|
| OTel Collector OTLP gRPC | `localhost:4317` | 应用默认导出端点 |
| OTel Collector OTLP HTTP | `localhost:4318` | HTTP 导出（可选）|
| Jaeger UI | http://localhost:16686 | 查看 Trace |
| Prometheus UI | http://localhost:9091 | 查看 Metric |
| Loki API | http://localhost:3100 | Log 存储 |
| Grafana | http://localhost:3000 | 统一可视化（默认 `admin` / `admin`）|

确认 Collector 健康：

```bash
docker compose ps
docker compose logs otel-collector --tail=50
```

### 2. 启动服务端

```bash
go run ./cmd/server
```

服务端监听 `:8080`，提供：

| 接口 | 说明 |
|------|------|
| `GET /hello` | 问候消息，演示 span 与 metrics |
| `GET /rolldice` | 掷骰子，演示嵌套 span（模拟数据库调用）|
| `GET /health` | 健康检查（不记录 trace）|

### 3. 启动客户端

另开一个终端：

```bash
go run ./cmd/client
```

客户端会发送 5 轮请求，演示跨服务分布式追踪。

### 4. 查看数据

1. 打开 Jaeger UI: http://localhost:16686
2. Service 选择 `otel-demo-server` 或 `otel-demo-client`
3. 点击 Find Traces 查看调用链

Prometheus 中可搜索以 `otel_demo_` 为前缀的指标（Collector Prometheus exporter 的 namespace）。

## 手动测试

```bash
curl http://localhost:8080/hello
curl http://localhost:8080/rolldice
```

## 本地校验（无需 Docker）

```bash
go build ./...
go vet ./...
go test ./...
# 可选：漏洞扫描
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

## 核心代码说明

### OpenTelemetry 初始化 (`pkg/otel/otel.go`)

统一初始化三大 Provider，并通过 OTLP gRPC 导出：

```go
tracerProvider, _ := newTracerProvider(ctx, endpoint, res)
otel.SetTracerProvider(tracerProvider)

meterProvider, _ := newMeterProvider(ctx, endpoint, res)
otel.SetMeterProvider(meterProvider)

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

span.AddEvent("event.name")
span.RecordError(err)
```

### 记录 Metrics

```go
requestCounter.Add(ctx, 1, metric.WithAttributes(...))
requestLatency.Record(ctx, duration, metric.WithAttributes(...))
```

### 日志与 Trace 关联

```go
slog.InfoContext(ctx, "message", slog.String("key", "value"))
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC 端点（不含 `http://` 前缀）|
| `SERVER_URL` | `http://localhost:8080` | 服务端地址（客户端使用）|

## 当前依赖与镜像版本

| 项目 | 版本 |
|------|------|
| Go | 1.27.4 |
| OpenTelemetry Go SDK（稳定模块）| v1.47.0 |
| OTLP Log exporter（仍为 experimental）| v0.23.0 |
| otelslog bridge | v0.21.0 |
| otelhttp | v0.72.0 |
| otel/opentelemetry-collector-contrib | 0.165.0 |
| jaegertracing/all-in-one | 1.76.0 |
| grafana/loki | 3.7.10 |
| prom/prometheus | v3.16.0 |
| grafana/grafana | 13.3.1 |

## 参考资源

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [OpenTelemetry Go Contrib](https://github.com/open-telemetry/opentelemetry-go-contrib)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
