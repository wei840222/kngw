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
	"go.opentelemetry.io/otel/trace"
)

func RegisterHandler(rc *resty.Client, e *gin.Engine) {
	h := &handler{rc}
	e.Any("/:ns/:ksvc/*path", h.syncInvoke)
	e.Any("/async/:ns/:ksvc/*path", h.asyncInvoke)
}

type handler struct {
	rc *resty.Client
}

func (h handler) syncInvoke(c *gin.Context) {
	res, err := h.rc.R().
		SetContext(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c))).
		SetHeaderMultiValues(c.Request.Header).
		SetBody(c.Request.Body).
		Execute(c.Request.Method, fmt.Sprintf("http://%s.%s.svc.cluster.local%s", c.Param("ksvc"), c.Param("ns"), c.Param("path")))
	if err != nil {
		panic(err)
	}
	c.Data(res.StatusCode(), res.Header().Get("Content-Type"), res.Body())
}

func (h handler) asyncInvoke(c *gin.Context) {
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
	}(trace.ContextWithSpan(context.Background(), trace.SpanFromContext(c)))

	c.Status(http.StatusAccepted)
}
