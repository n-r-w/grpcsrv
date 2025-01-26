//nolint:mnd,gocritic // ok
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/n-r-w/ctxlog"
	"github.com/n-r-w/grpcsrv"
	api "github.com/n-r-w/grpcsrv/example/protogen"
	"github.com/n-r-w/grpcsrv/example/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type helloResponse struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

const serviceName = "greeter-http-client"

func main() {
	// Create the context with logger (slog wrapper with zap backend)
	ctx := ctxlog.MustContext(
		context.Background(),
		ctxlog.WithName(serviceName),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	// Initialize tracing
	cleanupTracer, err := telemetry.InitTracer(ctx, serviceName)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer func() {
		_ = cleanupTracer(ctx)
	}()

	var span trace.Span
	ctx, span = otel.Tracer("").Start(ctx, serviceName)
	defer span.End()

	// Test unary call
	if err := makeUnaryRequest(ctx, client); err != nil {
		log.Fatal(err)
	}

	// Test streaming call
	if err := makeStreamingRequest(ctx, client); err != nil {
		log.Fatal(err)
	}
}

func makeUnaryRequest(ctx context.Context, client *http.Client) error {
	reqBody := &api.HelloRequest{Name: "World"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"http://localhost:50052/v1/greeter:SayHello",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Inject trace context into request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req) //nolint:bodyclose // false positive
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer ctxlog.CloseError(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body) //nolint:govet //ok
		if err != nil {
			return fmt.Errorf("server returned error %d and failed to read body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("server returned error: %d, body: %s", resp.StatusCode, string(body))
	}

	var helloResp helloResponse
	if err = json.NewDecoder(resp.Body).Decode(&helloResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	ctxlog.Info(ctx, "Unary Response",
		"message", helloResp.Message,
		"timestamp", helloResp.Timestamp,
		grpcsrv.TraceIDKey, resp.Header.Get(grpcsrv.TraceIDKey),
	)
	return nil
}

func makeStreamingRequest(ctx context.Context, client *http.Client) error {
	reqBody := &api.HelloRequest{Name: "Streaming World"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"http://localhost:50052/v1/greeter:SayManyHellos",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Inject trace context into request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req) //nolint:bodyclose // false positive
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer ctxlog.CloseError(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("server returned error %d and failed to read body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("server returned error: %d, body: %s", resp.StatusCode, string(body))
	}

	ctxlog.Info(ctx, "Streaming responses:")

	// Read the response line by line as it streams
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var streamResp helloResponse
		if err := json.Unmarshal(scanner.Bytes(), &streamResp); err != nil {
			ctxlog.Info(ctx, "Failed to decode stream response", "error", err, "response", scanner.Text())
			continue
		}
		ctxlog.Info(ctx, "Stream Response",
			"message", streamResp.Message,
			"timestamp", streamResp.Timestamp,
			grpcsrv.TraceIDKey, resp.Header.Get(grpcsrv.TraceIDKey),
		)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}
