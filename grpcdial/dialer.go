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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/n-r-w/bootstrap"
	"github.com/n-r-w/ctxlog"
)

// Dialer - manages connections to gRPC server. Implements IService interface.
type Dialer struct {
	logger      ctxlog.ILogger
	connections map[string]*grpc.ClientConn
	opts        []Option
}

// New creates a new Dialer.
func New(ctx context.Context, logger ctxlog.ILogger, opts ...Option) *Dialer {
	d := &Dialer{
		connections: make(map[string]*grpc.ClientConn),
		opts:        opts,
	}

	return d
}

// WithCredentials sets TLS configuration for connecting to gRPC server.
// If not set, insecure.NewCredentials() is used.
func WithCredentials(creds credentials.TransportCredentials) Option {
	return func(g *targetInfo) {
		g.creds = creds
	}
}

// WithUnaryInterceptors sets list of UnaryClientInterceptor for gRPC client. Optional.
func WithUnaryInterceptors(interceptors ...grpc.UnaryClientInterceptor) Option {
	return func(g *targetInfo) {
		g.unaryInterceptors = interceptors
	}
}

// WithStreamInterceptors sets list of StreamClientInterceptor for gRPC client. Optional.
func WithStreamInterceptors(interceptors ...grpc.StreamClientInterceptor) Option {
	return func(g *targetInfo) {
		g.streamInterceptors = interceptors
	}
}

// WithRetryOptions sets list of CallOption for gRPC client.
// Use either WithRetryOptions or WithClientDefaultRetryOptions.
// If neither is set, default settings are used: 3 retries, 1 second between retries.
func WithRetryOptions(opts ...grpc_retry.CallOption) Option {
	return func(g *targetInfo) {
		g.retryOpts = opts
	}
}

// WithDefaultRetryOptions sets default retry parameters for gRPC client.
// Use either WithClientRetryOptions or WithDefaultRetryOptions.
// If neither is set, default settings are used:
// maxRetries: 3 retries
// requestTimeout: maximum 10 seconds per request
// retryTimeout: 1 second between retries.
func WithDefaultRetryOptions(maxRetries int, requestTimeout, retryTimeout time.Duration) Option {
	return func(g *targetInfo) {
		g.retryOpts = nil

		g.maxRetries = maxRetries
		g.requestTimeout = requestTimeout
		g.retryTimeout = retryTimeout
	}
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
				d.logger.Warn(ctx, "grpc client retry",
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
			d.clientInterceptor,
			grpc_retry.UnaryClientInterceptor(t.retryOpts...),
		}
	}

	if t.streamInterceptors == nil {
		t.streamInterceptors = []grpc.StreamClientInterceptor{
			d.clientStreamInterceptor,
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
func (d *Dialer) Info() bootstrap.Info {
	return bootstrap.Info{
		Name: "grpc dialer",
	}
}

// Start does nothing since gRPC server connection is established in Dial.
func (d *Dialer) Start(_ context.Context) error {
	return nil
}

// Stop disconnects from gRPC servers.
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

type targetInfo struct {
	creds              credentials.TransportCredentials
	unaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor

	retryOpts      []grpc_retry.CallOption
	maxRetries     int
	requestTimeout time.Duration // request timeout
	retryTimeout   time.Duration // retry timeout
}

// Option - function for configuring targetInfo.
type Option func(*targetInfo)
