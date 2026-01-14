package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer         = otel.Tracer("otel-demo/handler")
	meter          = otel.Meter("otel-demo/handler")
	requestCounter metric.Int64Counter
	requestLatency metric.Float64Histogram
)

func init() {
	var err error
	requestCounter, err = meter.Int64Counter("http.server.requests.total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		panic(err)
	}

	requestLatency, err = meter.Float64Histogram("http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(err)
	}
}

// HelloHandler handles /hello endpoint
func HelloHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	// Create a child span for business logic
	ctx, span := tracer.Start(ctx, "HelloHandler.process",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
		),
	)
	defer span.End()

	// Log with trace context
	slog.InfoContext(ctx, "Processing hello request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	// Record metrics
	duration := time.Since(start).Seconds()
	attrs := metric.WithAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", "/hello"),
		attribute.Int("http.status_code", http.StatusOK),
	)
	requestCounter.Add(ctx, 1, attrs)
	requestLatency.Record(ctx, duration, attrs)

	// Add span event
	span.AddEvent("response.prepared", trace.WithAttributes(
		attribute.String("message", "Hello, OpenTelemetry!"),
	))

	// Respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Hello, OpenTelemetry!",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// RollDiceHandler handles /rolldice endpoint (demonstrates nested spans)
func RollDiceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "RollDiceHandler.process")
	defer span.End()

	// Simulate database call with child span
	result := simulateDatabaseCall(ctx)

	slog.InfoContext(ctx, "Dice rolled",
		slog.Int("result", result),
	)

	span.SetAttributes(attribute.Int("dice.result", result))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": result})
}

func simulateDatabaseCall(ctx context.Context) int {
	_, span := tracer.Start(ctx, "database.query",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()

	// Simulate database latency
	time.Sleep(20 * time.Millisecond)

	span.AddEvent("query.executed")

	return rand.Intn(6) + 1 // Random 1-6
}
