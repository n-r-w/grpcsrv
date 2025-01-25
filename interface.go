package grpcsrv

//go:generate mockgen -source interface.go -destination interface_mock.go -package grpcsrv

import (
	"context"
	"net/http"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// InitializeOptions options for gRPC server initialization.
type InitializeOptions struct {
	GRPCUnaryInterceptors  []grpc.UnaryServerInterceptor  // gRPC unary interceptors
	GRPCStreamInterceptors []grpc.StreamServerInterceptor // gRPC stream interceptors
	GRPCOptions            []grpc.ServerOption            // gRPC options
	
	HTTPHandlerRequired bool // whether HTTP handler is required that will proxy requests to gRPC server
}

// IGRPCInitializer interface for gRPC server initialization.
type IGRPCInitializer interface {
// RegisterGRPCServer registers the gRPC server.
	RegisterGRPCServer(*grpc.Server)
	// RegisterHTTPHandler registers HTTP handler.
	RegisterHTTPHandler(context.Context, *grpc_runtime.ServeMux, *grpc.ClientConn) error
	// GetOptions returns options for gRPC server initialization.
	GetOptions() InitializeOptions
}

// IHealther allows adding liveness and readiness checks.
type IHealther interface {
// LiveEndpoint is an HTTP handler only for the /liveness endpoint, which
// is useful if you need to add it to your own HTTP handler tree.
	LiveEndpoint(http.ResponseWriter, *http.Request)

	// ReadyEndpoint is an HTTP handler only for the /readiness endpoint, which
	// is useful if you need to add it to your own HTTP handler tree.
	ReadyEndpoint(http.ResponseWriter, *http.Request)
}
