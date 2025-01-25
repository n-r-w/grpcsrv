// Package grpcsrv provides functionality for running a gRPC server and its HTTP gateway.
package grpcsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/moznion/go-optional"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/ctxlog"
)

// Service service for working with gRPC and HTTP servers.
type Service struct {
	name                  string
	logger                ctxlog.ILogger
	httpReadHeaderTimeout time.Duration
	grpcInitializers      []IGRPCInitializer
	grpcOptions           []grpc.ServerOption
	endpoint              Endpoint
	healthCheckHandler    IHealther
	// list of keys whose values will be replaced with "sanitized" in logs.
	sanitizeKeys []string

	recoverEnabled bool
	pprofEnabled   bool

	httpFileSupport         bool
	httpDialOptions         []grpc.DialOption
	httpMarshallers         map[string]grpc_runtime.Marshaler // content-type -> marshaler
	httpHeadersFromMetadata []string
	corsOptions             optional.Option[cors.Options]

	wg         sync.WaitGroup
	httpServer *http.Server

	// used for serving prometheus metrics (if enabled)
	httpMetricsPort   string
	httpMetricsServer *http.Server

	// function for panic logging (logging only, not recovery)
	panicLogger func(ctx context.Context, p any)
	// function for enriching context. Called before request processing.
	ctxUnaryModifier  CtxUnaryModifier
	ctxStreamModifier CtxStreamModifier
	ctxHTTPModifier   CtxHTTPModifier
	// Function for registering health check endpoints.
	registerHealthCheckEndpoints RegisterHealthCheckEndpoints

	grpcGatewayConn *grpc.ClientConn
	grpcServer      *grpc.Server
}

var _ bootstrap.IService = (*Service)(nil)

// New creates a new service instance.
func New(ctx context.Context, grpcSevices []IGRPCInitializer, opt ...Option) *Service {
	s := &Service{
		name:             "grpc",
		grpcInitializers: grpcSevices,
		endpoint: Endpoint{
			GRPC: ":50051",
			HTTP: ":50052",
		},
		httpMetricsPort: ":50053",
	}

	for _, o := range opt {
		o(s)
	}

	if s.logger == nil {
		s.logger = ctxlog.NewStubWrapper()
	}

	if s.ctxUnaryModifier == nil {
		s.ctxUnaryModifier = func(
			ctx context.Context, _ any, _ *grpc.UnaryServerInfo, _ grpc.UnaryHandler, _ string,
		) context.Context {
			return ctx
		}
	}

	if s.ctxStreamModifier == nil {
		s.ctxStreamModifier = func(
			ctx context.Context, _ *grpc.StreamServerInfo, _ grpc.StreamHandler, _ string,
		) context.Context {
			return ctx
		}
	}

	if s.ctxHTTPModifier == nil {
		s.ctxHTTPModifier = func(ctx context.Context, _ *http.Request) context.Context {
			return ctx
		}
	}

	if s.registerHealthCheckEndpoints == nil {
		s.registerHealthCheckEndpoints = func(ctx context.Context, _ *grpc_runtime.ServeMux) error {
			return nil
		}
	}

	if len(s.sanitizeKeys) == 0 {
		s.sanitizeKeys = []string{"password", "token", "refreshToken", "accessToken"}
	}

	return s
}

// Info returns information about the service.
func (s *Service) Info() bootstrap.Info {
	return bootstrap.Info{
		Name:          s.name,
		RestartPolicy: nil, // startup does not depend on external factors
	}
}

// Start starts the service.
func (s *Service) Start(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx) // ignore startup timeout since context will go to goroutine

	httpRequired := s.prepare(ctx)

	if err := s.startGRPCServer(ctx); err != nil {
		return err
	}

	// start HTTP gateway
	if httpRequired {
		if err := s.startHTTPGateway(ctx); err != nil {
			return err
		}
	}

	if !httpRequired {
		s.logger.Info(ctx, "HTTP server is disabled")
	}

	return nil
}

// Stop stops the service. Stop timeout is set through context.
func (s *Service) Stop(ctx context.Context) error {
	var wg sync.WaitGroup

	if s.httpServer != nil {
		wg.Add(1)

		go func() {
			defer wg.Done()

			s.logger.Info(ctx, "gracefully stopping http")
			err := s.httpServer.Shutdown(ctx)
			if err != nil {
				s.logger.Error(ctx, "failed to stop http server", "error", err)
			}
			s.logger.Info(ctx, "http stopped gracefully")
			err = s.grpcGatewayConn.Close()
			if err != nil {
				s.logger.Error(ctx, "failed to close grpc gateway connection", "error", err)
			}
		}()
	}

	if s.httpMetricsServer != nil {
		wg.Add(1)

		go func() {
			defer wg.Done()

			s.logger.Info(ctx, "gracefully stopping metrics server")
			err := s.httpMetricsServer.Shutdown(ctx)
			if err != nil {
				s.logger.Error(ctx, "failed to stop metrics server", "error", err)
			}
			s.logger.Info(ctx, "metrics server stopped gracefully")
		}()
	}

	wg.Wait()

	s.logger.Info(ctx, "gracefully stopping grpc")
	s.grpcServer.GracefulStop()
	s.logger.Info(ctx, "grpc stopped gracefully")

	s.wg.Wait()

	return nil
}

func (s *Service) prepare(_ context.Context) (httpRequired bool) {
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		s.callServerInterceptor,
		pprofUnaryInterceptor,
		s.tracingDataServerInterceptor,
	}

	if s.recoverEnabled {
		unaryInterceptors = append(unaryInterceptors, s.recoverUnaryGRPC)
	}

	streamInterceptors := []grpc.StreamServerInterceptor{
		s.callServerStreamInterceptor,
		pprofStreamInterceptor,
	}
	if s.recoverEnabled {
		streamInterceptors = append(streamInterceptors, s.recoverStreamGRPC)
	}

	grpcOptions := s.grpcOptions
	grpcOptions = append(grpcOptions, grpc.StatsHandler(otelgrpc.NewServerHandler()))

	for _, i := range s.grpcInitializers {
		opt := i.GetOptions()

		unaryInterceptors = append(unaryInterceptors, opt.GRPCUnaryInterceptors...)
		streamInterceptors = append(streamInterceptors, opt.GRPCStreamInterceptors...)
		grpcOptions = append(grpcOptions, opt.GRPCOptions...)
	}

	grpcOptions = append(grpcOptions,
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
	)

	s.grpcServer = grpc.NewServer(grpcOptions...)

	reflection.Register(s.grpcServer)

	for _, i := range s.grpcInitializers {
		i.RegisterGRPCServer(s.grpcServer)
	}

	return s.endpoint.HTTP != ""
}

func (s *Service) startGRPCServer(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.endpoint.GRPC)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if errServe := s.grpcServer.Serve(listener); errServe != nil {
			panic(s.name + ". failed to serve gRPC server: " + errServe.Error())
		}
	}()

	if s.endpoint.HTTP != "" {
		s.logger.Info(ctx, "listening", "grpc", s.endpoint.GRPC, "http", s.endpoint.HTTP)
	} else {
		s.logger.Info(ctx, "listening", "grpc", s.endpoint.GRPC)
	}

	return nil
}
