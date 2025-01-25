package grpcsrv

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/rs/cors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	// MaxSpanBytes maximum message size in bytes that will be sent in the span.
	MaxSpanBytes = 64000

	// traceIDKey key for traceID in response metadata.
	traceIDKey = "x-trace-id"
	// traceDebugKey key for debug information in response metadata.
	// If value equals 1, the entire request and response will be logged.
	traceDebugKey = "x-trace-debug"
	// traceDebugKeyValue value for traceDebugKey.
	traceDebugKeyValue = "1"
)

// TraceIDFromContext returns traceID from context.
func (s *Service) traceIDFromContext(ctx context.Context) (string, bool) {
	span := trace.SpanFromContext(ctx).SpanContext()
	if span.HasTraceID() {
		return span.TraceID().String(), true
	}

	return "", false
}

// interceptor for incoming gRPC requests.
func (s *Service) callServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	// add traceID to response metadata
	traceID, traceOK := s.traceIDFromContext(ctx)
	if traceOK {
		header := metadata.Pairs(traceIDKey, traceID)
		_ = grpc.SetTrailer(ctx, header)
	}

	// add additional data to context
	ctx = s.ctxUnaryModifier(ctx, req, info, handler, extractRemoteAddr(ctx))

	resp, err = handler(ctx, req)
	if err != nil {
		s.logger.Debug(ctx, "grpc server error", "error", err)
	}

	return resp, err
}

// interceptor for tracing stream request calls.
func (s *Service) callServerStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	wrapped := grpc_middleware.WrapServerStream(ss)

	// add additional data to context
	wrapped.WrappedContext = s.ctxStreamModifier(ss.Context(), info, handler, extractRemoteAddr(ss.Context()))

	err := handler(srv, wrapped)
	if err != nil {
		s.logger.Debug(ss.Context(), "grpc server stream error", "error", err)
	}

	return err
}

// creates span for gRPC request and adds request and response to it.
func (s *Service) tracingDataServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	// check for debug header requirement
	needDebug := false
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(traceDebugKey); len(v) > 0 && v[0] == traceDebugKeyValue {
			needDebug = true
		}
	}

	if !needDebug {
		return handler(ctx, req)
	}

	var span trace.Span
	ctx, span = otel.GetTracerProvider().Tracer("").Start(ctx, "grpc_data")
	defer span.End()

	tagRemoteAddr(ctx, span)

	var (
		reqMessage protoreflect.ProtoMessage
		ok         bool
	)
	if reqMessage, ok = req.(protoreflect.ProtoMessage); ok {
		if reqBytes, err := protojson.Marshal(reqMessage); err == nil {
			if len(reqBytes) < MaxSpanBytes {
				span.SetAttributes(attribute.String("grpc_request", string(s.sanitizeBytes(reqBytes))))
			}
		}
	}

	resp, rpcErr := handler(ctx, req)

	if rpcErr == nil { //nolint:nestif // ok
		if reqMessage, ok = resp.(protoreflect.ProtoMessage); ok {
			if replyBytes, err := protojson.Marshal(reqMessage); err == nil {
				if len(replyBytes) > MaxSpanBytes {
					replyBytes = replyBytes[:MaxSpanBytes]
				}
				span.SetAttributes(attribute.String("grpc_response", string(s.sanitizeBytes(replyBytes))))
			}
		}
	}

	return resp, rpcErr
}

// removes values of keys from sanitizeKeys in JSON.
func (s *Service) sanitizeBytes(data []byte) []byte {
	var (
		m   map[string]any
		err error
	)

	if err = json.Unmarshal(data, &m); err != nil {
		return data
	}

	s.sanitizeJSON(m)

	if data, err = json.Marshal(m); err != nil {
		return data
	}

	return data
}

// removes values of keys from sanitizeKeys in JSON.
func (s *Service) sanitizeJSON(data map[string]any) {
	for key, value := range data {
		switch v := value.(type) {
		case map[string]any:
			s.sanitizeJSON(v)
		case []any:
			for i := range v {
				if m, ok := v[i].(map[string]any); ok {
					s.sanitizeJSON(m)
				}
			}
		case string:
			for _, k := range s.sanitizeKeys {
				if strings.EqualFold(key, k) {
					data[key] = "sanitized"
				}
			}
		}
	}
}

// extracts IP address from context.
func extractRemoteAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		if host, _, err := net.SplitHostPort(p.Addr.String()); err == nil {
			return host
		}
	}
	return ""
}

// adds IP address to span.
func tagRemoteAddr(ctx context.Context, span trace.Span) {
	if host := extractRemoteAddr(ctx); host != "" {
		span.SetAttributes(attribute.String("remote_addr", host))
	}
}

// adds traceID to HTTP response metadata.
func (s *Service) setTraceIDHTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if traceID, traceOK := s.traceIDFromContext(r.Context()); traceOK {
			w.Header().Set(traceIDKey, traceID)
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Service) setLoggerHTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// add additional data to context
		ctx := s.ctxHTTPModifier(r.Context(), r)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// setTraceRouteHTTPMiddleware adds request URI to trace attributes taken from context.
func (s *Service) setTraceRouteHTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("http.request.uri", r.RequestURI))
		ctx := trace.ContextWithSpan(r.Context(), span)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// setCORSMiddleware adds CORS headers.
func (s *Service) setCORSMiddleware(next http.Handler) http.Handler {
	if s.corsOptions.IsNone() {
		return next
	}

	return cors.New(s.corsOptions.Unwrap()).Handler(next)
}
