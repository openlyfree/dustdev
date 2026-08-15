package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type config struct {
	Port           string
	DatabaseURL    string
	PodmanSocket   string
	PodmanNetwork  string
	BaseDomain     string
	URLScheme      string
	IDEImage       string
	CookieDomain   string // empty = host-only cookie
	CookieSecure   bool
	MaxProjects    int
	MaxRunning     int
	ContainerMemMB int64
	ContainerCPUs  float64
	SessionTTLDays int
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// defaultIDEImage points at the mutable production tag CI publishes. When
// GITHUB_REPOSITORY is set (owner/repo) we derive the registry path from it so
// forks work without code changes; otherwise fall back to the canonical repo.
func defaultIDEImage() string {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		return "ghcr.io/" + strings.ToLower(repo) + "-idehost:production"
	}
	return "ghcr.io/ethan/code-az-idehost:production"
}

func loadConfig() config {
	baseDomain := envStr("BASE_DOMAIN", "dustdev.app")
	cookieSecure := envBool("COOKIE_SECURE", true)
	cookieDomain := envStr("COOKIE_DOMAIN", baseDomain)
	if baseDomain == "localhost" {
		// Local development: host-only cookie over plain HTTP.
		cookieSecure = false
		cookieDomain = ""
	}

	cfg := config{
		Port:           envStr("PORT", "8081"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		PodmanSocket:   envStr("PODMAN_SOCKET", "/run/podman/podman.sock"),
		PodmanNetwork:  envStr("PODMAN_NETWORK", "dustdev"),
		BaseDomain:     baseDomain,
		URLScheme:      envStr("URL_SCHEME", "https"),
		IDEImage:       envStr("IDE_IMAGE", defaultIDEImage()),
		CookieDomain:   cookieDomain,
		CookieSecure:   cookieSecure,
		MaxProjects:    envInt("MAX_PROJECTS_PER_USER", 5),
		MaxRunning:     envInt("MAX_RUNNING_PER_USER", 2),
		ContainerMemMB: envInt64("CONTAINER_MEMORY_MB", 2048),
		ContainerCPUs:  envFloat("CONTAINER_CPUS", 2),
		SessionTTLDays: envInt("SESSION_TTL_DAYS", 30),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	return cfg
}
