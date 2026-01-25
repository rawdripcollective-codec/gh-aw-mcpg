package tty

import (
	"os"
	"strings"
)

// IsRunningInContainer detects if the current process is running inside a container
func IsRunningInContainer() bool {
	// Method 1: Check for /.dockerenv file (Docker-specific)
	if _, err := os.Stat("/.dockerenv"); err == nil {
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
			return true
		}
	}

	// Method 3: Check environment variable (set by Dockerfile)
	if os.Getenv("RUNNING_IN_CONTAINER") == "true" {
		return true
	}

	return false
}
