package grpcsrv

import (
	"context"
	"net/http"
	http_pprof "net/http/pprof"
	"runtime/pprof"
	"strings"

	"google.golang.org/grpc"
)

// pprofUnaryInterceptor interceptor for adding pprof label of the called handler.
func pprofUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	var (
		resp any
		err  error
	)

	pprof.Do(ctx, pprof.Labels("grpc-handler", info.FullMethod), func(ctx context.Context) {
		resp, err = handler(ctx, req)
	})
	return resp, err
}

// pprofStreamInterceptor interceptor for adding pprof label of the called handler.
func pprofStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	var err error
	pprof.Do(ss.Context(), pprof.Labels("grpc-handler", info.FullMethod), func(ctx context.Context) {
		err = handler(srv, newStreamWithContext(ctx, ss))
	})
	return err
}

type streamWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (s streamWrapper) Context() context.Context {
	return s.ctx
}

func newStreamWithContext(ctx context.Context, stream grpc.ServerStream) grpc.ServerStream {
	return streamWrapper{
		ctx:          ctx,
		ServerStream: stream,
	}
}

func pprofHandler(next http.Handler) http.Handler {
	debugMux := http.NewServeMux()
	debugMux.Handle("/debug/pprof/", http.HandlerFunc(http_pprof.Index))
	debugMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(http_pprof.Cmdline))
	debugMux.Handle("/debug/pprof/profile", http.HandlerFunc(http_pprof.Profile))
	debugMux.Handle("/debug/pprof/symbol", http.HandlerFunc(http_pprof.Symbol))
	debugMux.Handle("/debug/pprof/trace", http.HandlerFunc(http_pprof.Trace))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
			debugMux.ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
