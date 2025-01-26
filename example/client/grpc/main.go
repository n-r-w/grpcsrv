//nolint:mnd,gocritic // ok
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/n-r-w/ctxlog"
	"github.com/n-r-w/grpcsrv"
	api "github.com/n-r-w/grpcsrv/example/protogen"
	"github.com/n-r-w/grpcsrv/example/telemetry"
	"github.com/n-r-w/grpcsrv/grpcdial"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const serviceName = "greeter-grpc-client"

func main() {
	ctx := setupContext()

	// Initialize connection and client
	client, cleanup, err := setupGRPCClient(ctx)
	if err != nil {
		log.Fatalf("Failed to setup gRPC client: %v", err)
	}
	defer cleanup()

	// Set timeout for all operations
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var span trace.Span
	ctx, span = otel.Tracer("").Start(ctx, serviceName)
	defer span.End()

	// Execute unary call
	if err := handleUnaryCall(ctx, client); err != nil {
		log.Fatalf("Unary call failed: %v", err)
	}

	// Execute streaming call
	if err := handleStreamingCall(ctx, client); err != nil {
		log.Fatalf("Streaming call failed: %v", err)
	}
}

// setupContext initializes the context with logger and required configurations.
func setupContext() context.Context {
	return ctxlog.MustContext(
		context.Background(),
		ctxlog.WithName(serviceName),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)
}

// setupGRPCClient initializes the gRPC connection and client.
// Returns the client, cleanup function, and any error.
func setupGRPCClient(ctx context.Context) (api.GreeterClient, func(), error) {
	loggerWrapper := ctxlog.NewWrapper()

	// Initialize tracing
	cleanupTracer, err := telemetry.InitTracer(ctx, serviceName)
	if err != nil {
		return nil, nil, err
	}

	// Create a dialer with default settings
	dialer := grpcdial.New(ctx,
		grpcdial.WithLogger(loggerWrapper),
	)

	// Connect to the gRPC server with insecure credentials
	conn, err := dialer.Dial(ctx, "localhost:50051", "greeter-client",
		grpcdial.WithCredentials(insecure.NewCredentials()),
		grpcdial.WithDefaultRetryOptions(3, time.Second*10, time.Second))
	if err != nil {
		_ = cleanupTracer(ctx)
		return nil, nil, err
	}

	// Create cleanup function that handles both tracer and connection
	cleanup := func() {
		_ = dialer.Stop(ctx)
		_ = cleanupTracer(ctx)
	}

	return api.NewGreeterClient(conn), cleanup, nil
}

// handleUnaryCall performs a unary gRPC call to the SayHello endpoint.
// Returns any error that occurred during the call.
func handleUnaryCall(ctx context.Context, client api.GreeterClient) error {
	var trailer metadata.MD
	resp, err := client.SayHello(ctx,
		&api.HelloRequest{Name: "World"},
		grpc.Trailer(&trailer))
	if err != nil {
		return err
	}

	// Extract trace ID from trailer
	var traceID string
	if trailers := trailer.Get(grpcsrv.TraceIDKey); len(trailers) > 0 {
		traceID = trailers[0]
	}

	// Log response with trace ID
	ctxlog.Info(ctx, "Unary Response",
		"message", resp.GetMessage(),
		"timestamp", resp.GetTimestamp(),
		grpcsrv.TraceIDKey, traceID)

	return nil
}

// handleStreamingCall performs a streaming gRPC call to the SayManyHellos endpoint.
// Returns any error that occurred during the streaming operation.
func handleStreamingCall(ctx context.Context, client api.GreeterClient) error {
	var trailer metadata.MD
	stream, err := client.SayManyHellos(ctx,
		&api.HelloRequest{Name: "Streaming World"},
		grpc.Trailer(&trailer))
	if err != nil {
		return err
	}

	ctxlog.Info(ctx, "Streaming responses:")
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Get trace ID after stream is fully consumed
			var traceID string
			if trailers := trailer.Get(grpcsrv.TraceIDKey); len(trailers) > 0 {
				traceID = trailers[0]
			}
			ctxlog.Info(ctx, "Stream completed",
				grpcsrv.TraceIDKey, traceID)
			break
		}
		if err != nil {
			return err
		}

		ctxlog.Info(ctx, "Stream Response",
			"message", resp.GetMessage(),
			"timestamp", resp.GetTimestamp())
	}

	return nil
}
