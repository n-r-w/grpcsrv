package grpcdial

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

type statHandlerWrapper struct {
	h stats.Handler
}

// TagRPC can attach some information to the given context.
// The context used for the rest lifetime of the RPC will be derived from
// the returned context.
func (w *statHandlerWrapper) TagRPC(ctx context.Context, s *stats.RPCTagInfo) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		md = md.Copy()
		md.Set("trace-id", traceID)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return w.h.TagRPC(ctx, s)
}

// HandleRPC processes the RPC stats.
func (w *statHandlerWrapper) HandleRPC(ctx context.Context, s stats.RPCStats) {
	w.h.HandleRPC(ctx, s)
}

// TagConn can attach some information to the given context.
// The returned context will be used for stats handling.
// For conn stats handling, the context used in HandleConn for this
// connection will be derived from the context returned.
// For RPC stats handling,
//   - On server side, the context used in HandleRPC for all RPCs on this
//
// connection will be derived from the context returned.
//   - On client side, the context is not derived from the context returned.
func (w *statHandlerWrapper) TagConn(ctx context.Context, s *stats.ConnTagInfo) context.Context {
	return w.h.TagConn(ctx, s)
}

// HandleConn processes the Conn stats.
func (w *statHandlerWrapper) HandleConn(ctx context.Context, s stats.ConnStats) {
	w.h.HandleConn(ctx, s)
}
