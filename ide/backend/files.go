package main

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/radovskyb/watcher"
)

type FileMessage struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

type recentWrite struct {
	path string
	at   time.Time
}

var (
	recentWritesMu sync.Mutex
	recentWrites   []recentWrite
)

func recordClientWrite(path string) {
	recentWritesMu.Lock()
	defer recentWritesMu.Unlock()

	now := time.Now()
	recentWrites = append(recentWrites, recentWrite{path: path, at: now})

	cutoff := now.Add(-2 * time.Second)
	filtered := recentWrites[:0]
	for _, w := range recentWrites {
		if w.at.After(cutoff) {
			filtered = append(filtered, w)
		}
	}
	recentWrites = filtered
}

func shouldSkipWatcherEvent(relPath string) bool {
	recentWritesMu.Lock()
	defer recentWritesMu.Unlock()

	now := time.Now()
	for _, w := range recentWrites {
		if w.path == relPath && now.Sub(w.at) < 500*time.Millisecond {
			return true
		}
	}
	return false
}

func fileSync(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	if err := conn.WriteJSON(bootstrapFileSync()); err != nil {
		log.Printf("Failed to write bootstrap message: %v", err)
	}

	projectDir := getProjectDir()
	w := watcher.New()
	w.IgnoreHiddenFiles(true)

	if err := w.AddRecursive(projectDir); err != nil {
		log.Printf("Failed to watch directory: %v", err)
		conn.Close()
		return
	}

	done := make(chan struct{})

	defer func() {
		close(done)
		w.Close()
		conn.Close()
		log.Println("File sync connection closed")
	}()

	go func() {
		for {
			select {
			case event := <-w.Event:
				if event.IsDir() {
					continue
				}

				relPath, err := filepath.Rel(projectDir, event.Path)
				if err != nil {
					log.Printf("Failed to get relative path: %v", err)
					continue
				}
				relPath = filepath.ToSlash(relPath)

				if shouldIgnorePath(relPath) || shouldSkipWatcherEvent(relPath) {
					continue
				}

				var msg FileMessage
				msg.Path = relPath

				if event.Op == watcher.Remove {
					msg.Data = "delete"
				} else {
					dataRaw, err := os.ReadFile(event.Path)
					if err != nil {
						log.Printf("Failed to read modified file %s: %v", event.Path, err)
						continue
					}
					msg.Data = base64.StdEncoding.EncodeToString(dataRaw)
				}

				if err := conn.WriteJSON(msg); err != nil {
					log.Printf("Failed to send file sync message: %v", err)
					return
				}

			case err := <-w.Error:
				log.Printf("Watcher error: %v", err)
			case <-w.Closed:
				return
			case <-done:
				return
			}
		}
	}()

	go func() {
		if err := w.Start(100 * time.Millisecond); err != nil {
			log.Printf("Watcher start error: %v", err)
		}
	}()

	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg FileMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		if err := handleClientMessage(msg); err != nil {
			log.Printf("Failed to handle client message for path %s: %v", msg.Path, err)
		}
	}
}

func bootstrapFileSync() []FileMessage {
	var filesToSend []FileMessage
	projDir := getProjectDir()

	_ = filepath.WalkDir(projDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(projDir, path)
		if err != nil || shouldIgnorePath(relPath) {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		dataRaw, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Bootstrap read failed for %s: %v", path, err)
			return nil
		}

		filesToSend = append(filesToSend, FileMessage{
			Path: relPath,
			Data: base64.StdEncoding.EncodeToString(dataRaw),
		})
		return nil
	})

	return filesToSend
}

func handleClientMessage(msg FileMessage) error {
	projectDir := getProjectDir()
	cleanPath := filepath.ToSlash(filepath.Clean(msg.Path))
	fullPath := filepath.Join(projectDir, cleanPath)

	if !isPathInsideProject(projectDir, fullPath) {
		log.Printf("Security alert: path traversal prevented (%s)", msg.Path)
		return filepath.ErrBadPattern
	}

	if msg.Data == "delete" {
		if err := os.RemoveAll(fullPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		recordClientWrite(cleanPath)
		return nil
	}

	fileData, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		log.Printf("Failed to decode base64 data: %v", err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(fullPath, fileData, 0o644); err != nil {
		return err
	}

	recordClientWrite(cleanPath)
	return nil
}
