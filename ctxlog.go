package grpcsrv

import (
	"context"
	"errors"
	"net/http"

	"github.com/n-r-w/ctxlog"
	"google.golang.org/grpc"
)

// GetCtxLogOptions returns options for using ctxlog with grpcsrv.
// context must contain ctxlog.Logger.
func GetCtxLogOptions(ctx context.Context) ([]Option, error) {
	if !ctxlog.InContext(ctx) {
		return nil, errors.New("context does not contain ctxlog.Logger")
	}

	opts := []Option{
		WithLogger(ctxlog.NewWrapper()),
	}

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

	opts = append(opts, WithContextModifiers(unaryRequestModifier, streamRequestModifier, httpRequestModifier))

	return opts, nil
}
