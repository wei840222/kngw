package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/trace"
)

func RegisterHandler(rc *resty.Client, mp metric.MeterProvider, e *gin.Engine) error {
	meter := mp.Meter("github.com/wei840222/kngw.handler")
	syncInvokeCounter, err := meter.SyncInt64().Counter("sync_invoke")
	if err != nil {
		return err
	}
	asyncInvokeCounter, err := meter.SyncInt64().Counter("async_invoke")
	if err != nil {
		return err
	}

	h := &handler{rc, syncInvokeCounter, asyncInvokeCounter}
	e.Any("/:ns/:ksvc/*path", h.syncInvoke)
	e.Any("/async/:ns/:ksvc/*path", h.asyncInvoke)
	return nil
}

type handler struct {
	rc                 *resty.Client
	syncInvokeCounter  syncint64.Counter
	asyncInvokeCounter syncint64.Counter
}

func (h handler) syncInvoke(c *gin.Context) {
	h.syncInvokeCounter.Add(c, 1)
	res, err := h.rc.R().
		SetContext(httptrace.WithClientTrace(c.Request.Context(), otelhttptrace.NewClientTrace(c.Request.Context()))).
		SetHeaderMultiValues(c.Request.Header).
		SetBody(c.Request.Body).
		Execute(c.Request.Method, fmt.Sprintf("http://%s.%s.svc.cluster.local%s", c.Param("ksvc"), c.Param("ns"), c.Param("path")))
	if err != nil {
		panic(err)
	}
	c.Data(res.StatusCode(), res.Header().Get("Content-Type"), res.Body())
}

func (h handler) asyncInvoke(c *gin.Context) {
	h.asyncInvokeCounter.Add(c, 1)
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		panic(err)
	}

	cc := c.Copy()

	go func(ctx context.Context) {
		res, err := h.rc.R().
			SetContext(httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))).
			SetHeaderMultiValues(cc.Request.Header).
			SetBody(string(b)).
			Execute(cc.Request.Method, fmt.Sprintf("http://%s.%s.svc.cluster.local%s", cc.Param("ksvc"), cc.Param("ns"), cc.Param("path")))
		if err != nil {
			log.Printf("Do async request error: %s", err)
			return
		}
		log.Printf("Do async request: %s", res.Status())
	}(trace.ContextWithSpan(context.Background(), trace.SpanFromContext(cc.Request.Context())))

	c.Status(http.StatusAccepted)
}
