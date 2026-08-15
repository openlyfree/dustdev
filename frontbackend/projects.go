package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *server) projectURL(slug string) string {
	return fmt.Sprintf("%s://%s.%s", s.cfg.URLScheme, slug, s.cfg.BaseDomain)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$`)

var reservedSlugs = map[string]struct{}{
	"www": {}, "api": {}, "app": {}, "mail": {}, "smtp": {},
	"dashboard": {}, "login": {}, "signup": {}, "auth": {},
	"admin": {}, "support": {}, "status": {}, "cdn": {}, "static": {},
}

func slugify(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := true
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 32 {
		out = strings.TrimRight(out[:32], "-")
	}
	return out
}

func validSlug(slug string) bool {
	if !slugPattern.MatchString(slug) {
		return false
	}
	_, reserved := reservedSlugs[slug]
	return !reserved
}

func (s *server) ownedProject(c *gin.Context) (project, user, bool) {
	u := c.MustGet("user").(user)
	id := c.Param("id")
	var p project
	err := s.db.Pool.QueryRow(c.Request.Context(),
		"SELECT id, name, slug, status, created_at FROM projects WHERE id = $1 AND owner_id = $2",
		id, u.ID).Scan(&p.ID, &p.Name, &p.Slug, &p.Status, &p.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return project{}, user{}, false
	}
	return p, u, true
}

func (s *server) listProjects(c *gin.Context) {
	u := c.MustGet("user").(user)
	rows, err := s.db.Pool.Query(c.Request.Context(),
		"SELECT id, name, slug, status, created_at FROM projects WHERE owner_id = $1 ORDER BY created_at DESC",
		u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list projects"})
		return
	}
	defer rows.Close()

	projects := []project{}
	for rows.Next() {
		var p project
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Status, &p.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read projects"})
			return
		}
		p.URL = s.projectURL(p.Slug)
		projects = append(projects, p)
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (s *server) createProject(c *gin.Context) {
	u := c.MustGet("user").(user)

	var body struct {
		Name string `json:"name" binding:"required,min=1,max=64"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project name required (max 64 chars)"})
		return
	}

	slug := slugify(body.Name)
	if !validSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project name must contain letters or digits"})
		return
	}

	var count int
	if err := s.db.Pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM projects WHERE owner_id = $1", u.ID).Scan(&count); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check quota"})
		return
	}
	if count >= s.cfg.MaxProjects {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("project limit reached (%d)", s.cfg.MaxProjects)})
		return
	}

	var p project
	err := s.db.Pool.QueryRow(c.Request.Context(),
		"INSERT INTO projects (owner_id, name, slug) VALUES ($1, $2, $3) RETURNING id, name, slug, status, created_at",
		u.ID, body.Name, slug).Scan(&p.ID, &p.Name, &p.Slug, &p.Status, &p.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "a project with a similar name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}

	p.URL = s.projectURL(p.Slug)
	c.JSON(http.StatusCreated, p)
}

func (s *server) startProject(c *gin.Context) {
	p, u, ok := s.ownedProject(c)
	if !ok {
		return
	}

	if p.Status != "running" {
		var running int
		if err := s.db.Pool.QueryRow(c.Request.Context(),
			"SELECT count(*) FROM projects WHERE owner_id = $1 AND status = 'running'", u.ID).
			Scan(&running); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check quota"})
			return
		}
		if running >= s.cfg.MaxRunning {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("running limit reached (%d) — stop another project first", s.cfg.MaxRunning),
			})
			return
		}

		if err := s.pod.ensureContainer(c.Request.Context(), s.cfg, p); err != nil {
			s.setProjectStatus(c, p.ID, "error")
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to create container: %v", err)})
			return
		}
		if err := s.pod.startContainer(c.Request.Context(), containerName(p.Slug)); err != nil {
			s.setProjectStatus(c, p.ID, "error")
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to start container: %v", err)})
			return
		}
		s.setProjectStatus(c, p.ID, "running")
		p.Status = "running"
	}

	p.URL = s.projectURL(p.Slug)
	c.JSON(http.StatusOK, p)
}

func (s *server) stopProject(c *gin.Context) {
	p, _, ok := s.ownedProject(c)
	if !ok {
		return
	}

	if p.Status == "running" {
		if err := s.pod.stopContainer(c.Request.Context(), containerName(p.Slug)); err != nil {
			s.setProjectStatus(c, p.ID, "error")
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to stop container: %v", err)})
			return
		}
	}
	s.setProjectStatus(c, p.ID, "stopped")
	p.Status = "stopped"
	p.URL = s.projectURL(p.Slug)
	c.JSON(http.StatusOK, p)
}

func (s *server) deleteProject(c *gin.Context) {
	p, _, ok := s.ownedProject(c)
	if !ok {
		return
	}

	if err := s.pod.removeContainer(c.Request.Context(), containerName(p.Slug)); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to remove container: %v", err)})
		return
	}
	if err := s.pod.removeVolume(c.Request.Context(), volumeName(p.ID)); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to remove volume: %v", err)})
		return
	}
	if _, err := s.db.Pool.Exec(c.Request.Context(), "DELETE FROM projects WHERE id = $1", p.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) setProjectStatus(c *gin.Context, id, status string) {
	if _, err := s.db.Pool.Exec(c.Request.Context(),
		"UPDATE projects SET status = $1 WHERE id = $2", status, id); err != nil {
		c.Error(fmt.Errorf("failed to set project status: %w", err))
	}
}

// reconcileProjects aligns stored statuses with actual container states at boot,
// so a restart of this service (or a crashed container) never leaves stale
// "running" rows behind.
func (s *server) reconcileProjects(ctx context.Context) error {
	states, err := s.pod.listManagedContainerStates(ctx)
	if err != nil {
		return err
	}

	rows, err := s.db.Pool.Query(ctx, "SELECT id, slug, status FROM projects")
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct{ id, slug, status string }
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.slug, &r.status); err != nil {
			return err
		}
		all = append(all, r)
	}

	for _, r := range all {
		want := "stopped"
		if states[containerName(r.slug)] == "running" {
			want = "running"
		}
		if r.status != want {
			if _, err := s.db.Pool.Exec(ctx,
				"UPDATE projects SET status = $1 WHERE id = $2", want, r.id); err != nil {
				return err
			}
		}
	}
	return nil
}
