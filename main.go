package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func registerRouter(lc fx.Lifecycle, e *gin.Engine) {
	e.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	e.Any("/:ns/:ksvc/*path", func(c *gin.Context) {
		ksvcURL, _ := url.Parse(fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Param("ksvc"), c.Param("ns")))
		proxy := httputil.NewSingleHostReverseProxy(ksvcURL)
		proxy.Director = func(req *http.Request) {
			req.Header = c.Request.Header
			req.Host = ksvcURL.Host
			req.URL.Scheme = ksvcURL.Scheme
			req.URL.Host = ksvcURL.Host
			req.URL.Path = c.Param("path")
		}
		proxy.ServeHTTP(c.Writer, c.Request)
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
		fx.Provide(gin.Default),
		fx.Invoke(registerRouter),
	).Run()
}
