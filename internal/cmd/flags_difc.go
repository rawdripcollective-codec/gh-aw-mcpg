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
)

// DIFC flag variables
var (
	enableDIFC bool
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().BoolVar(&enableDIFC, "enable-difc", getDefaultEnableDIFC(), "Enable DIFC enforcement and session requirement (requires sys___init call before tool access)")
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
