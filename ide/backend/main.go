package main

import (
	"io/fs"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	staticFS, err := fs.Sub(embeddedFrontend, "dist")
	if err != nil {
		log.Fatalf("Failed to load embedded frontend: %v", err)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatalf("Failed to configure trusted proxies: %v", err)
	}
	router.Use(gin.Recovery())
	router.Use(crossOriginIsolationMiddleware())

	initExcludedPorts()
	registerPreviewRoutes(router)
	router.GET("/term", termSync)
	router.GET("/file", fileSync)
	registerStaticRoutes(router, staticFS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("IDE host listening on :%s (project: %s)", port, getProjectDir())
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
