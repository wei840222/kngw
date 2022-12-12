package main

import (
	"fmt"
	"net/http/httptrace"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

func RegisterHandler(rc *resty.Client, mp metric.MeterProvider, e *gin.Engine) error {
	syncInvokeCounter, err := meter.SyncInt64().Counter("sync_invoke")
	if err != nil {
		return err
	}

	h := &handler{rc, syncInvokeCounter}
	e.Any("/serving/:ns/:ksvc/*path", h.serving)
	e.POST("/eventing/:ns/:broker", h.eventing)
	return nil
}

type handler struct {
	rc                *resty.Client
	syncInvokeCounter syncint64.Counter
}

func (h handler) joinURL(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

func (h handler) serving(c *gin.Context) {
	h.syncInvokeCounter.Add(c, 1)
	res, err := h.rc.R().
		SetContext(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c))).
		SetHeaderMultiValues(c.Request.Header).
		SetBody(c.Request.Body).
		Execute(c.Request.Method, h.joinURL(fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Param("ksvc"), c.Param("ns")), c.Param("path")))
	if err != nil {
		panic(err)
	}
	c.Data(res.StatusCode(), res.Header().Get("Content-Type"), res.Body())
}

func (h handler) eventing(c *gin.Context) {
	res, err := h.rc.R().
		SetContext(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c))).
		SetHeaderMultiValues(c.Request.Header).
		SetBody(c.Request.Body).
		Post(h.joinURL("http://broker-ingress.knative-eventing.svc.cluster.local", c.Param("ns"), c.Param("broker")))
	if err != nil {
		panic(err)
	}
	c.Data(res.StatusCode(), res.Header().Get("Content-Type"), res.Body())
}
