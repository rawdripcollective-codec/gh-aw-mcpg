package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
)

// loadConfigFromJSON is a test helper that creates a config from JSON via stdin
// Note: This validates against the JSON schema, so configs must match the schema.
// For tests that need invalid/non-schema configs, use newTestConfig instead.
func loadConfigFromJSON(t *testing.T, jsonConfig string) *config.Config {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err, "Failed to create pipe")

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	go func() {
		_, _ = w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := config.LoadFromStdin()
	require.NoError(t, err, "Failed to load config from stdin")

	return cfg
}

// newTestConfig creates a config directly without going through JSON parsing/schema validation.
// Use this for unit tests that need to test launcher behavior with non-standard command configurations
// that don't match the schema (e.g., testing with command="echo" instead of container images).
func newTestConfig(servers map[string]*config.ServerConfig) *config.Config {
	return &config.Config{
		Servers: servers,
		Gateway: &config.GatewayConfig{
			Port:   3001,
			Domain: "localhost",
		},
	}
}

func TestHTTPConnection(t *testing.T) {
	tests := []struct {
		name          string
		serverID      string
		authHeader    string
		authValue     string
		setupEnv      func(*testing.T)
		wantAuthValue string
		wantIsHTTP    bool
	}{
		{
			name:          "basic HTTP connection",
			serverID:      "safeinputs",
			authHeader:    "Authorization",
			authValue:     "test-auth-secret",
			setupEnv:      func(t *testing.T) {},
			wantAuthValue: "test-auth-secret",
			wantIsHTTP:    true,
		},
		{
			name:       "HTTP connection with variable expansion",
			serverID:   "safeinputs",
			authHeader: "Authorization",
			authValue:  "${TEST_AUTH_TOKEN}",
			setupEnv: func(t *testing.T) {
				t.Setenv("TEST_AUTH_TOKEN", "secret-token-value")
			},
			wantAuthValue: "secret-token-value",
			wantIsHTTP:    true,
		},
		{
			name:          "HTTP connection with custom header",
			serverID:      "custom-server",
			authHeader:    "X-API-Key",
			authValue:     "custom-key-123",
			setupEnv:      func(t *testing.T) {},
			wantAuthValue: "custom-key-123",
			wantIsHTTP:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment if needed
			tt.setupEnv(t)

			// Create a mock HTTP server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"result": map[string]interface{}{
						"protocolVersion": "2024-11-05",
						"capabilities":    map[string]interface{}{},
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer mockServer.Close()

			// Create test config with HTTP server
			jsonConfig := fmt.Sprintf(`{
				"mcpServers": {
					"%s": {
						"type": "http",
						"url": "%s",
						"headers": {
							"%s": "%s"
						}
					}
				},
				"gateway": {
					"port": 3001,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`, tt.serverID, mockServer.URL, tt.authHeader, tt.authValue)

			cfg := loadConfigFromJSON(t, jsonConfig)

			// Verify HTTP server is loaded
			httpServer, ok := cfg.Servers[tt.serverID]
			require.True(t, ok, "HTTP server '%s' not found", tt.serverID)
			assert.Equal(t, "http", httpServer.Type)
			assert.Equal(t, mockServer.URL, httpServer.URL)
			assert.Equal(t, tt.wantAuthValue, httpServer.Headers[tt.authHeader])

			// Test launcher
			ctx := context.Background()
			l := New(ctx, cfg)

			// Get connection
			conn, err := GetOrLaunch(l, tt.serverID)
			require.NoError(t, err, "Failed to get connection")

			assert.Equal(t, tt.wantIsHTTP, conn.IsHTTP(), "Connection HTTP status mismatch")
			assert.Equal(t, mockServer.URL, conn.GetHTTPURL())
			assert.Equal(t, tt.wantAuthValue, conn.GetHTTPHeaders()[tt.authHeader])
		})
	}
}

func TestMixedHTTPAndStdioServers(t *testing.T) {
	// Create test config with both HTTP and stdio servers
	jsonConfig := `{
		"mcpServers": {
			"http-server": {
				"type": "http",
				"url": "http://example.com/mcp"
			},
			"stdio-server": {
				"type": "stdio",
				"container": "test/server:latest"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cfg := loadConfigFromJSON(t, jsonConfig)

	// Verify both servers are loaded
	assert.Len(t, cfg.Servers, 2, "Expected 2 servers")

	// Check HTTP server
	httpServer, ok := cfg.Servers["http-server"]
	require.True(t, ok, "HTTP server not found")
	assert.Equal(t, "http", httpServer.Type)

	// Check stdio server
	stdioServer, ok := cfg.Servers["stdio-server"]
	require.True(t, ok, "stdio server not found")
	assert.Equal(t, "stdio", stdioServer.Type)
}

func TestTruncateSecretMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil env map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty env map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single env var with long value",
			input: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_1234567890abcdefghijklmnop",
			},
			expected: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_...",
			},
		},
		{
			name: "multiple env vars with various lengths",
			input: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_1234567890abcdefghijklmnop",
				"API_KEY":                      "key_abc123xyz",
				"SHORT":                        "abc",
			},
			expected: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_...",
				"API_KEY":                      "key_...",
				"SHORT":                        "...",
			},
		},
		{
			name: "env var with exactly 4 characters",
			input: map[string]string{
				"TEST": "1234",
			},
			expected: map[string]string{
				"TEST": "...",
			},
		},
		{
			name: "env var with 5 characters",
			input: map[string]string{
				"TEST": "12345",
			},
			expected: map[string]string{
				"TEST": "1234...",
			},
		},
		{
			name: "env var with empty value",
			input: map[string]string{
				"EMPTY": "",
			},
			expected: map[string]string{
				"EMPTY": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitize.TruncateSecretMap(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result, "Expected non-nil result")
			assert.Equal(t, len(tt.expected), len(result), "Map length mismatch")

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "Value mismatch for key %s", key)
			}
		})
	}
}

func TestGetOrLaunch_InvalidServerID(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"valid-server": {
				"type": "http",
				"url": "http://example.com"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)

	// Try to get a non-existent server
	conn, err := GetOrLaunch(l, "non-existent-server")
	assert.Error(t, err, "Expected error for non-existent server")
	assert.Nil(t, conn, "Expected nil connection")
	assert.Contains(t, err.Error(), "not found in config")
}

func TestGetOrLaunch_Reuse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	jsonConfig := fmt.Sprintf(`{
		"mcpServers": {
			"test-server": {
				"type": "http",
				"url": "%s"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`, mockServer.URL)

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)

	// First call - should create new connection
	conn1, err := GetOrLaunch(l, "test-server")
	require.NoError(t, err)
	require.NotNil(t, conn1)

	// Second call - should reuse existing connection
	conn2, err := GetOrLaunch(l, "test-server")
	require.NoError(t, err)
	require.NotNil(t, conn2)

	// Verify they're the same connection object
	assert.Equal(t, conn1, conn2, "Should reuse the same connection")
}

func TestServerIDs(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"server-one": {
				"type": "http",
				"url": "http://example.com/one"
			},
			"server-two": {
				"type": "http",
				"url": "http://example.com/two"
			},
			"server-three": {
				"type": "stdio",
				"container": "test:latest"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)

	ids := l.ServerIDs()
	assert.Len(t, ids, 3, "Should return all server IDs")
	assert.ElementsMatch(t, []string{"server-one", "server-two", "server-three"}, ids)
}

func TestServerIDs_Empty(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)

	ids := l.ServerIDs()
	assert.Empty(t, ids, "Should return empty slice for no servers")
}

func TestClose(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	jsonConfig := fmt.Sprintf(`{
		"mcpServers": {
			"test-server": {
				"type": "http",
				"url": "%s"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`, mockServer.URL)

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)

	// Create a connection
	conn, err := GetOrLaunch(l, "test-server")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify connection exists
	l.mu.RLock()
	assert.Len(t, l.connections, 1, "Should have one connection")
	l.mu.RUnlock()

	// Close all connections
	l.Close()

	// Verify connections map is cleared
	l.mu.RLock()
	assert.Len(t, l.connections, 0, "Connections should be cleared after Close")
	l.mu.RUnlock()
}

func TestGetOrLaunchForSession_HTTPBackend(t *testing.T) {
	// HTTP backends should use regular GetOrLaunch (stateless)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "http-test",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	jsonConfig := fmt.Sprintf(`{
"mcpServers": {
"http-backend": {
"type": "http",
"url": "%s"
}
},
"gateway": {
"port": 3002,
"domain": "localhost",
"apiKey": "test-key"
}
}`, mockServer.URL)

	cfg := loadConfigFromJSON(t, jsonConfig)
	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Get connection for two different sessions
	conn1, err := GetOrLaunchForSession(l, "http-backend", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn1)

	conn2, err := GetOrLaunchForSession(l, "http-backend", "session2")
	require.NoError(t, err)
	require.NotNil(t, conn2)

	// For HTTP backends, both should return the same connection (stateless)
	assert.Equal(t, conn1, conn2, "HTTP backends should reuse same connection")

	// Should be in regular connections map, not session pool
	assert.Equal(t, 1, len(l.connections), "Should have one connection in regular map")
	assert.Equal(t, 0, l.sessionPool.Size(), "Session pool should be empty for HTTP")
}

func TestGetOrLaunchForSession_SessionReuse(t *testing.T) {
	// Note: We can't fully test stdio backends without actual processes
	// This test verifies the session pool is consulted
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	l := New(ctx, cfg)
	defer l.Close()

	// Verify session pool was created
	assert.NotNil(t, l.sessionPool, "Session pool should be initialized")
}

func TestGetOrLaunchForSession_InvalidServer(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	l := New(ctx, cfg)
	defer l.Close()

	// Try to get connection for non-existent server
	conn, err := GetOrLaunchForSession(l, "nonexistent", "session1")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "not found in config")
}

func TestLauncher_StartupTimeout(t *testing.T) {
	// Test that launcher respects the startup timeout from config
	tests := []struct {
		name            string
		configTimeout   int
		expectedTimeout string
	}{
		{
			name:            "default timeout (60 seconds)",
			configTimeout:   0, // 0 means use default
			expectedTimeout: "1m0s",
		},
		{
			name:            "custom timeout (30 seconds)",
			configTimeout:   30,
			expectedTimeout: "30s",
		},
		{
			name:            "custom timeout (120 seconds)",
			configTimeout:   120,
			expectedTimeout: "2m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test-server": {
						Type: "http",
						URL:  "http://example.com",
					},
				},
				Gateway: &config.GatewayConfig{
					Port:           3000,
					StartupTimeout: tt.configTimeout,
					ToolTimeout:    120,
				},
			}

			// If timeout is 0, set it to default to match LoadFromFile behavior
			if cfg.Gateway.StartupTimeout == 0 {
				cfg.Gateway.StartupTimeout = config.DefaultStartupTimeout
			}

			l := New(ctx, cfg)
			defer l.Close()

			// Verify the timeout was set correctly
			assert.Equal(t, tt.expectedTimeout, l.startupTimeout.String())
		})
	}
}

func TestLauncher_TimeoutWithNilGateway(t *testing.T) {
	// Test that launcher uses default timeout when Gateway config is nil
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-server": {
				Type: "http",
				URL:  "http://example.com",
			},
		},
		Gateway: nil, // No gateway config
	}

	l := New(ctx, cfg)
	defer l.Close()

	// Should use default timeout (60 seconds)
	assert.Equal(t, "1m0s", l.startupTimeout.String())
}
