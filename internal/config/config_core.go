// Package config provides configuration loading and parsing.
// This file defines the core configuration types that are stable and rarely change.
package config

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/BurntSushi/toml"
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

	// Guards holds guard configurations (optional, experimental)
	Guards map[string]*GuardConfig `toml:"guards" json:"guards,omitempty"`

	// Gateway holds global gateway settings
	Gateway *GatewayConfig `toml:"gateway" json:"gateway,omitempty"`

	// EnableDIFC enables Decentralized Information Flow Control
	EnableDIFC bool `toml:"enable_difc" json:"enable_difc,omitempty"`

	// DIFCFilter enables DIFC response filtering (removes content that violates agent labels)
	DIFCFilter bool `toml:"difc_filter" json:"difc_filter,omitempty"`

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

	// Guard is the guard ID to use for this server (references a guard in the guards section)
	Guard string `toml:"guard" json:"guard,omitempty"`
}

// LoadFromFile loads configuration from a TOML file.
func LoadFromFile(path string) (*Config, error) {
	logConfig.Printf("Loading configuration from file: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	logConfig.Printf("Read %d bytes from config file", len(data))

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	logConfig.Printf("Parsed TOML config with %d servers", len(cfg.Servers))

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
