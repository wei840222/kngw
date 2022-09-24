package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

func initTracerProvider(lc fx.Lifecycle) (trace.TracerProvider, error) {
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

func initHTTPClient() *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
}

func registerRouter(lc fx.Lifecycle, tp trace.TracerProvider, e *gin.Engine, hc *http.Client) {
	e.ContextWithFallback = true
	e.Use(otelgin.Middleware("gin", otelgin.WithTracerProvider(tp)))

	e.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	e.Any("/:ns/:ksvc/*path", func(c *gin.Context) {
		ksvcURL, _ := url.Parse(fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Param("ksvc"), c.Param("ns")))
		ksvcURL.Path = c.Param("path")

		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, ksvcURL.String(), bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		req.Host = ksvcURL.Host
		req.Header = c.Request.Header

		res, err := hc.Do(req)

		if err != nil {
			panic(err)
		}

		b, err = io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		c.Data(res.StatusCode, res.Header.Get("Content-Type"), b)
	})

	e.Any("/async/:ns/:ksvc/*path", func(c *gin.Context) {
		ksvcURL, _ := url.Parse(fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Param("ksvc"), c.Param("ns")))
		ksvcURL.Path = c.Param("path")

		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}

		ctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(c.Request.Context()))
		go func() {
			req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx)), c.Request.Method, ksvcURL.String(), bytes.NewReader(b))
			if err != nil {
				panic(err)
			}
			req.Host = ksvcURL.Host
			req.Header = c.Request.Header

			res, err := hc.Do(req)
			if err != nil {
				log.Printf("Do async request error: %s", err)
				return
			}
			log.Printf("Do async request status: %d", res.StatusCode)
		}()

		c.Status(http.StatusAccepted)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: e,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatal(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}

func main() {
	fx.New(
		fx.Provide(initTracerProvider),
		fx.Provide(initHTTPClient),
		fx.Provide(gin.Default),
		fx.Invoke(registerRouter),
	).Run()
}
