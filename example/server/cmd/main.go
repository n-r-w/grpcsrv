package main

import (
	"context"
	"log/slog"

	"github.com/n-r-w/ctxlog"
	srvimpl "github.com/n-r-w/grpcsrv/example/server/implementation"

	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/grpcsrv"
)

const serviceName = "greeter"

func main() {
	// Create the context with logger (slog wrapper with zap backend)
	ctx := ctxlog.MustContext(
		context.Background(),
		ctxlog.WithName(serviceName),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)

	// wrap logger for other package usage
	loggerWrapper := ctxlog.NewWrapper()

	// Create the greeter service
	initializer := srvimpl.NewGreeterInitializer(&srvimpl.GreeterService{})

	// Configure and create the gRPC server
	srv := grpcsrv.New(
		ctx,
		[]grpcsrv.IGRPCInitializer{initializer},
		grpcsrv.WithName(serviceName),
		grpcsrv.WithEndpoint(grpcsrv.Endpoint{
			GRPC: ":50051",
			HTTP: ":50052",
		}),
		grpcsrv.WithRecover(),             // enable panic recovery
		grpcsrv.WithLogger(loggerWrapper), // set external logging interface
	)

	// Start the server
	b, err := bootstrap.New(serviceName)
	if err != nil {
		panic(err)
	}

	err = b.Run(ctx,
		bootstrap.WithOrdered(srv),
		bootstrap.WithLogger(loggerWrapper),
	)
	if err != nil {
		panic(err)
	}
}
