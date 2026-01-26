package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// intPtr is a helper function to create integer pointers for test data
func intPtr(i int) *int {
	return &i
}

func TestValidateStringPatternsComprehensive(t *testing.T) {
	t.Run("stdio server container validation", func(t *testing.T) {
		tests := []struct {
			name        string
			container   string
			serverType  string
			shouldError bool
			errorField  string
		}{
			// Valid container patterns
			{
				name:        "valid container with full path and tag",
				container:   "ghcr.io/github/github-mcp-server:latest",
				serverType:  "stdio",
				shouldError: false,
			},
			{
				name:        "valid container with owner/image format",
				container:   "owner/image:latest",
				serverType:  "stdio",
				shouldError: false,
			},
			{
				name:        "valid container without tag",
				container:   "ghcr.io/owner/image",
				serverType:  "stdio",
				shouldError: false,
			},
			{
				name:        "valid container with version tag",
				container:   "nginx:1.21.0",
				serverType:  "stdio",
				shouldError: false,
			},
			{
				name:        "valid container simple name",
				container:   "redis",
				serverType:  "stdio",
				shouldError: false,
			},
			{
				name:        "valid container with local type",
				container:   "ghcr.io/test/image:v1",
				serverType:  "local",
				shouldError: false,
			},
			{
				name:        "valid container with empty type defaults to stdio",
				container:   "myimage:latest",
				serverType:  "",
				shouldError: false,
			},
			// Invalid container patterns
			{
				name:        "invalid container starts with special char",
				container:   "-invalid/image:latest",
				serverType:  "stdio",
				shouldError: true,
				errorField:  "container",
			},
			{
				name:        "invalid container with spaces",
				container:   "owner/image name:latest",
				serverType:  "stdio",
				shouldError: true,
				errorField:  "container",
			},
			{
				name:        "invalid container with double colon",
				container:   "owner/image::tag",
				serverType:  "stdio",
				shouldError: true,
				errorField:  "container",
			},
			{
				name:        "invalid container empty string accepted (empty is valid)",
				container:   "",
				serverType:  "stdio",
				shouldError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:      tt.serverType,
							Container: tt.container,
						},
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), tt.errorField, "Error should mention the problematic field")
				} else {
					assert.NoError(t, err, "Expected no error for valid container pattern")
				}
			})
		}
	})

	t.Run("stdio server mount validation", func(t *testing.T) {
		tests := []struct {
			name        string
			mounts      []string
			shouldError bool
			errorField  string
		}{
			// Valid mount patterns
			{
				name:        "valid mount with ro mode",
				mounts:      []string{"/host/path:/container/path:ro"},
				shouldError: false,
			},
			{
				name:        "valid mount with rw mode",
				mounts:      []string{"/host/path:/container/path:rw"},
				shouldError: false,
			},
			{
				name:        "valid mount without mode",
				mounts:      []string{"/host/path:/container/path"},
				shouldError: false,
			},
			{
				name:        "valid multiple mounts",
				mounts:      []string{"/host1:/container1:ro", "/host2:/container2:rw"},
				shouldError: false,
			},
			// Invalid mount patterns
			{
				name:        "invalid mount missing destination",
				mounts:      []string{"/host/path"},
				shouldError: true,
				errorField:  "mounts",
			},
			{
				name:        "invalid mount with wrong mode",
				mounts:      []string{"/host:/container:invalid"},
				shouldError: true,
				errorField:  "mounts",
			},
			{
				name:        "invalid mount empty string",
				mounts:      []string{""},
				shouldError: true,
				errorField:  "mounts",
			},
			{
				name:        "invalid mount in array with valid ones",
				mounts:      []string{"/host1:/container1:ro", "invalid"},
				shouldError: true,
				errorField:  "mounts",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:      "stdio",
							Container: "test/image:latest",
							Mounts:    tt.mounts,
						},
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), tt.errorField, "Error should mention the problematic field")
				} else {
					assert.NoError(t, err, "Expected no error for valid mount pattern")
				}
			})
		}
	})

	t.Run("stdio server entrypoint validation", func(t *testing.T) {
		tests := []struct {
			name        string
			entrypoint  string
			shouldError bool
		}{
			{
				name:        "valid entrypoint path",
				entrypoint:  "/bin/bash",
				shouldError: false,
			},
			{
				name:        "valid entrypoint with spaces",
				entrypoint:  "/usr/bin/my script",
				shouldError: false,
			},
			{
				name:        "valid empty entrypoint",
				entrypoint:  "",
				shouldError: false,
			},
			{
				name:        "invalid entrypoint whitespace only",
				entrypoint:  "   ",
				shouldError: true,
			},
			{
				name:        "invalid entrypoint single space",
				entrypoint:  " ",
				shouldError: true,
			},
			{
				name:        "invalid entrypoint tabs only",
				entrypoint:  "\t\t",
				shouldError: true,
			},
			{
				name:        "invalid entrypoint newline only",
				entrypoint:  "\n",
				shouldError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:       "stdio",
							Container:  "test/image:latest",
							Entrypoint: tt.entrypoint,
						},
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), "entrypoint", "Error should mention entrypoint")
				} else {
					assert.NoError(t, err, "Expected no error for valid entrypoint")
				}
			})
		}
	})

	t.Run("http server url validation", func(t *testing.T) {
		tests := []struct {
			name        string
			url         string
			shouldError bool
		}{
			// Valid URLs
			{
				name:        "valid https url",
				url:         "https://api.example.com/mcp",
				shouldError: false,
			},
			{
				name:        "valid http url",
				url:         "http://localhost:8080/mcp",
				shouldError: false,
			},
			{
				name:        "valid https with port",
				url:         "https://api.example.com:443/path",
				shouldError: false,
			},
			{
				name:        "valid http with query params",
				url:         "http://example.com/api?key=value",
				shouldError: false,
			},
			{
				name:        "valid empty url",
				url:         "",
				shouldError: false,
			},
			// Invalid URLs
			{
				name:        "invalid url without protocol",
				url:         "api.example.com/mcp",
				shouldError: true,
			},
			{
				name:        "invalid url with ftp protocol",
				url:         "ftp://example.com/file",
				shouldError: true,
			},
			{
				name:        "invalid url with ws protocol",
				url:         "ws://example.com/socket",
				shouldError: true,
			},
			{
				name:        "invalid url just protocol",
				url:         "https://",
				shouldError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type: "http",
							URL:  tt.url,
						},
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), "url", "Error should mention url")
				} else {
					assert.NoError(t, err, "Expected no error for valid URL")
				}
			})
		}
	})

	t.Run("gateway port validation", func(t *testing.T) {
		tests := []struct {
			name        string
			port        *int
			shouldError bool
		}{
			{
				name:        "valid port 8080",
				port:        intPtr(8080),
				shouldError: false,
			},
			{
				name:        "valid port 80",
				port:        intPtr(80),
				shouldError: false,
			},
			{
				name:        "valid port 443",
				port:        intPtr(443),
				shouldError: false,
			},
			{
				name:        "valid port 1 (minimum)",
				port:        intPtr(1),
				shouldError: false,
			},
			{
				name:        "valid port 65535 (maximum)",
				port:        intPtr(65535),
				shouldError: false,
			},
			{
				name:        "valid nil port",
				port:        nil,
				shouldError: false,
			},
			{
				name:        "invalid port 0",
				port:        intPtr(0),
				shouldError: true,
			},
			{
				name:        "invalid port -1",
				port:        intPtr(-1),
				shouldError: true,
			},
			{
				name:        "invalid port 65536 (above maximum)",
				port:        intPtr(65536),
				shouldError: true,
			},
			{
				name:        "invalid port 100000",
				port:        intPtr(100000),
				shouldError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:      "stdio",
							Container: "test/image:latest",
						},
					},
					Gateway: &StdinGatewayConfig{
						Port: tt.port,
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), "port", "Error should mention port")
				} else {
					assert.NoError(t, err, "Expected no error for valid port")
				}
			})
		}
	})

	t.Run("gateway domain validation", func(t *testing.T) {
		tests := []struct {
			name        string
			domain      string
			shouldError bool
		}{
			// Valid domains
			{
				name:        "valid domain localhost",
				domain:      "localhost",
				shouldError: false,
			},
			{
				name:        "valid domain host.docker.internal",
				domain:      "host.docker.internal",
				shouldError: false,
			},
			{
				name:        "valid domain variable simple",
				domain:      "${MCP_GATEWAY_DOMAIN}",
				shouldError: false,
			},
			{
				name:        "valid domain variable with underscores",
				domain:      "${MY_CUSTOM_DOMAIN}",
				shouldError: false,
			},
			{
				name:        "valid domain variable with numbers",
				domain:      "${DOMAIN123}",
				shouldError: false,
			},
			{
				name:        "valid empty domain",
				domain:      "",
				shouldError: false,
			},
			// Invalid domains
			{
				name:        "invalid domain regular hostname",
				domain:      "example.com",
				shouldError: true,
			},
			{
				name:        "invalid domain IP address",
				domain:      "127.0.0.1",
				shouldError: true,
			},
			{
				name:        "invalid domain with port",
				domain:      "localhost:8080",
				shouldError: true,
			},
			{
				name:        "invalid domain variable lowercase",
				domain:      "${domain}",
				shouldError: true,
			},
			{
				name:        "invalid domain variable starts with number",
				domain:      "${1DOMAIN}",
				shouldError: true,
			},
			{
				name:        "invalid domain variable missing braces",
				domain:      "$DOMAIN",
				shouldError: true,
			},
			{
				name:        "invalid domain variable with hyphen",
				domain:      "${MY-DOMAIN}",
				shouldError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:      "stdio",
							Container: "test/image:latest",
						},
					},
					Gateway: &StdinGatewayConfig{
						Domain: tt.domain,
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), "domain", "Error should mention domain")
				} else {
					assert.NoError(t, err, "Expected no error for valid domain")
				}
			})
		}
	})

	t.Run("gateway timeout validation", func(t *testing.T) {
		tests := []struct {
			name           string
			startupTimeout *int
			toolTimeout    *int
			shouldError    bool
			errorField     string
		}{
			// Valid timeouts
			{
				name:           "valid startup timeout",
				startupTimeout: intPtr(30),
				shouldError:    false,
			},
			{
				name:        "valid tool timeout",
				toolTimeout: intPtr(60),
				shouldError: false,
			},
			{
				name:           "valid both timeouts",
				startupTimeout: intPtr(30),
				toolTimeout:    intPtr(60),
				shouldError:    false,
			},
			{
				name:           "valid minimum timeout 1",
				startupTimeout: intPtr(1),
				toolTimeout:    intPtr(1),
				shouldError:    false,
			},
			{
				name:           "valid large timeout",
				startupTimeout: intPtr(3600),
				toolTimeout:    intPtr(7200),
				shouldError:    false,
			},
			{
				name:        "valid nil timeouts",
				shouldError: false,
			},
			// Invalid timeouts
			{
				name:           "invalid startup timeout zero",
				startupTimeout: intPtr(0),
				shouldError:    true,
				errorField:     "startupTimeout",
			},
			{
				name:           "invalid startup timeout negative",
				startupTimeout: intPtr(-5),
				shouldError:    true,
				errorField:     "startupTimeout",
			},
			{
				name:        "invalid tool timeout zero",
				toolTimeout: intPtr(0),
				shouldError: true,
				errorField:  "toolTimeout",
			},
			{
				name:        "invalid tool timeout negative",
				toolTimeout: intPtr(-10),
				shouldError: true,
				errorField:  "toolTimeout",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &StdinConfig{
					MCPServers: map[string]*StdinServerConfig{
						"test-server": {
							Type:      "stdio",
							Container: "test/image:latest",
						},
					},
					Gateway: &StdinGatewayConfig{
						StartupTimeout: tt.startupTimeout,
						ToolTimeout:    tt.toolTimeout,
					},
				}

				err := validateStringPatterns(config)

				if tt.shouldError {
					require.Error(t, err, "Expected validation error but got none")
					assert.Contains(t, err.Error(), tt.errorField, "Error should mention the problematic field")
				} else {
					assert.NoError(t, err, "Expected no error for valid timeouts")
				}
			})
		}
	})

	t.Run("multiple servers validation", func(t *testing.T) {
		t.Run("multiple valid servers", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"github": {
						Type:      "stdio",
						Container: "ghcr.io/github/github-mcp-server:latest",
					},
					"api": {
						Type: "http",
						URL:  "https://api.example.com/mcp",
					},
					"local": {
						Type:      "local",
						Container: "myimage:v1",
						Mounts:    []string{"/host:/container:ro"},
					},
				},
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "Expected no error for multiple valid servers")
		})

		t.Run("multiple servers with one invalid", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"valid-server": {
						Type:      "stdio",
						Container: "ghcr.io/github/github-mcp-server:latest",
					},
					"invalid-server": {
						Type:      "stdio",
						Container: "-invalid/image:latest", // Invalid container
					},
				},
			}

			err := validateStringPatterns(config)
			require.Error(t, err, "Expected validation error for invalid server")
			assert.Contains(t, err.Error(), "container", "Error should mention container issue")
		})
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Run("empty mcpServers map", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{},
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "Empty servers map should be valid")
		})

		t.Run("nil gateway config", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"test": {
						Type:      "stdio",
						Container: "test/image:latest",
					},
				},
				Gateway: nil,
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "Nil gateway should be valid")
		})

		t.Run("server with all fields empty", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"test": {},
				},
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "Empty server config should pass pattern validation")
		})

		t.Run("http server without container field", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"http-server": {
						Type: "http",
						URL:  "https://example.com/mcp",
						// No Container field - should be valid for HTTP servers
					},
				},
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "HTTP server without container should be valid")
		})

		t.Run("stdio server with container and mounts", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"complex-server": {
						Type:       "stdio",
						Container:  "ghcr.io/test/image:v2.0.1",
						Entrypoint: "/usr/bin/python3",
						Mounts: []string{
							"/host/data:/container/data:ro",
							"/host/config:/container/config:rw",
						},
					},
				},
				Gateway: &StdinGatewayConfig{
					Port:           intPtr(8080),
					Domain:         "localhost",
					StartupTimeout: intPtr(30),
					ToolTimeout:    intPtr(120),
				},
			}

			err := validateStringPatterns(config)
			assert.NoError(t, err, "Complex valid configuration should pass")
		})
	})

	t.Run("comprehensive error messages", func(t *testing.T) {
		t.Run("container error includes json path", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"my-server": {
						Type:      "stdio",
						Container: "-invalid",
					},
				},
			}

			err := validateStringPatterns(config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "mcpServers.my-server.container", "Error should include JSON path")
			assert.Contains(t, err.Error(), "Suggestion", "Error should include suggestion")
		})

		t.Run("mount error includes array index", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"test-server": {
						Type:      "stdio",
						Container: "test/image:latest",
						Mounts:    []string{"/valid:/mount:ro", "invalid"},
					},
				},
			}

			err := validateStringPatterns(config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "mounts[1]", "Error should include array index")
		})

		t.Run("domain error includes suggestion", func(t *testing.T) {
			config := &StdinConfig{
				MCPServers: map[string]*StdinServerConfig{
					"test": {
						Type:      "stdio",
						Container: "test/image:latest",
					},
				},
				Gateway: &StdinGatewayConfig{
					Domain: "example.com",
				},
			}

			err := validateStringPatterns(config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Suggestion", "Error should include suggestion")
			assert.Contains(t, err.Error(), "localhost", "Suggestion should mention localhost")
		})
	})
}
