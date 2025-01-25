package grpcdial

import (
	"context"

	"google.golang.org/grpc"
)

func (d *Dialer) clientInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		d.logger.Debug(ctx,
			"grpc client error",
			"grpc_client", "unary",
			"grpc_method", method,
			"grpc_target", cc.Target(),
			"error", err)
	}

	return err
}

func (d *Dialer) clientStreamInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	stream, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		d.logger.Debug(ctx,
			"grpc client error",
			"grpc_client", "unary",
			"grpc_method", method,
			"grpc_target", cc.Target(),
			"error", err)
	}

	return stream, err
}
