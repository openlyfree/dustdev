package main

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func crossOriginIsolationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cross-Origin-Embedder-Policy", "require-corp")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Next()
	}
}

func registerStaticRoutes(router *gin.Engine, staticFS fs.FS) {
	router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	fileServer := http.FileServer(http.FS(staticFS))

	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			serveIndex(c, staticFS)
			return
		}

		if f, err := staticFS.Open(path); err == nil {
			defer f.Close()
			if stat, err := f.Stat(); err == nil && !stat.IsDir() {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		serveIndex(c, staticFS)
	})
}

func serveIndex(c *gin.Context, staticFS fs.FS) {
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "frontend not embedded — run `make all` first")
		return
	}
	_, _ = c.Writer.Write(data)
}
