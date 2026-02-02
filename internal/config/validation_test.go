package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandVariables(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		envVars   map[string]string
		expected  string
		shouldErr bool
	}{
		{
			name:     "simple variable",
			input:    "${TEST_VAR}",
			envVars:  map[string]string{"TEST_VAR": "value"},
			expected: "value",
		},
		{
			name:     "multiple variables",
			input:    "${VAR1}-${VAR2}",
			envVars:  map[string]string{"VAR1": "hello", "VAR2": "world"},
			expected: "hello-world",
		},
		{
			name:     "variable in middle",
			input:    "prefix-${VAR}-suffix",
			envVars:  map[string]string{"VAR": "middle"},
			expected: "prefix-middle-suffix",
		},
		{
			name:     "no variables",
			input:    "static-value",
			envVars:  map[string]string{},
			expected: "static-value",
		},
		{
			name:      "undefined variable",
			input:     "${UNDEFINED_VAR}",
			envVars:   map[string]string{},
			shouldErr: true,
		},
		{
			name:      "mixed defined and undefined",
			input:     "${DEFINED}-${UNDEFINED}",
			envVars:   map[string]string{"DEFINED": "value"},
			shouldErr: true,
		},
		{
			name:     "nested variables in path",
			input:    "/path/${VAR1}/subdir/${VAR2}",
			envVars:  map[string]string{"VAR1": "foo", "VAR2": "bar"},
			expected: "/path/foo/subdir/bar",
		},
		{
			name:     "empty variable value",
			input:    "prefix-${EMPTY_VAR}-suffix",
			envVars:  map[string]string{"EMPTY_VAR": ""},
			expected: "prefix--suffix",
		},
		{
			name:     "variable at start",
			input:    "${VAR}/path/to/file",
			envVars:  map[string]string{"VAR": "/root"},
			expected: "/root/path/to/file",
		},
		{
			name:     "variable at end",
			input:    "/path/to/${VAR}",
			envVars:  map[string]string{"VAR": "file.txt"},
			expected: "/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result, err := expandVariables(tt.input, "test.path")

			if tt.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExpandEnvVariables(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")
	t.Setenv("API_KEY", "secret")

	tests := []struct {
		name       string
		input      map[string]string
		serverName string
		expected   map[string]string
		shouldErr  bool
	}{
		{
			name: "expand single variable",
			input: map[string]string{
				"TOKEN": "${GITHUB_TOKEN}",
			},
			serverName: "test",
			expected: map[string]string{
				"TOKEN": "ghp_test123",
			},
		},
		{
			name: "expand multiple variables",
			input: map[string]string{
				"TOKEN":   "${GITHUB_TOKEN}",
				"API_KEY": "${API_KEY}",
			},
			serverName: "test",
			expected: map[string]string{
				"TOKEN":   "ghp_test123",
				"API_KEY": "secret",
			},
		},
		{
			name: "mixed literal and variable",
			input: map[string]string{
				"LITERAL": "static",
				"DYNAMIC": "${GITHUB_TOKEN}",
			},
			serverName: "test",
			expected: map[string]string{
				"LITERAL": "static",
				"DYNAMIC": "ghp_test123",
			},
		},
		{
			name: "undefined variable",
			input: map[string]string{
				"TOKEN": "${UNDEFINED_VAR}",
			},
			serverName: "test",
			shouldErr:  true,
		},
		{
			name:       "empty env map",
			input:      map[string]string{},
			serverName: "test",
			expected:   map[string]string{},
		},
		{
			name: "no variables to expand",
			input: map[string]string{
				"STATIC1": "value1",
				"STATIC2": "value2",
			},
			serverName: "test",
			expected: map[string]string{
				"STATIC1": "value1",
				"STATIC2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandEnvVariables(tt.input, tt.serverName)

			if tt.shouldErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.serverName, "Error should mention server name")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidateStdioServer(t *testing.T) {
	tests := []struct {
		name      string
		server    *StdinServerConfig
		shouldErr bool
		errorMsg  string
	}{
		{
			name: "valid with container",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "valid with entrypointArgs and container",
			server: &StdinServerConfig{
				Type:           "stdio",
				Container:      "test:latest",
				EntrypointArgs: []string{"--verbose"},
			},
			shouldErr: false,
		},
		{
			name: "valid with entrypoint and container",
			server: &StdinServerConfig{
				Type:       "stdio",
				Container:  "test:latest",
				Entrypoint: "/bin/bash",
			},
			shouldErr: false,
		},
		{
			name: "valid with mounts (ro)",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host/path:/container/path:ro"},
			},
			shouldErr: false,
		},
		{
			name: "valid with mounts (rw)",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host/data:/app/data:rw"},
			},
			shouldErr: false,
		},
		{
			name: "valid with multiple mounts",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts: []string{
					"/host/path1:/container/path1:ro",
					"/host/path2:/container/path2:rw",
				},
			},
			shouldErr: false,
		},
		{
			name: "valid with all new fields",
			server: &StdinServerConfig{
				Type:           "stdio",
				Container:      "test:latest",
				Entrypoint:     "/custom/entrypoint.sh",
				EntrypointArgs: []string{"--verbose", "--debug"},
				Mounts:         []string{"/host:/container:ro"},
			},
			shouldErr: false,
		},
		{
			name: "missing container",
			server: &StdinServerConfig{
				Type: "stdio",
			},
			shouldErr: true,
			errorMsg:  "'container' is required for stdio servers",
		},
		{
			name: "command field not supported",
			server: &StdinServerConfig{
				Type:      "stdio",
				Command:   "node",
				Container: "test:latest",
			},
			shouldErr: true,
			errorMsg:  "'command' field is not supported",
		},
		{
			name: "command without container",
			server: &StdinServerConfig{
				Type:    "stdio",
				Command: "node",
			},
			shouldErr: true,
			errorMsg:  "'container' is required for stdio servers",
		},
		{
			name: "http server without url",
			server: &StdinServerConfig{
				Type: "http",
			},
			shouldErr: true,
			errorMsg:  "'url' is required for HTTP servers",
		},
		{
			name: "http server with url",
			server: &StdinServerConfig{
				Type: "http",
				URL:  "https://example.com/mcp",
			},
			shouldErr: false,
		},
		{
			name: "empty type defaults to stdio with container",
			server: &StdinServerConfig{
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "local type normalizes to stdio with container",
			server: &StdinServerConfig{
				Type:      "local",
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "valid mount without mode",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container"},
			},
			shouldErr: false,
		},
		{
			name: "invalid mount format - too many parts",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container:ro:extra"},
			},
			shouldErr: true,
			errorMsg:  "invalid mount format",
		},
		{
			name: "invalid mount mode",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container:invalid"},
			},
			shouldErr: true,
			errorMsg:  "invalid mount mode",
		},
		{
			name: "mount with empty source",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{":/container:ro"},
			},
			shouldErr: true,
			errorMsg:  "mount source cannot be empty",
		},
		{
			name: "mount with empty destination",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host::ro"},
			},
			shouldErr: true,
			errorMsg:  "mount destination cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerConfig("test-server", tt.server)

			if tt.shouldErr {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.ErrorContains(t, err, tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGatewayConfig(t *testing.T) {
	tests := []struct {
		name      string
		gateway   *StdinGatewayConfig
		shouldErr bool
		errorMsg  string
	}{
		{
			name:      "nil gateway",
			gateway:   nil,
			shouldErr: false,
		},
		{
			name: "valid gateway",
			gateway: &StdinGatewayConfig{
				Port:           intPtr(8080),
				Domain:         "example.com",
				StartupTimeout: intPtr(30),
				ToolTimeout:    intPtr(60),
			},
			shouldErr: false,
		},
		{
			name: "valid gateway with absolute Unix payloadDir",
			gateway: &StdinGatewayConfig{
				Port:       intPtr(8080),
				Domain:     "example.com",
				PayloadDir: "/tmp/jq-payloads",
			},
			shouldErr: false,
		},
		{
			name: "valid gateway with absolute Windows payloadDir",
			gateway: &StdinGatewayConfig{
				Port:       intPtr(8080),
				Domain:     "example.com",
				PayloadDir: "C:\\payloads",
			},
			shouldErr: false,
		},
		{
			name: "invalid gateway with relative payloadDir",
			gateway: &StdinGatewayConfig{
				Port:       intPtr(8080),
				Domain:     "example.com",
				PayloadDir: "tmp/payloads",
			},
			shouldErr: true,
			errorMsg:  "must be an absolute path",
		},
		{
			name: "invalid gateway with dot-relative payloadDir",
			gateway: &StdinGatewayConfig{
				Port:       intPtr(8080),
				Domain:     "example.com",
				PayloadDir: "./payloads",
			},
			shouldErr: true,
			errorMsg:  "must be an absolute path",
		},
		{
			name: "port too low",
			gateway: &StdinGatewayConfig{
				Port: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "port too high",
			gateway: &StdinGatewayConfig{
				Port: intPtr(70000),
			},
			shouldErr: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "negative startupTimeout",
			gateway: &StdinGatewayConfig{
				StartupTimeout: intPtr(-1),
			},
			shouldErr: true,
			errorMsg:  "startupTimeout must be at least 1",
		},
		{
			name: "zero startupTimeout",
			gateway: &StdinGatewayConfig{
				StartupTimeout: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "startupTimeout must be at least 1",
		},
		{
			name: "negative toolTimeout",
			gateway: &StdinGatewayConfig{
				ToolTimeout: intPtr(-1),
			},
			shouldErr: true,
			errorMsg:  "toolTimeout must be at least 1",
		},
		{
			name: "zero toolTimeout",
			gateway: &StdinGatewayConfig{
				ToolTimeout: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "toolTimeout must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGatewayConfig(tt.gateway)

			if tt.shouldErr {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.ErrorContains(t, err, tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// setupStdinTest is a helper that sets up stdin with the given JSON config
// Returns a cleanup function that should be deferred
func setupStdinTest(t *testing.T, jsonConfig string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err, "Failed to create pipe")

	oldStdin := os.Stdin
	os.Stdin = r

	go func() {
		defer w.Close()
		_, err := w.Write([]byte(jsonConfig))
		if err != nil {
			t.Logf("Failed to write to pipe: %v", err)
		}
	}()

	return func() {
		os.Stdin = oldStdin
		r.Close()
	}
}

func TestLoadFromStdin_WithVariableExpansion(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_expanded")

	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"container": "ghcr.io/github/github-mcp-server:latest",
				"env": {
					"TOKEN": "${GITHUB_TOKEN}",
					"LITERAL": "static-value"
				}
			}
		},
		"gateway": {
			"port": 8080,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cleanup := setupStdinTest(t, jsonConfig)
	defer cleanup()

	cfg, err := LoadFromStdin()
	require.NoError(t, err)

	server := cfg.Servers["github"]
	assert.Equal(t, "docker", server.Command, "Expected Command to be 'docker'")
}

func TestLoadFromStdin_UndefinedVariable(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"container": "ghcr.io/github/github-mcp-server:latest",
				"env": {
					"TOKEN": "${UNDEFINED_GITHUB_TOKEN}"
				}
			}
		},
		"gateway": {
			"port": 8080,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cleanup := setupStdinTest(t, jsonConfig)
	defer cleanup()

	_, err := LoadFromStdin()
	require.Error(t, err)
	assert.ErrorContains(t, err, "UNDEFINED_GITHUB_TOKEN", "Error should mention the undefined variable")
	assert.ErrorContains(t, err, "undefined environment variable", "Error should describe the issue")
}

func TestLoadFromStdin_VariableExpansionInContainer(t *testing.T) {
	t.Setenv("REGISTRY", "ghcr.io")
	t.Setenv("IMAGE_NAME", "github/github-mcp-server")

	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"container": "${REGISTRY}/${IMAGE_NAME}:latest",
				"env": {
					"TOKEN": "static-value"
				}
			}
		},
		"gateway": {
			"port": 8080,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	cleanup := setupStdinTest(t, jsonConfig)
	defer cleanup()

	cfg, err := LoadFromStdin()
	require.NoError(t, err)

	server := cfg.Servers["github"]
	// Container field should have variables expanded in docker args
	assert.Contains(t, server.Args, "ghcr.io/github/github-mcp-server:latest")
}

func TestLoadFromStdin_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errorMsg  string
	}{
		{
			name: "missing container",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio"
					}
				},
				"gateway": {
					"port": 8080,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
		{
			name: "command field not supported",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"command": "node",
						"container": "test:latest"
					}
				},
				"gateway": {
					"port": 8080,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
		{
			name: "invalid gateway port",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"container": "test:latest"
					}
				},
				"gateway": {
					"port": 99999,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
		{
			name: "malformed JSON",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"container": "test:latest"
					}
				// missing closing brace`,
			shouldErr: true,
		},
		{
			name: "empty mcpServers",
			config: `{
				"mcpServers": {},
				"gateway": {
					"port": 8080,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupStdinTest(t, tt.config)
			defer cleanup()

			_, err := LoadFromStdin()

			if tt.shouldErr {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.ErrorContains(t, err, tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function - defined in validation_string_patterns_test.go
