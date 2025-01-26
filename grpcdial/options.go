package grpcdial

import (
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/n-r-w/ctxlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Option - function for configuring targetInfo.
type Option func(*targetInfo)

// WithLogger sets logger for dialer.
func WithLogger(logger ctxlog.ILogger) Option {
	return func(g *targetInfo) {
		g.logger = logger
	}
}

// WithCredentials sets TLS configuration for connecting to gRPC server.
// If not set, insecure.NewCredentials() is used.
func WithCredentials(creds credentials.TransportCredentials) Option {
	return func(g *targetInfo) {
		g.creds = creds
	}
}

// WithUnaryInterceptors sets list of UnaryClientInterceptor for gRPC client. Optional.
func WithUnaryInterceptors(interceptors ...grpc.UnaryClientInterceptor) Option {
	return func(g *targetInfo) {
		g.unaryInterceptors = interceptors
	}
}

// WithStreamInterceptors sets list of StreamClientInterceptor for gRPC client. Optional.
func WithStreamInterceptors(interceptors ...grpc.StreamClientInterceptor) Option {
	return func(g *targetInfo) {
		g.streamInterceptors = interceptors
	}
}

// WithRetryOptions sets list of CallOption for gRPC client.
// Use either WithRetryOptions or WithClientDefaultRetryOptions.
// If neither is set, default settings are used: 3 retries, 1 second between retries.
func WithRetryOptions(opts ...grpc_retry.CallOption) Option {
	return func(g *targetInfo) {
		g.retryOpts = opts
	}
}

// WithDefaultRetryOptions sets default retry parameters for gRPC client.
// Use either WithClientRetryOptions or WithDefaultRetryOptions.
// If neither is set, default settings are used:
// maxRetries: 3 retries
// requestTimeout: maximum 10 seconds per request
// retryTimeout: 1 second between retries.
func WithDefaultRetryOptions(maxRetries int, requestTimeout, retryTimeout time.Duration) Option {
	return func(g *targetInfo) {
		g.retryOpts = nil

		g.maxRetries = maxRetries
		g.requestTimeout = requestTimeout
		g.retryTimeout = retryTimeout
	}
}

type targetInfo struct {
	creds              credentials.TransportCredentials
	unaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor

	retryOpts      []grpc_retry.CallOption
	maxRetries     int
	requestTimeout time.Duration
	retryTimeout   time.Duration
	logger         ctxlog.ILogger
}
