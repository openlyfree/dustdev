package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func getProjectDir() string {
	if dir := os.Getenv("PROJECT_DIR"); dir != "" {
		clean := filepath.Clean(dir)
		if err := os.MkdirAll(clean, 0o755); err != nil {
			log.Printf("Failed to create PROJECT_DIR %s: %v", clean, err)
		}
		return clean
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf("Failed to get home directory: %v", err)
	}

	projectName := os.Getenv("PROJECT")
	if projectName == "" {
		projectName = "default-project"
	}

	targetDir := filepath.Join(homeDir, filepath.Clean(projectName))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		log.Printf("Failed to create project directory %s: %v", targetDir, err)
	}
	return targetDir
}

func isPathInsideProject(projectDir, fullPath string) bool {
	rel, err := filepath.Rel(projectDir, fullPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func shouldIgnorePath(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		switch part {
		case ".git", ".svelte-kit":
			return true
		}
		if strings.HasSuffix(part, ".swp") || strings.HasSuffix(part, "~") {
			return true
		}
	}
	return false
}

func getLoginShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}
	return "/bin/bash"
}
