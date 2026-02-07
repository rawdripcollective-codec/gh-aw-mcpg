package cmd

// Logging-related flags

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Logging flag defaults
const (
	defaultLogDir               = "/tmp/gh-aw/mcp-logs"
	defaultPayloadDir           = "/tmp/jq-payloads"
	defaultPayloadSizeThreshold = 1024 // 1KB default threshold
)

// Logging flag variables
var (
	logDir               string
	payloadDir           string
	payloadSizeThreshold int
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&logDir, "log-dir", getDefaultLogDir(), "Directory for log files (falls back to stdout if directory cannot be created)")
		cmd.Flags().StringVar(&payloadDir, "payload-dir", getDefaultPayloadDir(), "Directory for storing large payload files (segmented by session ID)")
		cmd.Flags().IntVar(&payloadSizeThreshold, "payload-size-threshold", getDefaultPayloadSizeThreshold(), "Size threshold (in bytes) for storing payloads to disk. Payloads larger than this are stored, smaller ones returned inline")
	})
}

// getDefaultLogDir returns the default log directory, checking MCP_GATEWAY_LOG_DIR
// environment variable first, then falling back to the hardcoded default
func getDefaultLogDir() string {
	if envLogDir := os.Getenv("MCP_GATEWAY_LOG_DIR"); envLogDir != "" {
		return envLogDir
	}
	return defaultLogDir
}

// getDefaultPayloadDir returns the default payload directory, checking MCP_GATEWAY_PAYLOAD_DIR
// environment variable first, then falling back to the hardcoded default
func getDefaultPayloadDir() string {
	if envPayloadDir := os.Getenv("MCP_GATEWAY_PAYLOAD_DIR"); envPayloadDir != "" {
		return envPayloadDir
	}
	return defaultPayloadDir
}

// getDefaultPayloadSizeThreshold returns the default payload size threshold, checking
// MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD environment variable first, then falling back to the hardcoded default
func getDefaultPayloadSizeThreshold() int {
	if envThreshold := os.Getenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD"); envThreshold != "" {
		// Try to parse as integer
		var threshold int
		if _, err := fmt.Sscanf(envThreshold, "%d", &threshold); err == nil && threshold > 0 {
			return threshold
		}
		// Invalid value, use default
	}
	return defaultPayloadSizeThreshold
}
