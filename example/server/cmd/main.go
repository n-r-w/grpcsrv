//nolint:gocritic // ok
package main

import (
	"context"
	"log"
	"log/slog"

	"github.com/n-r-w/ctxlog"
	srvimpl "github.com/n-r-w/grpcsrv/example/server/implementation"
	"github.com/n-r-w/grpcsrv/example/telemetry"

	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/grpcsrv"
)

const serviceName = "greeter-server"

func main() {
	// Initialize the logger
	// Configure and create the logger context
	ctx, err := ctxlog.NewContext(
		context.Background(),
		ctxlog.WithName(serviceName),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Initialize tracing
	cleanupTracer, err := telemetry.InitTracer(ctx, serviceName)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer func() {
		_ = cleanupTracer(ctx)
	}()

	// Create the greeter service
	initializer := srvimpl.NewGreeterInitializer(&srvimpl.GreeterService{})

	// Get logger options for grpc and http servers
	opts, err := grpcsrv.GetCtxLogOptions(ctx)
	if err != nil {
		log.Fatalf("Failed to get context logger options: %v", err)
	}

	// Configure and create the gRPC server
	opts = append(opts,
		grpcsrv.WithName(serviceName),
		grpcsrv.WithEndpoint(grpcsrv.Endpoint{
			GRPC: ":50051",
			HTTP: ":50052",
		}),
		grpcsrv.WithPprof(),
		// grpcsrv.WithRecover(), // enable panic recovery
	)
	srv := grpcsrv.New(ctx, []grpcsrv.IGRPCInitializer{initializer}, opts...)

	// Start the server
	b, err := bootstrap.New(serviceName)
	if err != nil {
		log.Fatalf("Failed to create bootstrap: %v", err)
	}

	err = b.Run(ctx,
		bootstrap.WithOrdered(srv),
		bootstrap.WithLogger(ctxlog.NewWrapper()),
	)
	if err != nil {
		log.Fatalf("Failed to run bootstrap: %v", err)
	}
}
