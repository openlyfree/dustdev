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
	go s.idleReaperLoop(ctx)

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

// idleReaperLoop stops running containers that have had no IDE traffic for
// longer than IdleTimeout. A stopped container keeps its volume, so no work is
// lost; the next Start or page load boots it again.
func (s *server) idleReaperLoop(ctx context.Context) {
	if s.cfg.IdleTimeout <= 0 {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reapIdleProjects(ctx); err != nil {
				log.Printf("idle reaper: %v", err)
			}
		}
	}
}

func (s *server) reapIdleProjects(ctx context.Context) error {
	rows, err := s.db.Pool.Query(ctx,
		"SELECT id, slug FROM projects WHERE status = 'running' AND last_activity < now() - $1::interval",
		s.cfg.IdleTimeout.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	type idle struct{ id, slug string }
	var stale []idle
	for rows.Next() {
		var p idle
		if err := rows.Scan(&p.id, &p.slug); err != nil {
			return err
		}
		stale = append(stale, p)
	}
	rows.Close()

	for _, p := range stale {
		if err := s.pod.stopContainer(ctx, containerName(p.slug)); err != nil {
			log.Printf("idle reaper: stop %s: %v", p.slug, err)
			continue
		}
		if _, err := s.db.Pool.Exec(ctx,
			"UPDATE projects SET status = 'stopped' WHERE id = $1", p.id); err != nil {
			log.Printf("idle reaper: mark %s stopped: %v", p.slug, err)
			continue
		}
		log.Printf("idle reaper: stopped %s after %s idle", p.slug, s.cfg.IdleTimeout)
	}
	return nil
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
