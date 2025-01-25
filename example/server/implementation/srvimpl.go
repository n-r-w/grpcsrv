package srvimpl

import (
	"context"
	"fmt"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/n-r-w/grpcsrv"
	api "github.com/n-r-w/grpcsrv/example/protogen"
	"google.golang.org/grpc"
)

// GreeterService implements the Greeter service
type GreeterService struct {
	api.UnimplementedGreeterServer
}

// SayHello implements the SayHello RPC method
func (s *GreeterService) SayHello(ctx context.Context, req *api.HelloRequest) (*api.HelloResponse, error) {
	return &api.HelloResponse{
		Message:   fmt.Sprintf("Hello, %s!", req.Name),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// SayManyHellos implements the streaming RPC method
func (s *GreeterService) SayManyHellos(req *api.HelloRequest, stream api.Greeter_SayManyHellosServer) error {
	for i := 0; i < 5; i++ {
		if err := stream.Send(&api.HelloResponse{
			Message:   fmt.Sprintf("Hello %d times, %s!", i+1, req.Name),
			Timestamp: time.Now().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	return nil
}

// GreeterInitializer implements the gRPC initializer interface
type GreeterInitializer struct {
	service *GreeterService
}

// NewGreeterInitializer creates a new GreeterInitializer
func NewGreeterInitializer(service *GreeterService) *GreeterInitializer {
	return &GreeterInitializer{
		service: service,
	}
}

func (i *GreeterInitializer) RegisterGRPCServer(s *grpc.Server) {
	api.RegisterGreeterServer(s, i.service)
}

func (i *GreeterInitializer) RegisterHTTPHandler(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return nil // HTTP gateway not required for this example
}

func (i *GreeterInitializer) GetOptions() grpcsrv.InitializeOptions {
	return grpcsrv.InitializeOptions{
		HTTPHandlerRequired: false,
	}
}
