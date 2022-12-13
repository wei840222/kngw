package main

import (
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	ce_client "github.com/cloudevents/sdk-go/v2/client"
	ce_http "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func InitHTTPClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}

func InitResty(hc *http.Client) *resty.Client {
	return resty.NewWithClient(hc)
}

func InitCloudEventsClient(hc *http.Client) (ce_client.Client, error) {
	return cloudevents.NewClientHTTP(ce_http.WithClient(*hc))
}
