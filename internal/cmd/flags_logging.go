package cmd

// Logging-related flags

import (
	"os"

	"github.com/spf13/cobra"
)

// Logging flag defaults
const (
	defaultLogDir     = "/tmp/gh-aw/mcp-logs"
	defaultPayloadDir = "/tmp/jq-payloads"
)

// Logging flag variables
var (
	logDir     string
	payloadDir string
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&logDir, "log-dir", getDefaultLogDir(), "Directory for log files (falls back to stdout if directory cannot be created)")
		cmd.Flags().StringVar(&payloadDir, "payload-dir", getDefaultPayloadDir(), "Directory for storing large payload files (segmented by session ID)")
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
