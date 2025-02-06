package grpcsrv

import (
	"context"
	"net/http"
	"time"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/moznion/go-optional"
	"github.com/n-r-w/ctxlog"
	"github.com/rs/cors"
	"google.golang.org/grpc"
)

type (
	// CtxUnaryModifier function for adding additional data to context when calling unary handler.
	CtxUnaryModifier func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler, remoteAddr, traceID string) context.Context
	// CtxStreamModifier function for adding additional data to context when calling stream handler.
	CtxStreamModifier func(ctx context.Context, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler, remoteAddr, traceID string) context.Context
	// CtxHTTPModifier function for adding additional data to context when processing HTTP request.
	CtxHTTPModifier func(ctx context.Context, r *http.Request, traceID string) context.Context
	// RegisterHTTPEndpoints function for registering additional endpoints.
	RegisterHTTPEndpoints func(ctx context.Context, mux *grpc_runtime.ServeMux) error
)

// Option option for service initialization.
type Option func(*Service)

// Endpoint hosts for gRPC and HTTP servers.
type Endpoint struct {
	GRPC string
	HTTP string
}

// WithEndpoint sets hosts for gRPC and HTTP servers.
func WithEndpoint(endpoint Endpoint) Option {
	return func(s *Service) {
		s.endpoint = endpoint
	}
}

// WithHTTPReadHeaderTimeout sets timeout for reading HTTP request headers.
func WithHTTPReadHeaderTimeout(timeout time.Duration) Option {
	return func(s *Service) {
		s.httpReadHeaderTimeout = timeout
	}
}

// WithGRPCInitializers sets gRPC server initializers.
func WithGRPCInitializers(initializers ...IGRPCInitializer) Option {
	return func(s *Service) {
		s.grpcInitializers = append(s.grpcInitializers, initializers...)
	}
}

// WithGRPCOptions sets options for gRPC server.
func WithGRPCOptions(options ...grpc.ServerOption) Option {
	return func(s *Service) {
		s.grpcOptions = append(s.grpcOptions, options...)
	}
}

// WithHealthCheck sets handler for service health checks.
func WithHealthCheck(handler IHealther, livenessHandlerPath, readinessHandlerPath string) Option {
	return func(s *Service) {
		if handler != nil && (livenessHandlerPath == "" || readinessHandlerPath == "") {
			panic("livenessHandlerPath and readinessHandlerPath must not be empty")
		}

		s.healthCheckHandler = handler

		if s.healthCheckHandler == nil {
			s.livenessHandlerPath = ""
			s.readinessHandlerPath = ""
		} else {
			s.livenessHandlerPath = livenessHandlerPath
			s.readinessHandlerPath = readinessHandlerPath
		}
	}
}

// WithName sets the service name.
func WithName(name string) Option {
	return func(s *Service) {
		s.name = "grpc-" + name
	}
}

// WithRecover sets handler for panic recovery. Recommended for production.
func WithRecover() Option {
	return func(s *Service) {
		s.recoverEnabled = true
	}
}

// WithHTTPFileSupport enables file upload/download support through HTTP gateway.
// Warning! Sets grpc stream delimiter to empty value,
// therefore httpFileSupport cannot be used together with regular grpc stream methods.
func WithHTTPFileSupport() Option {
	return func(s *Service) {
		s.httpFileSupport = true
	}
}

// WithHTTPDialOptions sets options for HTTP gateway client when connecting to gRPC endpoint.
// If not set, grpc.WithTransportCredentials(insecure.NewCredentials()) is used.
func WithHTTPDialOptions(options ...grpc.DialOption) Option {
	return func(s *Service) {
		s.httpDialOptions = append(s.httpDialOptions, options...)
	}
}

// WithHTTPMarshallers sets marshallers for HTTP gateway.
// marshallers: content-type -> marshaler.
func WithHTTPMarshallers(marshallers map[string]grpc_runtime.Marshaler) Option {
	return func(s *Service) {
		s.httpMarshallers = marshallers
	}
}

// WithHTTPHeadersFromMetadata passes specified gRPC metadata to headers
// For example, if you need a Location header in response, adding such metadata
// will result in a Grpc-Metadata-Location header.
func WithHTTPHeadersFromMetadata(headers ...string) Option {
	return func(s *Service) {
		s.httpHeadersFromMetadata = headers
	}
}

// WithCORSOptions sets options for CORS.
func WithCORSOptions(options cors.Options) Option {
	return func(s *Service) {
		s.corsOptions = optional.Some(options)
	}
}

// WithMetrics sets endpoint for prometheus metrics server.
func WithMetrics(endpoint string) Option {
	return func(s *Service) {
		s.metricsEndpoint = endpoint
	}
}

// WithPprof enables pprof support.
func WithPprof(endpoint string) Option {
	return func(s *Service) {
		s.pprofEndpoint = endpoint
	}
}

// WithLogger sets logger.
func WithLogger(logger ctxlog.ILogger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithPanicLogger sets function for panic logging (logging only, not recovery).
// If not set, standard logic is used.
func WithPanicLogger(panicLogger func(ctx context.Context, p any)) Option {
	return func(s *Service) {
		s.panicLogger = panicLogger
	}
}

// WithContextModifiers sets function for enriching context before calling handlers.
// For example, for setting logger or config in context.
func WithContextModifiers(
	ctxUnaryModifier CtxUnaryModifier, ctxStreamModifier CtxStreamModifier, ctxHTTPModifier CtxHTTPModifier,
) Option {
	return func(s *Service) {
		s.ctxUnaryModifier = ctxUnaryModifier
		s.ctxStreamModifier = ctxStreamModifier
		s.ctxHTTPModifier = ctxHTTPModifier
	}
}

// WithRegisterHTTPEndpoints registers additional HTTP endpoints.
func WithRegisterHTTPEndpoints(registerHealthCheckEndpoints RegisterHTTPEndpoints) Option {
	return func(s *Service) {
		s.registerHTTPEndpoints = registerHealthCheckEndpoints
	}
}

// WithSanitizeKeys sets list of keys whose values will be replaced with "sanitized" in logs and spans.
// Default: password, token, refreshToken, accessToken.
func WithSanitizeKeys(keys ...string) Option {
	return func(s *Service) {
		s.sanitizeKeys = keys
	}
}
