// Package config provides configuration loading and parsing.
// This file defines DIFC (Decentralized Information Flow Control) configuration types.
package config

// GuardConfig represents a DIFC guard configuration (experimental).
type GuardConfig struct {
	// Type is the guard type: "remote" for MCP-based guards
	Type string `toml:"type" json:"type"`

	// Command is the executable command (for stdio guards)
	Command string `toml:"command" json:"command,omitempty"`

	// Args are the command arguments
	Args []string `toml:"args" json:"args,omitempty"`

	// Env holds environment variables for the guard
	Env map[string]string `toml:"env" json:"env,omitempty"`

	// URL is the HTTP endpoint URL for remote guards
	URL string `toml:"url" json:"url,omitempty"`
}

// StdinGuardConfig represents a DIFC guard configuration from stdin JSON (experimental).
type StdinGuardConfig struct {
	// Type is the guard type: "remote" for MCP-based guards
	Type string `json:"type"`

	// Command is the executable command (for stdio guards)
	Command string `json:"command,omitempty"`

	// Args are the command arguments
	Args []string `json:"args,omitempty"`

	// Env holds environment variables
	Env map[string]string `json:"env,omitempty"`

	// Container is the container image (for containerized guards)
	Container string `json:"container,omitempty"`

	// URL is the HTTP endpoint URL for remote guards
	URL string `json:"url,omitempty"`
}
