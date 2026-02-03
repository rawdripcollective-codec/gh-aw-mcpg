// Package config provides configuration loading and parsing.
// This file defines DIFC (Decentralized Information Flow Control) configuration types.
package config

func init() {
	// Register a stdin converter for session configuration
	RegisterStdinConverter(func(cfg *Config, stdinCfg *StdinConfig) {
		// Convert session config if present
		if stdinCfg.Gateway != nil && stdinCfg.Gateway.Session != nil {
			if cfg.Gateway == nil {
				cfg.Gateway = &GatewayConfig{}
			}
			cfg.Gateway.Session = &SessionConfig{
				Secrecy:   stdinCfg.Gateway.Session.Secrecy,
				Integrity: stdinCfg.Gateway.Session.Integrity,
			}
		}
	})
}

// GuardConfig represents a DIFC guard configuration (experimental).
type GuardConfig struct {
	// Type is the guard type: "wasm" for WebAssembly guards
	Type string `toml:"type" json:"type"`

	// Path is the path to the WASM file (mutually exclusive with URL)
	Path string `toml:"path" json:"path,omitempty"`

	// URL is the URL to download WASM file from (mutually exclusive with Path)
	URL string `toml:"url" json:"url,omitempty"`

	// SHA256 is the checksum for URL downloads (required when URL is set)
	SHA256 string `toml:"sha256" json:"sha256,omitempty"`

	// CacheDir is the directory to cache downloaded WASM files (optional)
	CacheDir string `toml:"cache_dir" json:"cacheDir,omitempty"`
}

// StdinGuardConfig represents a DIFC guard configuration from stdin JSON (experimental).
type StdinGuardConfig struct {
	// Type is the guard type: "wasm" for WebAssembly guards
	Type string `json:"type"`

	// Path is the path to the WASM file (mutually exclusive with URL)
	Path string `json:"path,omitempty"`

	// URL is the URL to download WASM file from (mutually exclusive with Path)
	URL string `json:"url,omitempty"`

	// SHA256 is the checksum for URL downloads (required when URL is set)
	SHA256 string `json:"sha256,omitempty"`

	// CacheDir is the directory to cache downloaded WASM files (optional)
	CacheDir string `json:"cacheDir,omitempty"`
}

// SessionConfig represents initial DIFC labels for agent sessions.
// See github-difc.md section 11.5 for specification.
type SessionConfig struct {
	// Secrecy holds initial secrecy clearance tags
	Secrecy []string `toml:"secrecy" json:"secrecy,omitempty"`

	// Integrity holds initial integrity clearance tags
	Integrity []string `toml:"integrity" json:"integrity,omitempty"`
}

// StdinSessionConfig represents session configuration from stdin JSON.
type StdinSessionConfig struct {
	// Secrecy holds initial secrecy clearance tags
	Secrecy []string `json:"secrecy,omitempty"`

	// Integrity holds initial integrity clearance tags
	Integrity []string `json:"integrity,omitempty"`
}
