package cmd

// DIFC (Decentralized Information Flow Control) related flags

import (
	"github.com/github/gh-aw-mcpg/internal/envutil"
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
		cmd.Flags().BoolVar(&enableDIFC, "enable-difc", getDefaultEnableDIFC(), "Enable DIFC enforcement for information flow control")
	})
}

// getDefaultEnableDIFC returns the default DIFC setting, checking MCP_GATEWAY_ENABLE_DIFC
// environment variable first, then falling back to the hardcoded default (false)
func getDefaultEnableDIFC() bool {
	return envutil.GetEnvBool("MCP_GATEWAY_ENABLE_DIFC", defaultEnableDIFC)
}
