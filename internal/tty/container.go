package tty

import (
	"os"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var log = logger.New("tty:container")

// IsRunningInContainer detects if the current process is running inside a container
func IsRunningInContainer() bool {
	log.Print("Detecting container environment")
	
	// Method 1: Check for /.dockerenv file (Docker-specific)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		log.Print("Container detected: /.dockerenv file exists")
		return true
	}

	// Method 2: Check /proc/1/cgroup for container indicators
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "lxc") {
			log.Print("Container detected: /proc/1/cgroup contains container indicators")
			return true
		}
	} else {
		log.Printf("Failed to read /proc/1/cgroup: %v", err)
	}

	// Method 3: Check environment variable (set by Dockerfile)
	if os.Getenv("RUNNING_IN_CONTAINER") == "true" {
		log.Print("Container detected: RUNNING_IN_CONTAINER=true")
		return true
	}

	log.Print("Not running in container")
	return false
}
