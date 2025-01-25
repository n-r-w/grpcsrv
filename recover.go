package grpcsrv

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func errFromPanic(p any) error {
	var errText string
	switch e := p.(type) {
	case error:
		errText = e.Error()
	case string:
		errText = e
	default:
		errText = fmt.Sprintf("%#v", e)
	}

	return status.Errorf(codes.Internal, "recover: %s", errText)
}

func (s *Service) logPanic(ctx context.Context, p any) {
	if s.panicLogger != nil {
		s.panicLogger(ctx, p)
	}
}

// gRPC interceptor for panic recovery.
func (s *Service) recoverUnaryGRPC(ctx context.Context, req any, _ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (_ any, err error) {
	defer func() {
		if p := recover(); p != nil {
			traceID, traceOK := s.traceIDFromContext(ctx)

			attrs := make([]any, 0, 2) //nolint:mnd // ok
			attrs = append(attrs, "panic", p)
			if traceOK {
				attrs = append(attrs, "trace_id", traceID)
			}
			attrs = append(attrs, "stack_trace", string(debug.Stack()))

			s.logger.Error(ctx, "recovered from grpc panic", attrs...)

			err = errFromPanic(p)
			s.logPanic(ctx, p)
		}
	}()
	return handler(ctx, req)
}

// gRPC interceptor for panic recovery.
func (s *Service) recoverStreamGRPC(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	defer func() {
		if p := recover(); p != nil {
			traceID, traceOK := s.traceIDFromContext(ss.Context())

			attrs := make([]any, 0, 2) //nolint:mnd // ok
			attrs = append(attrs, "panic", p)
			if traceOK {
				attrs = append(attrs, "trace_id", traceID)
			}
			attrs = append(attrs, "stack_trace", string(debug.Stack()))
			s.logger.Error(ss.Context(), "recovered from grpc panic", attrs...)

			err = errFromPanic(p)
			s.logPanic(ss.Context(), p)
		}
	}()
	return handler(srv, ss)
}

// recovers from panic in http.Handler.
func (s *Service) recoverHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				traceID, traceOK := s.traceIDFromContext(r.Context())

				attrs := make([]any, 0, 2) //nolint:mnd // ok
				attrs = append(attrs, "panic", p)
				if traceOK {
					attrs = append(attrs, "trace_id", traceID)
				}
				attrs = append(attrs, "stack_trace", string(debug.Stack()))
				s.logger.Error(r.Context(), "recovered from http panic", attrs...)

				err := errFromPanic(p)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				s.logPanic(r.Context(), p)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
