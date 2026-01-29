package cmd

// DIFC (Decentralized Information Flow Control) related flags

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// DIFC flag defaults
const (
	defaultEnableDIFC = false
	defaultDIFCFilter = false
)

// DIFC flag variables
var (
	enableDIFC bool
	difcFilter bool
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().BoolVar(&enableDIFC, "enable-difc", getDefaultEnableDIFC(), "Enable DIFC enforcement and session requirement (requires sys___init call before tool access)")
		cmd.Flags().BoolVar(&difcFilter, "difc-filter", getDefaultDIFCFilter(), "Enable DIFC response filtering (removes content that violates agent labels)")
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
