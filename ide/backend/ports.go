package main

import (
	"bufio"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var excludedPorts = map[int]struct{}{}

var blockedSystemPorts = map[int]struct{}{
	631:  {}, // CUPS
	5353: {}, // mDNS
	5355: {}, // mDNS
}

func initExcludedPorts() {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	excludedPorts[port] = struct{}{}
}

func isExcludedPort(port int) bool {
	if port < 1024 || port > 65535 {
		return true
	}
	if _, ok := excludedPorts[port]; ok {
		return true
	}
	_, blocked := blockedSystemPorts[port]
	return blocked
}

func isLocalBind(hexIP string) bool {
	switch strings.ToUpper(hexIP) {
	case "00000000", "0100007F":
		return true
	case "00000000000000000000000000000000", "01000000000000000000000000000001":
		return true
	default:
		return false
	}
}

func parseListeningPorts() ([]int, error) {
	found := map[int]struct{}{}

	for _, procFile := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(procFile)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		if !scanner.Scan() {
			_ = f.Close()
			continue
		}

		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 {
				continue
			}
			if fields[3] != "0A" {
				continue
			}

			hostPort := strings.Split(fields[1], ":")
			if len(hostPort) != 2 {
				continue
			}
			if !isLocalBind(hostPort[0]) {
				continue
			}

			port, err := strconv.ParseInt(hostPort[1], 16, 32)
			if err != nil {
				continue
			}
			if isExcludedPort(int(port)) {
				continue
			}
			found[int(port)] = struct{}{}
		}

		_ = f.Close()
	}

	ports := make([]int, 0, len(found))
	for port := range found {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}

func listPorts(c *gin.Context) {
	ports, err := parseListeningPorts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ports": ports})
}

func validatePreviewPort(portStr string) (int, bool) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, false
	}
	if isExcludedPort(port) {
		return 0, false
	}
	return port, true
}
