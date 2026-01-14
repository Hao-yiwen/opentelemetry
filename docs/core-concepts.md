# OpenTelemetry 核心概念与代码解析

## 为什么推荐 OpenTelemetry

### 历史背景

在 OpenTelemetry 出现之前，Tracing 和 Metrics 是完全分离的生态：

```
Tracing:  OpenTracing / Jaeger Client / Zipkin Client  → 各有各的 SDK
Metrics:  Prometheus Client / StatsD / DataDog Client → 各有各的 SDK
Logging:  各种日志库，与 trace 无关联
```

**这带来了几个问题**：

1. **SDK 碎片化** - 每个可观测性后端都有自己的 SDK，集成成本高
2. **厂商锁定** - 更换后端（如 Jaeger → Zipkin）需要修改代码
3. **数据孤岛** - Trace、Metric、Log 三者无法关联，排查问题困难

### OpenTelemetry 的解决方案

OpenTelemetry（简称 OTel）是 CNCF 的毕业项目，合并了 OpenTracing 和 OpenCensus，成为可观测性领域的统一标准。

```
            ┌─────────────────────────────────────┐
            │        OpenTelemetry SDK            │
            │       (统一的 API，一次埋点)          │
            └─────────────────────────────────────┘
                              │
                              ▼ OTLP 协议
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
         Jaeger          Prometheus         Grafana
         Zipkin           DataDog           Splunk
         Tempo           New Relic          ...任何后端
```

### 核心优势

| 优势 | 说明 |
|------|------|
| **厂商无关** | 一次埋点，可导出到任何支持 OTLP 的后端 |
| **统一标准** | CNCF 毕业项目，已成为行业事实标准 |
| **三支柱统一** | Trace/Metric/Log 可以通过 context 关联 |
| **语义约定** | 标准化的属性命名（如 `http.method`、`db.system`）|
| **自动埋点** | 丰富的 instrumentation 库，减少手动代码 |

### 与传统方案对比

| 方面 | 传统方案 | OpenTelemetry |
|------|---------|---------------|
| Tracing SDK | `jaeger-client-go` | `go.opentelemetry.io/otel/sdk/trace` |
| Metrics SDK | `prometheus/client_golang` | `go.opentelemetry.io/otel/sdk/metric` |
| 更换后端 | 更换 SDK，修改代码 | 只需更换 Exporter 配置 |
| Trace-Log 关联 | 手动传递 trace_id | 自动通过 context 关联 |
| 标准化程度 | 各家各的规范 | CNCF 统一标准 |

---

## 核心架构

### 三大支柱 (Three Pillars)

OpenTelemetry 统一了可观测性的三大支柱：

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Tracing   │  │   Metrics   │  │   Logging   │
│   (追踪)    │  │   (指标)     │  │   (日志)    │
└─────────────┘  └─────────────┘  └─────────────┘
       │                │                │
       ▼                ▼                ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│TracerProvider│ │MeterProvider│  │LoggerProvider│
└─────────────┘  └─────────────┘  └─────────────┘
       │                │                │
       └────────────────┼────────────────┘
                        ▼
              ┌─────────────────┐
              │    Exporter     │
              │  (OTLP gRPC)    │
              └─────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │     Jaeger      │
              └─────────────────┘
```

### Provider 与 Exporter

```
Provider（提供者）
    │
    ├── 负责创建 Tracer/Meter/Logger 实例
    ├── 管理数据的采样、处理、导出
    └── 应用启动时配置一次，全局使用

Exporter（导出器）
    │
    ├── 决定数据发送到哪里
    ├── 支持多种协议：OTLP gRPC、OTLP HTTP、Stdout 等
    └── 可以同时配置多个 Exporter
```

---

## 代码详解

### 1. SDK 初始化 (`pkg/otel/otel.go`)

这是整个项目的核心，理解它就理解了 OpenTelemetry 的架构。

#### 整体结构

```go
func SetupOTelSDK(ctx context.Context, cfg Config) (func(context.Context) error, error) {
    // 1. 创建 Resource（描述服务身份）
    // 2. 设置 Propagator（跨服务传递 context）
    // 3. 初始化 TracerProvider（追踪）
    // 4. 初始化 MeterProvider（指标）
    // 5. 初始化 LoggerProvider（日志）
    // 6. 返回统一的 shutdown 函数
}
```

#### Resource - 服务身份标识

```go
res, _ := resource.Merge(
    resource.Default(),
    resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName(cfg.ServiceName),      // 服务名称
        semconv.ServiceVersion(cfg.ServiceVersion), // 服务版本
    ),
)
```

**Resource 的作用**：
- 描述产生遥测数据的实体（你的服务）
- 这些信息会附加到每条 trace/metric/log 上
- 在 Jaeger UI 里按服务名筛选，就是依靠 Resource

#### Propagator - 跨服务 Context 传播

```go
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},  // W3C Trace Context 标准
    propagation.Baggage{},       // 自定义数据传递
))
```

**Propagator 的工作原理**：

```
Client 发起请求时：
┌─────────────────────────────────────────────────────────┐
│ HTTP Request                                            │
│ Headers:                                                │
│   traceparent: 00-{trace_id}-{span_id}-01              │
│   baggage: user_id=123                                  │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
Server 接收请求时：
- 解析 traceparent header
- 提取 trace_id 和 parent_span_id
- 新创建的 span 自动关联到同一个 trace
```

#### TracerProvider 配置

```go
func newTracerProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdktrace.TracerProvider, error) {
    // 创建 OTLP gRPC Exporter
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),  // localhost:4317
        otlptracegrpc.WithInsecure(),          // 不使用 TLS（开发环境）
    )
    if err != nil {
        return nil, err
    }

    return sdktrace.NewTracerProvider(
        sdktrace.WithResource(res),                              // 服务身份
        sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second)), // 批量导出
        sdktrace.WithSampler(sdktrace.AlwaysSample()),          // 采样策略
    ), nil
}
```

**关键配置说明**：

| 配置 | 说明 |
|------|------|
| `WithResource` | 附加服务元信息到每个 span |
| `WithBatcher` | 批量发送 span，提高性能（vs `WithSimpleSpanProcessor` 逐条发送）|
| `WithSampler` | 采样策略，生产环境可设置比例采样以控制数据量 |

#### MeterProvider 配置

```go
func newMeterProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
    exporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(endpoint),
        otlpmetricgrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    return sdkmetric.NewMeterProvider(
        sdkmetric.WithResource(res),
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
            sdkmetric.WithInterval(10*time.Second),  // 每 10 秒导出一次
        )),
    ), nil
}
```

#### LoggerProvider 配置

```go
func newLoggerProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdklog.LoggerProvider, error) {
    exporter, err := otlploggrpc.New(ctx,
        otlploggrpc.WithEndpoint(endpoint),
        otlploggrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    return sdklog.NewLoggerProvider(
        sdklog.WithResource(res),
        sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
    ), nil
}
```

---

### 2. 手动创建 Span (`internal/handler/handler.go`)

#### 获取 Tracer

```go
// 参数是 instrumentation scope（通常是包名或模块名）
var tracer = otel.Tracer("otel-demo/handler")
```

#### 创建 Span

```go
func HelloHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // 重要：context 里已有父 span（由 otelhttp 自动创建）

    // 创建子 span
    ctx, span := tracer.Start(ctx, "HelloHandler.process",
        trace.WithAttributes(
            attribute.String("http.method", r.Method),  // 使用语义约定的属性名
            attribute.String("http.path", r.URL.Path),
        ),
    )
    defer span.End()  // 必须调用 End()，否则 span 不会导出

    // 业务逻辑...
}
```

**Span 的生命周期**：

```
tracer.Start()
    │
    ├── 记录开始时间
    ├── 关联父 span（如果 context 中存在）
    ├── 返回新的 context（包含当前 span）
    │
    ▼
span.End()
    │
    ├── 记录结束时间
    ├── 计算耗时
    └── 将 span 加入导出队列
```

#### Span 的父子关系

```go
func RollDiceHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 父 span
    ctx, span := tracer.Start(ctx, "RollDiceHandler.process")
    defer span.End()

    // 调用时传递 ctx
    result := simulateDatabaseCall(ctx)
}

func simulateDatabaseCall(ctx context.Context) int {
    // 子 span - 自动关联到父 span
    _, span := tracer.Start(ctx, "database.query",
        trace.WithAttributes(
            attribute.String("db.system", "postgresql"),
            attribute.String("db.operation", "SELECT"),
        ),
    )
    defer span.End()

    time.Sleep(20 * time.Millisecond)
    return rand.Intn(6) + 1
}
```

**在 Jaeger UI 中的展示**：

```
RollDiceHandler.process (100ms)
└── database.query (20ms)
    ├── db.system: postgresql
    └── db.operation: SELECT
```

#### Span 事件与错误

```go
// 添加事件（记录关键时间点）
span.AddEvent("cache.miss", trace.WithAttributes(
    attribute.String("cache.key", "user:123"),
))

// 记录错误
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}

// 设置额外属性
span.SetAttributes(
    attribute.Int("result.count", len(results)),
)
```

---

### 3. Metrics 记录

#### 指标类型

| 类型 | 说明 | 使用场景 |
|------|------|---------|
| **Counter** | 只增不减的计数器 | 请求总数、错误数、字节数 |
| **UpDownCounter** | 可增可减的计数器 | 活跃连接数、队列长度 |
| **Histogram** | 分布统计 | 请求耗时、响应大小 |
| **Gauge** | 瞬时值（异步） | CPU 使用率、内存占用 |

#### 创建指标

```go
var meter = otel.Meter("otel-demo/handler")

var (
    requestCounter metric.Int64Counter
    requestLatency metric.Float64Histogram
)

func init() {
    var err error

    // Counter - 请求计数
    requestCounter, err = meter.Int64Counter("http.server.requests.total",
        metric.WithDescription("Total number of HTTP requests"),
        metric.WithUnit("{request}"),
    )
    if err != nil {
        panic(err)
    }

    // Histogram - 请求耗时分布
    requestLatency, err = meter.Float64Histogram("http.server.request.duration",
        metric.WithDescription("HTTP request duration in seconds"),
        metric.WithUnit("s"),
    )
    if err != nil {
        panic(err)
    }
}
```

#### 记录指标

```go
func HelloHandler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // ... 业务逻辑 ...

    // 计算耗时
    duration := time.Since(start).Seconds()

    // 定义 attributes（类似 Prometheus 的 labels）
    attrs := metric.WithAttributes(
        attribute.String("http.method", r.Method),
        attribute.String("http.route", "/hello"),
        attribute.Int("http.status_code", http.StatusOK),
    )

    // 记录指标
    requestCounter.Add(ctx, 1, attrs)           // Counter +1
    requestLatency.Record(ctx, duration, attrs) // 记录耗时
}
```

---

### 4. 日志与 Trace 关联

#### 配置 slog Bridge

```go
// 创建 OpenTelemetry slog handler
logger := otelslog.NewLogger("otel-demo-server")
slog.SetDefault(logger)
```

#### 使用 Context 关联日志与 Trace

```go
// 使用 InfoContext 而不是 Info
slog.InfoContext(ctx, "Processing request",
    slog.String("user", "alice"),
    slog.Int("item_count", 5),
)
```

**日志自动携带的信息**：
- `trace_id` - 追踪 ID
- `span_id` - 当前 Span ID
- 服务的 Resource 信息

**关联的价值**：

```
在支持的后端（如 Grafana Loki + Tempo）中：

1. 从 Trace 视图点击 "View Logs"
   → 自动筛选该 trace_id 的所有日志

2. 从 Log 视图点击 "View Trace"
   → 跳转到对应的分布式追踪
```

---

### 5. 自动 HTTP 埋点

#### 服务端 - otelhttp.NewHandler

```go
mux := http.NewServeMux()
mux.HandleFunc("/hello", handler.HelloHandler)

// 包装 handler，自动创建 span
otelHandler := otelhttp.NewHandler(mux, "otel-demo-server",
    otelhttp.WithFilter(func(r *http.Request) bool {
        return r.URL.Path != "/health"  // 跳过健康检查
    }),
)

server := &http.Server{
    Addr:    ":8080",
    Handler: otelHandler,
}
```

**自动记录的信息**：
- Span 名称：`HTTP GET /hello`
- `http.method`: GET
- `http.url`: http://localhost:8080/hello
- `http.status_code`: 200
- `http.request_content_length`
- `http.response_content_length`

#### 客户端 - otelhttp.NewTransport

```go
client := &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
    Timeout:   10 * time.Second,
}

// 发起请求时自动：
// 1. 创建 client span
// 2. 注入 traceparent header（context propagation）
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
resp, _ := client.Do(req)
```

---

## 数据流向图

```
┌─────────────────────────────────────────────────────────────────┐
│                         你的应用代码                             │
├─────────────────────────────────────────────────────────────────┤
│  tracer.Start(ctx, "name")  →  Span 数据                        │
│  counter.Add(ctx, 1)        →  Metric 数据                      │
│  slog.InfoContext(ctx, msg) →  Log 数据                         │
└─────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                      OpenTelemetry SDK                          │
├─────────────────────────────────────────────────────────────────┤
│  BatchSpanProcessor    - 批量处理 Span                          │
│  PeriodicReader        - 定期读取 Metric                        │
│  BatchLogProcessor     - 批量处理 Log                           │
└─────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼ OTLP gRPC (localhost:4317)
┌─────────────────────────────────────────────────────────────────┐
│                    Jaeger (all-in-one)                          │
├─────────────────────────────────────────────────────────────────┤
│  OTLP Receiver  →  存储  →  Query API  →  Jaeger UI            │
└─────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                        http://localhost:16686
                            浏览器查看
```

---

## 最佳实践

### Span 命名规范

```go
// Good - 清晰、有层次
tracer.Start(ctx, "OrderService.CreateOrder")
tracer.Start(ctx, "PaymentGateway.ProcessPayment")
tracer.Start(ctx, "database.query")
tracer.Start(ctx, "cache.get")

// Bad - 太笼统
tracer.Start(ctx, "process")
tracer.Start(ctx, "handle")
tracer.Start(ctx, "do")
```

### Attribute 使用语义约定

OpenTelemetry 定义了标准的属性名称，尽量使用：

```go
import semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

span.SetAttributes(
    semconv.HTTPMethod(r.Method),           // http.method
    semconv.HTTPStatusCode(statusCode),     // http.status_code
    semconv.DBSystemPostgreSQL,             // db.system
    semconv.DBOperation("SELECT"),          // db.operation
)
```

### 错误处理

```go
result, err := doSomething()
if err != nil {
    span.RecordError(err)                           // 记录错误详情
    span.SetStatus(codes.Error, "operation failed") // 设置 span 状态
    return err
}
span.SetStatus(codes.Ok, "")  // 成功时可选设置
```

### 采样策略（生产环境）

```go
// 开发环境 - 全采样
sdktrace.WithSampler(sdktrace.AlwaysSample())

// 生产环境 - 比例采样（10%）
sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1))

// 生产环境 - 父级决定
sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1)))
```

---

## 参考资源

- [OpenTelemetry 官方文档](https://opentelemetry.io/docs/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [语义约定 (Semantic Conventions)](https://opentelemetry.io/docs/specs/semconv/)
- [OTLP 协议规范](https://opentelemetry.io/docs/specs/otlp/)
