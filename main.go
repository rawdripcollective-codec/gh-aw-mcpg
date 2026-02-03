package main

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/github/gh-aw-mcpg/internal/cmd"
)

func main() {
	// Build version string with metadata
	versionStr := buildVersionString()

	// Set the version for the CLI
	cmd.SetVersion(versionStr)

	// Execute the root command
	cmd.Execute()
}

const (
	shortHashLength = 7 // Length for short git commit hash
)

// buildVersionString constructs a detailed version string with build metadata
func buildVersionString() string {
	var parts []string

	// Add main version
	if Version != "" {
		parts = append(parts, Version)
	} else {
		parts = append(parts, "dev")
	}

	// Add git commit if available
	if GitCommit != "" {
		parts = append(parts, fmt.Sprintf("commit: %s", GitCommit))
	} else if buildInfo, ok := debug.ReadBuildInfo(); ok {
		// Try to extract commit from build info if not set via ldflags
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" {
				commitHash := setting.Value
				if len(commitHash) > shortHashLength {
					commitHash = commitHash[:shortHashLength] // Short hash
				}
				parts = append(parts, fmt.Sprintf("commit: %s", commitHash))
				break
			}
		}
	}

	// Add build date if available
	if BuildDate != "" {
		parts = append(parts, fmt.Sprintf("built: %s", BuildDate))
	} else if buildInfo, ok := debug.ReadBuildInfo(); ok {
		// Try to extract build time from build info if not set via ldflags
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.time" {
				parts = append(parts, fmt.Sprintf("built: %s", setting.Value))
				break
			}
		}
	}

	return strings.Join(parts, ", ")
}
