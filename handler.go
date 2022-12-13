package main

import (
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	ce_client "github.com/cloudevents/sdk-go/v2/client"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

func RegisterHandler(rc *resty.Client, cec ce_client.Client, e *gin.Engine) error {
	syncInvokeCounter, err := meter.SyncInt64().Counter("sync_invoke")
	if err != nil {
		return err
	}

	h := &handler{rc, cec, syncInvokeCounter}
	e.Any("/serving/:ns/:ksvc/*path", h.serving)
	e.POST("/eventing/:ns/:broker", h.eventing)
	e.POST("/eventing/:ns/:broker/webhook", h.eventingWebhook)
	return nil
}

type handler struct {
	rc                *resty.Client
	cec               ce_client.Client
	syncInvokeCounter syncint64.Counter
}

func (h handler) serving(c *gin.Context) {
	h.syncInvokeCounter.Add(c, 1)
	ksvcURL, err := url.JoinPath(fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Param("ksvc"), c.Param("ns")), c.Param("path"))
	if err != nil {
		panic(err)
	}
	res, err := h.rc.R().
		SetContext(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c))).
		SetHeaderMultiValues(c.Request.Header).
		SetBody(c.Request.Body).
		Execute(c.Request.Method, ksvcURL)
	if err != nil {
		panic(err)
	}
	c.Data(res.StatusCode(), res.Header().Get("Content-Type"), res.Body())
}

func (h handler) eventing(c *gin.Context) {
	event, err := cloudevents.NewEventFromHTTPRequest(c.Request)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	brokerURL, err := url.JoinPath("http://broker-ingress.knative-eventing.svc.cluster.local", c.Param("ns"), c.Param("broker"))
	if err != nil {
		panic(err)
	}

	res := h.cec.Send(cloudevents.ContextWithTarget(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c)), brokerURL), *event)
	if cloudevents.IsUndelivered(res) || cloudevents.IsNACK(res) {
		panic(res.Error())
	}

	c.JSON(http.StatusAccepted, gin.H{"id": event.ID()})
}

func (h handler) eventingWebhook(c *gin.Context) {
	var reqBody any
	if err := c.BindJSON(&reqBody); err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	source := c.GetHeader("User-Agent")
	if source == "" {
		source = "kngw/1.0.0"
	}

	id, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	event := cloudevents.NewEvent()
	event.SetID(id.String())
	event.SetSource(source)
	event.SetType("Webhook")
	event.SetData(cloudevents.ApplicationJSON, map[string]any{"headers": c.Request.Header, "body": reqBody})

	brokerURL, err := url.JoinPath("http://broker-ingress.knative-eventing.svc.cluster.local", c.Param("ns"), c.Param("broker"))
	if err != nil {
		panic(err)
	}

	res := h.cec.Send(cloudevents.ContextWithTarget(httptrace.WithClientTrace(c, otelhttptrace.NewClientTrace(c)), brokerURL), event)
	if cloudevents.IsUndelivered(res) || cloudevents.IsNACK(res) {
		panic(res.Error())
	}

	c.JSON(http.StatusAccepted, gin.H{"id": event.ID()})
}
