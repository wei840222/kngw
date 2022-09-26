package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

func InitTracerProvider(lc fx.Lifecycle) (trace.TracerProvider, error) {
	var tp *sdktrace.TracerProvider

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
				otlptracegrpc.WithInsecure(),
				otlptracegrpc.WithEndpoint("opentelemetry-collector.observability.svc.cluster.local:4317"),
			))
			if err != nil {
				return err
			}

			tp = sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(resource.NewWithAttributes(
					semconv.SchemaURL,
					semconv.ServiceNameKey.String("knative-gateway"),
					semconv.ServiceVersionKey.String("0.0.1"),
				)),
			)

			otel.SetTracerProvider(tp)
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	})

	return otel.GetTracerProvider(), nil
}
