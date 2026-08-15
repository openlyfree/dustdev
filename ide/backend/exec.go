package main

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	execTimeout   = 2 * time.Minute
	execMaxOutput = 20000
)

type execRequest struct {
	Command string `json:"command" binding:"required"`
}

type execResponse struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

// execSync runs a shell command in the project directory and returns its
// combined output and exit code. Used by the in-IDE assistant.
func execSync(c *gin.Context) {
	var req execRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected JSON body with a command field"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, getLoginShell(), "-c", req.Command)
	cmd.Dir = getProjectDir()

	out, err := cmd.CombinedOutput()
	output := string(out)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		output += "\n[command timed out]"
		c.JSON(http.StatusOK, execResponse{ExitCode: -1, Output: truncateExecOutput(output)})
		return
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, execResponse{ExitCode: exitCode, Output: truncateExecOutput(output)})
}

func truncateExecOutput(output string) string {
	if len(output) > execMaxOutput {
		return output[:execMaxOutput] + "\n[output truncated]"
	}
	return output
}
