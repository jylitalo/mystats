package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type otelCtxKeyType string

const otelCtxKey otelCtxKeyType = "github.com/jylitalo/pkg/otel"

func newConsoleExporter(fname string) (sdktrace.SpanExporter, error) {
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

func newOtelExporter(address string) (sdktrace.SpanExporter, error) {
	return otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(address),
			otlptracehttp.WithHeaders(map[string]string{"content-type": "application/json"}),
			otlptracehttp.WithInsecure(),
		),
	)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("mystats"),
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

func Setup(ctx context.Context, name string) (context.Context, *sdktrace.TracerProvider, error) {
	var newExp func(string) (sdktrace.SpanExporter, error)

	if strings.Contains(name, ":") {
		newExp = newOtelExporter
	} else {
		newExp = newConsoleExporter
		name = "." + filepath.Base(name) + ".telemetry"
	}
	exp, err := newExp(name)
	if err != nil {
		return ctx, nil, err
	}
	tp := newTraceProvider(exp)
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer(name)
	ctx = context.WithValue(ctx, otelCtxKey, tracer)
	return ctx, tp, err
}

func NewSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return ctx.Value(otelCtxKey).(trace.Tracer).Start(ctx, name)
}

func Error(span trace.Span, err error) error {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
