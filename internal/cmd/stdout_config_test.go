package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/github/gh-aw-mcpg/internal/config"
)

func TestWriteGatewayConfigToStdout(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		listenAddr string
		mode       string
		wantHost   string
		wantPort   string
		wantAPIKey string
	}{
		{
			name: "routed mode with single server",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
						Args:    []string{"run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"},
					},
				},
				Gateway: &config.GatewayConfig{
					APIKey: "test-api-key",
				},
			},
			listenAddr: "127.0.0.1:8080",
			mode:       "routed",
			wantHost:   "127.0.0.1",
			wantPort:   "8080",
			wantAPIKey: "test-api-key",
		},
		{
			name: "unified mode with multiple servers",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
					},
					"fetch": {
						Command: "docker",
					},
				},
				Gateway: &config.GatewayConfig{
					APIKey: "unified-api-key",
				},
			},
			listenAddr: "0.0.0.0:3000",
			mode:       "unified",
			wantHost:   "0.0.0.0",
			wantPort:   "3000",
			wantAPIKey: "unified-api-key",
		},
		{
			name: "default port when address has no port",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: "echo",
					},
				},
			},
			listenAddr: "localhost",
			mode:       "routed",
			wantHost:   "127.0.0.1",
			wantPort:   "3000",
			wantAPIKey: "",
		},
		{
			name: "IPv6 address with port",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: "echo",
					},
				},
			},
			listenAddr: "[::1]:8080",
			mode:       "routed",
			wantHost:   "::1",
			wantPort:   "8080",
			wantAPIKey: "",
		},
		{
			name: "IPv6 address with full notation",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
					},
				},
				Gateway: &config.GatewayConfig{
					APIKey: "ipv6-key",
				},
			},
			listenAddr: "[2001:db8::1]:3000",
			mode:       "unified",
			wantHost:   "2001:db8::1",
			wantPort:   "3000",
			wantAPIKey: "ipv6-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture output
			var buf bytes.Buffer

			// Write configuration to buffer
			err := writeGatewayConfig(tt.cfg, tt.listenAddr, tt.mode, &buf)

			require.NoError(t, err, "writeGatewayConfig() error = ")
			output := buf.String()

			// Parse JSON output
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
			}

			// Verify structure
			mcpServers, ok := result["mcpServers"].(map[string]interface{})
			if !ok {
				t.Fatal("Output missing 'mcpServers' field or wrong type")
			}

			// Verify all servers are present
			if len(mcpServers) != len(tt.cfg.Servers) {
				t.Errorf("Expected %d servers, got %d", len(tt.cfg.Servers), len(mcpServers))
			}

			// Verify each server configuration
			for serverName := range tt.cfg.Servers {
				serverConfig, ok := mcpServers[serverName].(map[string]interface{})
				if !ok {
					t.Errorf("Server '%s' missing or wrong type", serverName)
					continue
				}

				// Verify type is "http"
				if serverType, ok := serverConfig["type"].(string); !ok || serverType != "http" {
					t.Errorf("Server '%s' type = %v, want 'http'", serverName, serverConfig["type"])
				}

				// Verify URL format
				url, ok := serverConfig["url"].(string)
				if !ok {
					t.Errorf("Server '%s' missing url or wrong type", serverName)
					continue
				}

				// Check URL contains expected components
				expectedPrefix := "http://" + tt.wantHost + ":" + tt.wantPort + "/mcp"
				if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
					t.Errorf("Server '%s' url = %v, want prefix %v", serverName, url, expectedPrefix)
				}

				// In routed mode, URL should include server name
				if tt.mode == "routed" {
					expectedURL := expectedPrefix + "/" + serverName
					if url != expectedURL {
						t.Errorf("Server '%s' url = %v, want %v", serverName, url, expectedURL)
					}
				} else {
					// In unified mode, URL should be just /mcp
					if url != expectedPrefix {
						t.Errorf("Server '%s' url = %v, want %v", serverName, url, expectedPrefix)
					}
				}

				// Verify headers per MCP Gateway Specification Section 5.4
				if tt.wantAPIKey != "" {
					headers, ok := serverConfig["headers"].(map[string]interface{})
					if !ok {
						t.Errorf("Server '%s' missing headers or wrong type", serverName)
						continue
					}

					authHeader, ok := headers["Authorization"].(string)
					if !ok {
						t.Errorf("Server '%s' missing Authorization header or wrong type", serverName)
						continue
					}

					if authHeader != tt.wantAPIKey {
						t.Errorf("Server '%s' Authorization header = %v, want %v", serverName, authHeader, tt.wantAPIKey)
					}
				} else {
					// If no API key, headers should not be present
					if headers, ok := serverConfig["headers"]; ok {
						t.Errorf("Server '%s' should not have headers when no API key is configured, got: %v", serverName, headers)
					}
				}
			}
		})
	}
}

func TestWriteGatewayConfigToStdout_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	// Create buffer to capture output
	var buf bytes.Buffer

	err := writeGatewayConfig(cfg, "127.0.0.1:8080", "routed", &buf)

	require.NoError(t, err, "writeGatewayConfig() error = ")

	// Parse output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	mcpServers := result["mcpServers"].(map[string]interface{})
	if len(mcpServers) != 0 {
		t.Errorf("Expected empty mcpServers, got %d servers", len(mcpServers))
	}
}

func TestWriteGatewayConfigToStdout_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test": {
				Command: "echo",
			},
		},
	}

	// Create buffer to capture output
	var buf bytes.Buffer

	err := writeGatewayConfig(cfg, "localhost:3000", "routed", &buf)

	require.NoError(t, err, "writeGatewayConfig() error = ")

	output := buf.String()

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify output is pretty-printed (contains newlines)
	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Error("Output should be pretty-printed with indentation")
	}
}

func TestWriteGatewayConfigToStdout_WithPipe(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {
				Command: "docker",
				Args:    []string{"run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"},
			},
		},
	}

	// Create a pipe (simulates writing to /dev/stdout in containerized environment)
	r, w, err := os.Pipe()
	require.NoError(t, err, "Failed to create pipe")
	defer r.Close()
	defer w.Close()

	// Write configuration to pipe in a goroutine
	errCh := make(chan error, 1)
	go func() {
		err := writeGatewayConfig(cfg, "127.0.0.1:3000", "unified", w)
		w.Close() // Close writer to signal EOF
		errCh <- err
	}()

	// Read from pipe
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}

	// Check for errors from write operation
	if err := <-errCh; err != nil {
		t.Fatalf("writeGatewayConfig() error = %v", err)
	}

	// Verify output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	// Verify structure
	mcpServers, ok := result["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("Output missing 'mcpServers' field or wrong type")
	}

	// Verify github server is present
	if _, ok := mcpServers["github"]; !ok {
		t.Error("Expected 'github' server in output")
	}
}
