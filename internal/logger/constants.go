package logger

// Environment variable names for logging configuration
const (
	// EnvDebug is the environment variable for debug logging patterns.
	// Supports patterns like "*", "namespace:*", "ns1,ns2", "ns:*,-ns:skip"
	EnvDebug = "DEBUG"

	// EnvDebugColors controls colored output in debug logs.
	// Set to "0" to disable colors.
	EnvDebugColors = "DEBUG_COLORS"
)
