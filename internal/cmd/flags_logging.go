package cmd

// Logging-related flags

import (
	"github.com/github/gh-aw-mcpg/internal/envutil"
	"github.com/spf13/cobra"
)

// Logging flag defaults
const (
	defaultLogDir               = "/tmp/gh-aw/mcp-logs"
	defaultPayloadDir           = "/tmp/jq-payloads"
	defaultPayloadSizeThreshold = 10240 // 10KB default threshold
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
	return envutil.GetEnvString("MCP_GATEWAY_LOG_DIR", defaultLogDir)
}

// getDefaultPayloadDir returns the default payload directory, checking MCP_GATEWAY_PAYLOAD_DIR
// environment variable first, then falling back to the hardcoded default
func getDefaultPayloadDir() string {
	return envutil.GetEnvString("MCP_GATEWAY_PAYLOAD_DIR", defaultPayloadDir)
}

// getDefaultPayloadSizeThreshold returns the default payload size threshold, checking
// MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD environment variable first, then falling back to the hardcoded default
func getDefaultPayloadSizeThreshold() int {
	return envutil.GetEnvInt("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD", defaultPayloadSizeThreshold)
}
