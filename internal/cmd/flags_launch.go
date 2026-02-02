package cmd

// Launch behavior flags

import "github.com/spf13/cobra"

// Launch flag defaults
const (
	defaultSequentialLaunch = false
)

// Launch flag variables
var (
	sequentialLaunch bool
)

func init() {
	RegisterFlag(func(cmd *cobra.Command) {
		cmd.Flags().BoolVar(&sequentialLaunch, "sequential-launch", defaultSequentialLaunch, "Launch MCP servers sequentially during startup (parallel launch is default)")
	})
}
