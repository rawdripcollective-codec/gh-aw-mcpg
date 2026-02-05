package version

// gatewayVersion stores the gateway version string, used across multiple packages
// for error reporting, health checks, and MCP client implementation info.
// It defaults to "dev" and should be set once at startup.
var gatewayVersion = "dev"

// Set updates the gateway version string if the provided version is non-empty.
// This should be called once at application startup from main.
func Set(v string) {
	if v != "" {
		gatewayVersion = v
	}
}

// Get returns the current gateway version string.
func Get() string {
	return gatewayVersion
}
