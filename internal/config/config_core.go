// Package config provides configuration loading and parsing.
// This file defines the core configuration types that are stable and rarely change.
package config

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/github/gh-aw-mcpg/internal/logger"
)

// Core constants for configuration defaults
const (
	DefaultPort           = 3000
	DefaultStartupTimeout = 60  // seconds
	DefaultToolTimeout    = 120 // seconds
)

// Config represents the internal gateway configuration.
// Feature-specific fields are added in their respective config_*.go files.
type Config struct {
	// Servers maps server names to their configurations
	Servers map[string]*ServerConfig `toml:"servers" json:"servers"`

	// Gateway holds global gateway settings
	Gateway *GatewayConfig `toml:"gateway" json:"gateway,omitempty"`

	// EnableDIFC enables Decentralized Information Flow Control
	EnableDIFC bool `toml:"enable_difc" json:"enable_difc,omitempty"`

	// SequentialLaunch launches servers sequentially instead of in parallel
	SequentialLaunch bool `toml:"sequential_launch" json:"sequential_launch,omitempty"`
}

// GatewayConfig holds global gateway settings.
// Feature-specific fields are added in their respective config_*.go files.
type GatewayConfig struct {
	// Port is the HTTP port to listen on
	Port int `toml:"port" json:"port,omitempty"`

	// APIKey is the authentication key for the gateway
	APIKey string `toml:"api_key" json:"api_key,omitempty"`

	// Domain is the gateway domain for external access
	Domain string `toml:"domain" json:"domain,omitempty"`

	// StartupTimeout is the maximum time (seconds) to wait for server startup
	StartupTimeout int `toml:"startup_timeout" json:"startup_timeout,omitempty"`

	// ToolTimeout is the maximum time (seconds) to wait for tool execution
	ToolTimeout int `toml:"tool_timeout" json:"tool_timeout,omitempty"`

	// PayloadDir is the directory for storing large payloads
	PayloadDir string `toml:"payload_dir" json:"payload_dir,omitempty"`

	// PayloadSizeThreshold is the size threshold (in bytes) for storing payloads to disk.
	// Payloads larger than this threshold are stored to disk, smaller ones are returned inline.
	// Default: 10240 bytes (10KB)
	PayloadSizeThreshold int `toml:"payload_size_threshold" json:"payload_size_threshold,omitempty"`
}

// ServerConfig represents an individual MCP server configuration.
type ServerConfig struct {
	// Type is the server type: "stdio" or "http"
	Type string `toml:"type" json:"type,omitempty"`

	// Command is the executable command (for stdio servers)
	Command string `toml:"command" json:"command,omitempty"`

	// Args are the command arguments (for stdio servers)
	Args []string `toml:"args" json:"args,omitempty"`

	// Env holds environment variables for the server
	Env map[string]string `toml:"env" json:"env,omitempty"`

	// WorkingDirectory is the working directory for the server
	WorkingDirectory string `toml:"working_directory" json:"working_directory,omitempty"`

	// URL is the HTTP endpoint (for http servers)
	URL string `toml:"url" json:"url,omitempty"`

	// Headers are HTTP headers to send (for http servers)
	Headers map[string]string `toml:"headers" json:"headers,omitempty"`

	// Tools is an optional list of tools to filter/expose
	Tools []string `toml:"tools" json:"tools,omitempty"`
}

// LoadFromFile loads configuration from a TOML file.
func LoadFromFile(path string) (*Config, error) {
	logConfig.Printf("Loading configuration from file: %s", path)

	// Open file for streaming
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Use streaming decoder for better memory efficiency
	var cfg Config
	decoder := toml.NewDecoder(file)
	md, err := decoder.Decode(&cfg)
	if err != nil {
		// Extract position information from ParseError for better error messages
		// Try pointer type first (for compatibility)
		if perr, ok := err.(*toml.ParseError); ok {
			return nil, fmt.Errorf("failed to parse TOML at line %d, column %d: %s",
				perr.Position.Line, perr.Position.Col, perr.Message)
		}
		// Try value type (used by toml.Decode)
		if perr, ok := err.(toml.ParseError); ok {
			return nil, fmt.Errorf("failed to parse TOML at line %d, column %d: %s",
				perr.Position.Line, perr.Position.Col, perr.Message)
		}
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	logConfig.Printf("Parsed TOML config with %d servers", len(cfg.Servers))

	// Detect and warn about unknown configuration keys (typos, deprecated options)
	undecoded := md.Undecoded()
	if len(undecoded) > 0 {
		for _, key := range undecoded {
			// Log to both debug logger and file logger for visibility
			logConfig.Printf("WARNING: Unknown configuration key '%s' - check for typos or deprecated options", key)
			logger.LogWarn("config", "Unknown configuration key '%s' - check for typos or deprecated options", key)
		}
	}

	// Validate required fields
	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("no servers defined in configuration")
	}

	// Initialize gateway if not present
	if cfg.Gateway == nil {
		cfg.Gateway = &GatewayConfig{}
	}

	// Apply core gateway defaults
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = DefaultPort
	}
	if cfg.Gateway.StartupTimeout == 0 {
		cfg.Gateway.StartupTimeout = DefaultStartupTimeout
	}
	if cfg.Gateway.ToolTimeout == 0 {
		cfg.Gateway.ToolTimeout = DefaultToolTimeout
	}

	// Apply feature-specific defaults
	applyDefaults(&cfg)

	logConfig.Printf("Successfully loaded %d servers from TOML file", len(cfg.Servers))
	return &cfg, nil
}

// logger for config package
var logConfig = log.New(io.Discard, "[CONFIG] ", log.LstdFlags)

// SetDebug enables debug logging for config package
func SetDebug(enabled bool) {
	if enabled {
		logConfig = log.New(os.Stderr, "[CONFIG] ", log.LstdFlags)
	} else {
		logConfig = log.New(io.Discard, "[CONFIG] ", log.LstdFlags)
	}
}
