// Package config provides configuration loading and parsing.
// This file defines payload-related configuration and defaults.
package config

// DefaultPayloadDir is the default directory for storing large payloads.
const DefaultPayloadDir = "/tmp/jq-payloads"

func init() {
	// Register default setter for PayloadDir
	RegisterDefaults(func(cfg *Config) {
		if cfg.Gateway != nil && cfg.Gateway.PayloadDir == "" {
			cfg.Gateway.PayloadDir = DefaultPayloadDir
		}
	})
}
