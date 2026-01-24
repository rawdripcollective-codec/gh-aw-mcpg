// Package config provides configuration loading and parsing.
// This file defines DIFC (Decentralized Information Flow Control) configuration types.
package config

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
