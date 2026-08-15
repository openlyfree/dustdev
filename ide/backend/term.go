package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WSMessage handles multiplexed control messages and raw input
type WSMessage struct {
	Type string `json:"type"` // "input" or "resize"
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

func termSync(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	cmd := exec.Command(getLoginShell())
	cmd.Dir = getProjectDir()
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Start PTY with a sensible default size
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		log.Printf("PTY start error: %v", err)
		return
	}

	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	var wsMutex sync.Mutex
	writeToWS := func(messageType int, data []byte) error {
		wsMutex.Lock()
		defer wsMutex.Unlock()
		return conn.WriteMessage(messageType, data)
	}

	// Read from PTY -> Write to WS
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 2048)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				return
			}
			if err := writeToWS(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// Read from WS -> Process command or write to PTY
	for {
		messageType, rawMessage, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Check if message is a JSON resize or input control packet
		var msg WSMessage
		if err := json.Unmarshal(rawMessage, &msg); err == nil && msg.Type != "" {
			switch msg.Type {
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = pty.Setsize(ptmx, &pty.Winsize{
						Rows: msg.Rows,
						Cols: msg.Cols,
					})
				}
			case "input":
				_, _ = ptmx.Write([]byte(msg.Data))
			}
		} else {
			// Raw fallback (if message sent directly as binary or raw string)
			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				_, _ = ptmx.Write(rawMessage)
			}
		}
	}

	<-done
}
