//nolint:mnd // ok
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/n-r-w/ctxlog"
	api "github.com/n-r-w/grpcsrv/example/protogen"
	"github.com/n-r-w/grpcsrv/grpcdial"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Create the context with logger (slog wrapper with zap backend)
	ctx := ctxlog.MustContext(
		context.Background(),
		ctxlog.WithName("greeter-client"),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)

	// wrap logger for other package usage
	loggerWrapper := ctxlog.NewWrapper()

	// Create a dialer with default settings
	dialer := grpcdial.New(ctx, grpcdial.WithLogger(loggerWrapper))

	// Connect to the gRPC server with insecure credentials
	conn, err := dialer.Dial(ctx, "localhost:50051", "greeter-client",
		grpcdial.WithCredentials(insecure.NewCredentials()),
		grpcdial.WithDefaultRetryOptions(3, time.Second*10, time.Second))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = dialer.Stop(ctx) }()

	// Create a client
	client := api.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	// Test unary call
	resp, err := client.SayHello(ctx, &api.HelloRequest{Name: "World"})
	if err != nil {
		log.Fatalf("SayHello failed: %v", err) //nolint:gocritic // ok
	}
	ctxlog.Info(ctx, "Unary Response", "message", resp.GetMessage(), "timestamp", resp.GetTimestamp())

	// Test streaming call
	stream, err := client.SayManyHellos(ctx, &api.HelloRequest{Name: "Streaming World"})
	if err != nil {
		log.Fatalf("SayManyHellos failed: %v", err)
	}

	ctxlog.Info(ctx, "Streaming responses:")
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("Failed to receive: %v", err)
		}
		ctxlog.Info(ctx, "Stream Response", "message", resp.GetMessage(), "timestamp", resp.GetTimestamp())
	}
}
