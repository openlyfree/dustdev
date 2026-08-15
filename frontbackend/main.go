package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

type server struct {
	cfg config
	db  *db
	pod *podmanClient
}

func main() {
	cfg := loadConfig()

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()
	database, err := connectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	s := &server{
		cfg: cfg,
		db:  database,
		pod: newPodmanClient(cfg.PodmanSocket),
	}

	if err := s.reconcileProjects(ctx); err != nil {
		log.Printf("Failed to reconcile project states (podman unavailable?): %v", err)
	}

	go s.sessionCleanupLoop(ctx)

	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatalf("Failed to configure trusted proxies: %v", err)
	}
	router.Use(gin.Logger(), gin.Recovery())

	api := router.Group("/api")
	api.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	api.POST("/signup", s.signup)
	api.POST("/login", s.login)
	api.POST("/logout", s.logout)
	api.GET("/authz", s.authz)

	authed := api.Group("", s.requireAuth())
	authed.GET("/me", s.me)
	authed.GET("/projects", s.listProjects)
	authed.POST("/projects", s.createProject)
	authed.POST("/projects/:id/start", s.startProject)
	authed.POST("/projects/:id/stop", s.stopProject)
	authed.DELETE("/projects/:id", s.deleteProject)

	log.Printf("frontbackend listening on :%s (domain: %s)", cfg.Port, cfg.BaseDomain)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *server) sessionCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.db.Pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at < now()"); err != nil {
				log.Printf("Failed to clean expired sessions: %v", err)
			}
		}
	}
}
