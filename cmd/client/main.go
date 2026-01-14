package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	pkgotel "github.com/example/otel-demo/pkg/otel"
)

var tracer = otel.Tracer("otel-demo/client")

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry
	shutdown, err := pkgotel.SetupOTelSDK(ctx, pkgotel.Config{
		ServiceName:    "otel-demo-client",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
	})
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down OpenTelemetry: %v", err)
		}
	}()

	// Set up slog with OpenTelemetry bridge
	logger := otelslog.NewLogger("otel-demo-client")
	slog.SetDefault(logger)

	// Create HTTP client with OpenTelemetry instrumentation
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	serverURL := getEnv("SERVER_URL", "http://localhost:8080")

	slog.Info("Starting client", slog.String("server", serverURL))

	// Make requests in a loop
	for i := 0; i < 5; i++ {
		// Create parent span for the request cycle
		ctx, span := tracer.Start(ctx, "client.request.cycle",
			trace.WithAttributes(
				attribute.Int("request.iteration", i+1),
			),
		)

		slog.InfoContext(ctx, "Making request", slog.Int("iteration", i+1))

		// Call /hello endpoint
		if err := makeRequest(ctx, client, serverURL+"/hello"); err != nil {
			slog.ErrorContext(ctx, "Request failed", slog.String("error", err.Error()))
		}

		// Call /rolldice endpoint
		if err := makeRequest(ctx, client, serverURL+"/rolldice"); err != nil {
			slog.ErrorContext(ctx, "Request failed", slog.String("error", err.Error()))
		}

		span.End()
		time.Sleep(2 * time.Second)
	}

	slog.Info("Client finished")
}

func makeRequest(ctx context.Context, client *http.Client, url string) error {
	ctx, span := tracer.Start(ctx, "http.request",
		trace.WithAttributes(
			attribute.String("http.url", url),
		),
	)
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		span.RecordError(err)
		return err
	}

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.Int("http.response.size", len(body)),
	)

	slog.InfoContext(ctx, "Response received",
		slog.Int("status", resp.StatusCode),
		slog.Any("body", result),
	)

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
