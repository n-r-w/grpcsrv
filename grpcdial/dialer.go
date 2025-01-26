package grpcdial

import (
	"context"
	"errors"
	"fmt"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/ctxlog"
)

// Dialer - manages connections to gRPC server. Implements IService interface.
type Dialer struct {
	connections map[string]*grpc.ClientConn
	opts        []Option
}

// New creates a new Dialer.
func New(ctx context.Context, opts ...Option) *Dialer {
	d := &Dialer{
		connections: make(map[string]*grpc.ClientConn),
		opts:        opts,
	}

	return d
}

// Dial connects to gRPC server.
func (d *Dialer) Dial(ctx context.Context, target, name string, opts ...Option) (*grpc.ClientConn, error) {
	return d.dialHelper(ctx, target, name, true, opts...)
}

// DialNoClose connects to gRPC server without saving connection (connection is not closed on shutdown).
// Used for one-time connections.
func (d *Dialer) DialNoClose(ctx context.Context, target, name string, opts ...Option) (*grpc.ClientConn, error) {
	return d.dialHelper(ctx, target, name, false, opts...)
}

// Dial connects to gRPC server.
func (d *Dialer) dialHelper(
	_ context.Context,
	target, name string,
	saveCon bool,
	opts ...Option,
) (*grpc.ClientConn, error) {
	if saveCon {
		if _, ok := d.connections[target]; ok {
			panic("already connected to " + target)
		}
	}

	const (
		defaultMaxRetries     = 3
		defaultRequestTimeout = time.Second * 10
		defaultRetryTimeout   = time.Second
	)

	t := &targetInfo{
		maxRetries:     defaultMaxRetries,
		requestTimeout: defaultRequestTimeout,
		retryTimeout:   defaultRetryTimeout,
		logger:         ctxlog.NewStubWrapper(),
	}

	for _, opt := range d.opts {
		opt(t)
	}

	for _, opt := range opts {
		opt(t)
	}

	if t.creds == nil {
		t.creds = insecure.NewCredentials()
	}

	if t.retryOpts == nil {
		t.retryOpts = []grpc_retry.CallOption{
			grpc_retry.WithMax(uint(t.maxRetries)), //nolint:gosec // ok
			grpc_retry.WithCodes(append(grpc_retry.DefaultRetriableCodes, codes.Unknown, codes.Internal)...),
			grpc_retry.WithPerRetryTimeout(t.requestTimeout),
			grpc_retry.WithBackoffContext(func(ctx context.Context, attempt uint) time.Duration {
				t.logger.Warn(ctx, "grpc client retry",
					"target", name,
					"attempt", attempt)
				return t.retryTimeout
			}),
		}
	}

	statWrapper := &statHandlerWrapper{
		h: otelgrpc.NewClientHandler(
			otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents, otelgrpc.SentEvents),
		),
	}

	if t.unaryInterceptors == nil {
		t.unaryInterceptors = []grpc.UnaryClientInterceptor{
			d.getClientInterceptor(t.logger),
			grpc_retry.UnaryClientInterceptor(t.retryOpts...),
		}
	}

	if t.streamInterceptors == nil {
		t.streamInterceptors = []grpc.StreamClientInterceptor{
			d.getStreamClientInterceptor(t.logger),
			grpc_retry.StreamClientInterceptor(t.retryOpts...),
		}
	}

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(t.creds),
		grpc.WithStatsHandler(statWrapper),
		grpc.WithChainUnaryInterceptor(t.unaryInterceptors...),
		grpc.WithChainStreamInterceptor(t.streamInterceptors...))
	if err != nil {
		return nil, fmt.Errorf("grpc dial target %s, name %s: %w", target, name, err)
	}

	if saveCon {
		d.connections[target] = conn
	}

	return conn, nil
}

// Info returns information.
// Implements bootstrap.IService interface.
func (d *Dialer) Info() bootstrap.Info {
	return bootstrap.Info{
		Name: "grpc dialer",
	}
}

// Start does nothing since gRPC server connection is established in Dial.
// Implements bootstrap.IService interface.
func (d *Dialer) Start(_ context.Context) error {
	return nil
}

// Stop disconnects from gRPC servers.
// Implements bootstrap.IService interface.
func (d *Dialer) Stop(_ context.Context) error {
	var err error
	for _, conn := range d.connections {
		if conn != nil {
			if e := conn.Close(); e != nil {
				err = errors.Join(err, e)
			}
		}
	}

	return err
}
