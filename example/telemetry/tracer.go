package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// InitTracer initializes OpenTelemetry tracer.
func InitTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	otlpGrpcOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure(),
	}

	exporter, err := otlptracegrpc.New(ctx, otlpGrpcOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp exporter: %w", err)
	}

	attrs := []attribute.KeyValue{semconv.ServiceNameKey.String(serviceName)}

	rc := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(rc),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	// Return a cleanup function
	cleanup := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	// Set the global propagator
	propagators := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
		jaeger.Jaeger{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader | b3.B3SingleHeader)),
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagators...))

	return cleanup, nil
}
