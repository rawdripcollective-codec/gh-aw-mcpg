package cmd

// Core flags - these are the essential flags that rarely change.
// Feature-specific flags should go in separate files (e.g., flags_difc.go).

import "github.com/spf13/cobra"

// Core flag defaults
const (
	defaultConfigFile  = ""
	defaultConfigStdin = false
	defaultListenAddr  = DefaultListenIPv4 + ":" + DefaultListenPort
	defaultRoutedMode  = false
	defaultUnifiedMode = false
	defaultEnvFile     = ""
)

// Core flag variables
var (
	configFile  string
	configStdin bool
	listenAddr  string
	routedMode  bool
	unifiedMode bool
	envFile     string
	validateEnv bool
	verbosity   int
)

func init() {
	RegisterFlag(registerCoreFlags)
}

func registerCoreFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&configFile, "config", "c", defaultConfigFile, "Path to config file")
	cmd.Flags().BoolVar(&configStdin, "config-stdin", defaultConfigStdin, "Read MCP server configuration from stdin (JSON format). When enabled, overrides --config")
	cmd.Flags().StringVarP(&listenAddr, "listen", "l", defaultListenAddr, "HTTP server listen address")
	cmd.Flags().BoolVar(&routedMode, "routed", defaultRoutedMode, "Run in routed mode (each backend at /mcp/<server>)")
	cmd.Flags().BoolVar(&unifiedMode, "unified", defaultUnifiedMode, "Run in unified mode (all backends at /mcp)")
	cmd.Flags().StringVar(&envFile, "env", defaultEnvFile, "Path to .env file to load environment variables")
	cmd.Flags().BoolVar(&validateEnv, "validate-env", false, "Validate execution environment (Docker, env vars) before starting")
	cmd.Flags().CountVarP(&verbosity, "verbose", "v", "Increase verbosity level (use -v for info, -vv for debug, -vvv for trace)")

	// Mark mutually exclusive flags
	cmd.MarkFlagsMutuallyExclusive("routed", "unified")
}
