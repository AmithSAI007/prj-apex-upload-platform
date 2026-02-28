package config

import (
	"context"
	"errors"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(cfg *Config, ctx context.Context) (func(context.Context) error, error) {

	var shutdownFuncs []func(context.Context) error
	var err error

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	serviceName := cfg.OTEL_SERVICE_NAME

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C Trace Context for distributed tracing
		propagation.Baggage{},      // OpenTelemetry Baggage for custom key-value pairs
	)

	otel.SetTextMapPropagator(prop)

	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}

	// Create OTLP trace exporter to send spans to collector
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)))

	if err != nil {
		handleErr(err)
		return shutdown, err
	}

	resources, err := resource.New(
		context.Background(),
		resource.WithTelemetrySDK(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),        // logical service name
			attribute.String("telemetry.sdk.language", "go"),     // programming language
			attribute.String("gcp.project.id", cfg.GCPProjectID), // GCP project ID
		),
	)

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()), // Sample all traces for demonstration; adjust in production
		trace.WithBatcher(exporter), trace.WithResource(resources))

	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	// Return shutdown function to flush remaining traces on exit
	return shutdown, err
}
