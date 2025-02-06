package grpcsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	http_pprof "net/http/pprof"
	"runtime/pprof"

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

// getPProfHandler returns an http.Handler for serving pprof endpoints.
func getPProfHandler() http.Handler {
	debugMux := http.NewServeMux()
	debugMux.Handle("/debug/pprof/", http.HandlerFunc(http_pprof.Index))
	debugMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(http_pprof.Cmdline))
	debugMux.Handle("/debug/pprof/profile", http.HandlerFunc(http_pprof.Profile))
	debugMux.Handle("/debug/pprof/symbol", http.HandlerFunc(http_pprof.Symbol))
	debugMux.Handle("/debug/pprof/trace", http.HandlerFunc(http_pprof.Trace))
	return debugMux
}

// startPProfServer starts a dedicated HTTP server for pprof endpoints.
func (s *Service) startPProfServer(ctx context.Context) error {
	if s.pprofEndpoint == "" {
		return nil
	}

	s.pprofServer = &http.Server{
		Addr:              s.pprofEndpoint,
		Handler:           getPProfHandler(),
		ReadHeaderTimeout: s.httpReadHeaderTimeout,
	}

	listener, err := net.Listen("tcp", s.pprofEndpoint)
	if err != nil {
		return fmt.Errorf("failed to start pprof server listener: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.logger.Info(ctx, "starting pprof server", "addr", s.pprofEndpoint)
		if err := s.pprofServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error(ctx, "pprof server error", "error", err)
		}
	}()

	return nil
}
