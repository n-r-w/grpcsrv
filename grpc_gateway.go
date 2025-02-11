package grpcsrv

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// propagateTraceContext propagate trace from grpc-gateway to grpc. Without this magic, it doesn't work.
func propagateTraceContext(ctx context.Context, _ *http.Request) metadata.MD {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return metadata.New(carrier)
}

func (s *Service) startHTTPGateway(ctx context.Context) error {
	muxOptList := []runtime.ServeMuxOption{
		runtime.WithMetadata(propagateTraceContext),
	}

	if len(s.httpHeadersFromMetadata) > 0 {
		muxOptList = append(muxOptList, runtime.WithForwardResponseOption(s.responseHTTPHeaderMatcher))
	}

	// Whether to use default JSON marshaller
	jsonMarshallers, err := s.getJSONMarshallers()
	if err != nil {
		return err
	}
	muxOptList = append(muxOptList, jsonMarshallers...)

	var dialOpts []grpc.DialOption

	// telemetry
	dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))

	if len(s.httpDialOptions) > 0 {
		dialOpts = append(dialOpts, s.httpDialOptions...)
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Create gRPC client for gRPC gateway
	conn, err := grpc.NewClient(s.endpoint.GRPC, dialOpts...)
	if err != nil {
		return fmt.Errorf("grpc gateway: failed to create grpc client: %w", err)
	}
	s.grpcGatewayConn = conn

	// Create gRPC multiplexer for gRPC gateway
	mux := runtime.NewServeMux(muxOptList...)

	// register handlers for gRPC gateway
	for _, i := range s.grpcInitializers {
		if i.GetOptions().HTTPHandlerRequired {
			if err = i.RegisterHTTPHandler(ctx, mux, conn); err != nil {
				return fmt.Errorf("%s. failed to register gRPC gateway: %w", s.name, err)
			}
		}
	}

	var targetHandlers http.Handler = mux

	// Panic recovery support
	if s.recoverEnabled {
		targetHandlers = s.recoverHTTP(targetHandlers)
	}

	// Support for logging, tracing and metrics
	targetHandlers = s.setTraceRouteHTTPMiddleware(targetHandlers)
	targetHandlers = s.setCtxModifierHTTPMiddleware(targetHandlers)
	targetHandlers = s.setCORSMiddleware(targetHandlers)

	// Health check support
	if err = s.registerHealthCheckEndpoints(ctx, mux); err != nil {
		return err
	}

	// Register additional HTTP endpoints
	if err = s.registerHTTPEndpoints(ctx, mux); err != nil {
		return err
	}

	// add tracing support to grpc-gateway
	grpcgw := otelhttp.NewMiddleware("grpc-gateway", otelhttp.WithFilter(
		func(r *http.Request) bool {
			// ignore requests from prometheus otherwise they spam
			return r.URL.Path != "/metrics"
		},
	))

	// Start HTTP server
	s.httpServer = &http.Server{
		Addr:              s.endpoint.HTTP,
		Handler:           grpcgw(targetHandlers),
		ReadHeaderTimeout: s.httpReadHeaderTimeout,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if errListener := s.httpServer.ListenAndServe(); errListener != nil && errListener != http.ErrServerClosed {
			panic(s.name + ". failed to listen and serve HTTP server: " + errListener.Error())
		}
	}()

	return nil
}

// get marshallers for gRPC gateway.
func (s *Service) getJSONMarshallers() ([]runtime.ServeMuxOption, error) { //nolint:unparam // ok
	var marshallers []runtime.ServeMuxOption

	needDefaultJSONMarshaller := true
	const (
		jsonContentType = "application/json"
	)
	if len(s.httpMarshallers) > 0 {
		for contentType, marshaler := range s.httpMarshallers {
			marshallers = append(marshallers, runtime.WithMarshalerOption(contentType, marshaler))
		}

		if _, ok := s.httpMarshallers[jsonContentType]; ok {
			needDefaultJSONMarshaller = false
		}
	}

	if needDefaultJSONMarshaller {
		marshallers = append(marshallers,
			runtime.WithMarshalerOption(jsonContentType,
				&runtime.JSONPb{
					MarshalOptions: protojson.MarshalOptions{
						UseEnumNumbers:    false,
						AllowPartial:      false,
						EmitUnpopulated:   true,
						EmitDefaultValues: false,
					},
					UnmarshalOptions: protojson.UnmarshalOptions{
						DiscardUnknown: false,
						AllowPartial:   false,
					},
				},
			),
		)
	}

	return marshallers, nil
}

// support for headers from metadata in response.
func (s *Service) responseHTTPHeaderMatcher(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	for _, header := range s.httpHeadersFromMetadata {
		if vals := md.TrailerMD.Get(strings.ToLower(header)); len(vals) > 0 {
			w.Header().Set(header, vals[0])
		}
	}

	return nil
}
