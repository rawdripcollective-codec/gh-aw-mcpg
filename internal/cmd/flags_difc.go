package cmd

// DIFC (Decentralized Information Flow Control) related flags

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// DIFC flag defaults
const (
	defaultEnableDIFC       = false
	defaultDIFCFilter       = false
	defaultConfigExtensions = false
)

// DIFC flag variables
var (
	enableDIFC       bool
	difcFilter       bool
	enableConfigExt  bool   // Enable config extensions (guards, session labels)
	sessionSecrecy   string // Comma-separated initial secrecy labels
	sessionIntegrity string // Comma-separated initial integrity labels
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().BoolVar(&enableDIFC, "enable-difc", getDefaultEnableDIFC(), "Enable DIFC enforcement (sessions are auto-created from Authorization header)")
		cmd.Flags().BoolVar(&difcFilter, "difc-filter", getDefaultDIFCFilter(), "Enable DIFC response filtering based on path labels (requires --enable-difc)")
		cmd.Flags().BoolVar(&enableConfigExt, "enable-config-extensions", getDefaultConfigExtensions(), "Enable config extensions (guards, session labels) - required for DIFC features")
		cmd.Flags().StringVar(&sessionSecrecy, "session-secrecy", getDefaultSessionSecrecy(), "Comma-separated initial secrecy labels for agent sessions (requires --enable-config-extensions)")
		cmd.Flags().StringVar(&sessionIntegrity, "session-integrity", getDefaultSessionIntegrity(), "Comma-separated initial integrity labels for agent sessions (requires --enable-config-extensions)")
	})
}

// getDefaultEnableDIFC returns the default DIFC setting, checking MCP_GATEWAY_ENABLE_DIFC
// environment variable first, then falling back to the hardcoded default (false)
func getDefaultEnableDIFC() bool {
	if envDIFC := os.Getenv("MCP_GATEWAY_ENABLE_DIFC"); envDIFC != "" {
		switch strings.ToLower(envDIFC) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return defaultEnableDIFC
}

// getDefaultDIFCFilter returns the default DIFC filter setting, checking MCP_GATEWAY_DIFC_FILTER
// environment variable first, then falling back to the hardcoded default (false)
func getDefaultDIFCFilter() bool {
	if envFilter := os.Getenv("MCP_GATEWAY_DIFC_FILTER"); envFilter != "" {
		switch strings.ToLower(envFilter) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return defaultDIFCFilter
}

// getDefaultConfigExtensions returns the default config extensions setting,
// checking MCP_GATEWAY_CONFIG_EXTENSIONS environment variable first
func getDefaultConfigExtensions() bool {
	if envConfigExt := os.Getenv("MCP_GATEWAY_CONFIG_EXTENSIONS"); envConfigExt != "" {
		switch strings.ToLower(envConfigExt) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return defaultConfigExtensions
}

// getDefaultSessionSecrecy returns the default session secrecy labels from
// MCP_GATEWAY_SESSION_SECRECY environment variable
func getDefaultSessionSecrecy() string {
	return os.Getenv("MCP_GATEWAY_SESSION_SECRECY")
}

// getDefaultSessionIntegrity returns the default session integrity labels from
// MCP_GATEWAY_SESSION_INTEGRITY environment variable
func getDefaultSessionIntegrity() string {
	return os.Getenv("MCP_GATEWAY_SESSION_INTEGRITY")
}
