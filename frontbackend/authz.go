package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// touchProjectActivity bumps the idle clock for a project. Best-effort: a
// failed update only risks an inaccurate idle timestamp, so errors are logged
// by the caller's context, not surfaced to the request.
func (s *server) touchProjectActivity(ctx context.Context, slug string) {
	_, _ = s.db.Pool.Exec(ctx,
		"UPDATE projects SET last_activity = now() WHERE slug = $1", slug)
}

// authz is the Caddy forward_auth endpoint guarding *.BASE_DOMAIN requests
// (idehost itself has no authentication). Caddy copies the original request
// (including the Host header and cookies) to this endpoint before proxying:
//
//	204 — session valid and user owns the project: request proceeds
//	302 — no session: browser is redirected to the login page
//	403 — session valid but the project belongs to someone else
//	404 — unknown project subdomain
func (s *server) authz(c *gin.Context) {
	host := c.Request.Host
	if forwarded := c.GetHeader("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	host, _, _ = strings.Cut(host, ":")

	suffix := "." + s.cfg.BaseDomain
	if !strings.HasSuffix(host, suffix) {
		c.Status(http.StatusBadRequest)
		return
	}
	slug := strings.TrimSuffix(host, suffix)
	// Wildcard is single-label; reject anything nested or empty.
	if slug == "" || strings.Contains(slug, ".") || !validSlug(slug) {
		c.Status(http.StatusBadRequest)
		return
	}

	u, ok := s.userFromRequest(c)
	if !ok {
		loginURL := s.cfg.URLScheme + "://" + s.cfg.BaseDomain + "/login"
		c.Redirect(http.StatusFound, loginURL)
		return
	}

	var ownerID string
	err := s.db.Pool.QueryRow(c.Request.Context(),
		"SELECT owner_id FROM projects WHERE slug = $1", slug).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if ownerID != u.ID {
		c.Status(http.StatusForbidden)
		return
	}

	// Any proxied IDE request counts as activity; reset the idle clock.
	s.touchProjectActivity(c.Request.Context(), slug)

	c.Status(http.StatusNoContent)
}
