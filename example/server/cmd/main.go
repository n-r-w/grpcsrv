//nolint:gocritic // ok
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"

	"github.com/n-r-w/ctxlog"
	srvimpl "github.com/n-r-w/grpcsrv/example/server/implementation"
	"github.com/n-r-w/grpcsrv/example/telemetry"
	"google.golang.org/grpc"

	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/grpcsrv"
)

const serviceName = "greeter-server"

func main() {
	// Initialize the logger
	ctx, loggerWrapper, unaryRequestModifier, streamRequestModifier, httpRequestModifier, err := initLogger(serviceName)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
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

	// Configure and create the gRPC server
	srv := grpcsrv.New(
		ctx,
		[]grpcsrv.IGRPCInitializer{initializer},
		grpcsrv.WithName(serviceName),
		grpcsrv.WithEndpoint(grpcsrv.Endpoint{
			GRPC: ":50051",
			HTTP: ":50052",
		}),
		// grpcsrv.WithRecover(),             // enable panic recovery
		grpcsrv.WithLogger(loggerWrapper), // set external logging interface
		grpcsrv.WithPprof(),               // enable pprof handlers
		grpcsrv.WithContextModifiers(unaryRequestModifier, streamRequestModifier, httpRequestModifier),
	)

	// Start the server
	b, err := bootstrap.New(serviceName)
	if err != nil {
		log.Fatalf("Failed to create bootstrap: %v", err)
	}

	err = b.Run(ctx,
		bootstrap.WithOrdered(srv),
		bootstrap.WithLogger(loggerWrapper),
	)
	if err != nil {
		log.Fatalf("Failed to run bootstrap: %v", err)
	}
}

// initLogger initializes the logging system with the given service name.
// It returns a context with the configured logger, a logger wrapper for external package usage.
func initLogger(serviceName string) (
	context.Context, ctxlog.ILogger, grpcsrv.CtxUnaryModifier, grpcsrv.CtxStreamModifier, grpcsrv.CtxHTTPModifier, error,
) {
	ctx := context.Background()

	// Configure and create the logger context
	var err error
	ctx, err = ctxlog.NewContext(
		ctx,
		ctxlog.WithName(serviceName),
		ctxlog.WithEnvType(ctxlog.EnvProduction),
		ctxlog.WithLevel(slog.LevelDebug),
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// Create logger wrapper for other package usage (implement ctxlog.ILogger interface)
	loggerWrapper := ctxlog.NewWrapper()

	// because we use the logger from ctxlog, which is embedded in the context,
	// so we have to mix it into the context call of the grpc/http methods
	injectLoggerToContext := func(
		ctxRequest context.Context, reqType, method, remoteAddr, traceID string,
	) context.Context {
		if ctxlog.InContext(ctxRequest) {
			return ctxRequest // already injected
		}

		ctxRequest = ctxlog.ToContextFromContext(ctxRequest, ctx)
		ctxRequest = ctxlog.With(ctxRequest,
			"request-type", reqType,
			"method", method,
			"remote-addr", remoteAddr,
			"trace-id", traceID)

		return ctxRequest
	}

	unaryRequestModifier := func(ctxRequest context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler, remoteAddr, traceID string,
	) context.Context {
		return injectLoggerToContext(ctxRequest, "grpc-unary", info.FullMethod, remoteAddr, traceID)
	}
	streamRequestModifier := func(ctxRequest context.Context, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler, remoteAddr, traceID string,
	) context.Context {
		return injectLoggerToContext(ctxRequest, "grpc-stream", info.FullMethod, remoteAddr, traceID)
	}
	httpRequestModifier := func(ctxRequest context.Context, r *http.Request, traceID string) context.Context {
		return injectLoggerToContext(ctxRequest, "http", r.RequestURI, r.RemoteAddr, traceID)
	}

	return ctx, loggerWrapper, unaryRequestModifier, streamRequestModifier, httpRequestModifier, nil
}
