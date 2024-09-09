package telemetry

import (
	"context"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type otelCtxKeyType string

const otelCtxKey otelCtxKeyType = "github.com/jylitalo/pkg/otel"

func GetTracer(ctx context.Context) trace.Tracer {
	return ctx.Value(otelCtxKey).(trace.Tracer)
}

func newExporter(fname string) (sdktrace.SpanExporter, error) {
	// Your preferred exporter: console, jaeger, zipkin, OTLP, etc.
	opts := []stdouttrace.Option{}
	if fname != "" {
		f, err := os.Create(fname)
		if err != nil {
			return nil, err
		}
		opts = append(opts, stdouttrace.WithWriter(f))
	}
	return stdouttrace.New(opts...)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("ExampleService"),
		),
	)

	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

func SetupConsole(ctx context.Context, name string) (context.Context, *sdktrace.TracerProvider, error) {
	exp, err := newExporter("." + filepath.Base(name) + ".telemetry")
	if err != nil {
		return ctx, nil, err
	}
	tp := newTraceProvider(exp)
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer(name)
	ctx = context.WithValue(ctx, otelCtxKey, tracer)
	return ctx, tp, err
}
