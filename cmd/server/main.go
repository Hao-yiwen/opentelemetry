package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/example/otel-demo/internal/handler"
	"github.com/example/otel-demo/pkg/otel"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize OpenTelemetry
	shutdown, err := otel.SetupOTelSDK(ctx, otel.Config{
		ServiceName:    "otel-demo-server",
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
	logger := otelslog.NewLogger("otel-demo-server")
	slog.SetDefault(logger)

	// Set up HTTP server with instrumentation
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", handler.HelloHandler)
	mux.HandleFunc("/rolldice", handler.RollDiceHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with OpenTelemetry HTTP instrumentation
	otelHandler := otelhttp.NewHandler(mux, "otel-demo-server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health" // Skip health checks
		}),
	)

	server := &http.Server{
		Addr:    ":8080",
		Handler: otelHandler,
	}

	// Start server in goroutine
	go func() {
		slog.Info("Starting HTTP server", slog.String("addr", ":8080"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	slog.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	slog.Info("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
