package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func registerPreviewRoutes(router *gin.Engine) {
	router.GET("/ports", listPorts)
	router.Any("/preview/:port", previewProxy)
	router.Any("/preview/:port/*filepath", previewProxy)
}

func previewProxy(c *gin.Context) {
	port, ok := validatePreviewPort(c.Param("port"))
	if !ok {
		c.String(http.StatusBadRequest, "invalid port")
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid target")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, fmt.Sprintf("preview unavailable: %v", err), http.StatusBadGateway)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		prefix := fmt.Sprintf("/preview/%d", port)
		if loc := resp.Header.Get("Location"); strings.HasPrefix(loc, target.String()) {
			resp.Header.Set("Location", prefix+strings.TrimPrefix(loc, target.String()))
		}
		return nil
	}

	path := c.Param("filepath")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	c.Request.URL.Path = path
	c.Request.Host = target.Host

	proxy.ServeHTTP(c.Writer, c.Request)
}
