package main

import (
	"net/http"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func InitResty() *resty.Client {
	return resty.
		NewWithClient(&http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}).
		SetDebug(true).
		SetDebugBodyLimit(1024)
}
