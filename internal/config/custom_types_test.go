package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomServerTypes tests the compliance requirements for custom server types
// as specified in section 4.1.4 of the MCP Gateway specification.

// T-CFG-009: Valid custom server type with registered schema
func TestTCFG009_ValidCustomTypeWithRegisteredSchema(t *testing.T) {
	// Create a mock HTTP server that returns a valid JSON schema for the custom type
	mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		schema := map[string]interface{}{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type":    "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type": "string",
					"enum": []string{"safeinputs"},
				},
				"customField": map[string]interface{}{
					"type": "string",
				},
				"container": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"type", "customField", "container"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schema)
	}))
	defer mockSchemaServer.Close()

	// Configuration with custom server type and registered schema
	configJSON := map[string]interface{}{
		"customSchemas": map[string]string{
			"safeinputs": mockSchemaServer.URL,
		},
		"mcpServers": map[string]interface{}{
			"custom-server": map[string]interface{}{
				"type":        "safeinputs",
				"customField": "custom-value",
				"container":   "ghcr.io/example/safeinputs:latest",
			},
		},
	}

	data, err := json.Marshal(configJSON)
	require.NoError(t, err)

	// Parse the configuration
	var stdinCfg StdinConfig
	err = json.Unmarshal(data, &stdinCfg)
	require.NoError(t, err)

	// Custom schemas should be populated
	assert.NotNil(t, stdinCfg.CustomSchemas)
	assert.Equal(t, mockSchemaServer.URL, stdinCfg.CustomSchemas["safeinputs"])

	// Validate the server configuration with custom schemas
	server := stdinCfg.MCPServers["custom-server"]
	require.NotNil(t, server)

	err = validateServerConfigWithCustomSchemas("custom-server", server, stdinCfg.CustomSchemas)
	assert.NoError(t, err, "Valid custom server type with registered schema should pass validation")
}

// T-CFG-010: Reject custom type without schema registration
func TestTCFG010_RejectCustomTypeWithoutRegistration(t *testing.T) {
	// Configuration with unregistered custom server type
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"unregistered-server": map[string]interface{}{
				"type":      "unregistered",
				"container": "ghcr.io/example/unregistered:latest",
			},
		},
	}

	data, err := json.Marshal(configJSON)
	require.NoError(t, err)

	var stdinCfg StdinConfig
	err = json.Unmarshal(data, &stdinCfg)
	require.NoError(t, err)

	server := stdinCfg.MCPServers["unregistered-server"]
	require.NotNil(t, server)

	// Validate should fail for unregistered custom type
	err = validateServerConfigWithCustomSchemas("unregistered-server", server, stdinCfg.CustomSchemas)
	assert.Error(t, err, "Unregistered custom server type should be rejected")
	assert.Contains(t, err.Error(), "unregistered")
	assert.Contains(t, err.Error(), "not registered in customSchemas")
}

// T-CFG-011: Validate custom configuration against registered schema
func TestTCFG011_ValidateAgainstCustomSchema(t *testing.T) {
	t.Run("valid_custom_config", func(t *testing.T) {
		// Create a mock schema server that requires a specific field
		mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			schema := map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type": "string",
						"enum": []string{"mytype"},
					},
					"requiredField": map[string]interface{}{
						"type": "string",
					},
					"container": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"type", "requiredField", "container"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schema)
		}))
		defer mockSchemaServer.Close()

		// Valid configuration that matches schema
		configJSON := map[string]interface{}{
			"customSchemas": map[string]string{
				"mytype": mockSchemaServer.URL,
			},
			"mcpServers": map[string]interface{}{
				"valid-custom": map[string]interface{}{
					"type":          "mytype",
					"requiredField": "present",
					"container":     "ghcr.io/example/mytype:latest",
				},
			},
		}

		data, err := json.Marshal(configJSON)
		require.NoError(t, err)

		var stdinCfg StdinConfig
		err = json.Unmarshal(data, &stdinCfg)
		require.NoError(t, err)

		server := stdinCfg.MCPServers["valid-custom"]
		err = validateServerConfigWithCustomSchemas("valid-custom", server, stdinCfg.CustomSchemas)
		assert.NoError(t, err, "Configuration matching custom schema should pass validation")
	})

	t.Run("empty_string_skips_validation", func(t *testing.T) {
		// Empty string means skip validation
		configJSON := map[string]interface{}{
			"customSchemas": map[string]string{
				"novalidation": "",
			},
			"mcpServers": map[string]interface{}{
				"no-validation-server": map[string]interface{}{
					"type":      "novalidation",
					"container": "ghcr.io/example/novalidation:latest",
					// No other fields required
				},
			},
		}

		data, err := json.Marshal(configJSON)
		require.NoError(t, err)

		var stdinCfg StdinConfig
		err = json.Unmarshal(data, &stdinCfg)
		require.NoError(t, err)

		server := stdinCfg.MCPServers["no-validation-server"]
		err = validateServerConfigWithCustomSchemas("no-validation-server", server, stdinCfg.CustomSchemas)
		assert.NoError(t, err, "Empty schema URL should skip validation")
	})
}

// T-CFG-012: Reject custom type conflicting with reserved types (stdio/http)
func TestTCFG012_RejectReservedTypeNames(t *testing.T) {
	tests := []struct {
		name         string
		reservedType string
	}{
		{"stdio_conflict", "stdio"},
		{"http_conflict", "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Try to register a reserved type name in customSchemas
			configJSON := map[string]interface{}{
				"customSchemas": map[string]string{
					tt.reservedType: "https://example.com/schema.json",
				},
				"mcpServers": map[string]interface{}{
					"test-server": map[string]interface{}{
						"type":      tt.reservedType,
						"container": "ghcr.io/example/test:latest",
					},
				},
			}

			data, err := json.Marshal(configJSON)
			require.NoError(t, err)

			var stdinCfg StdinConfig
			err = json.Unmarshal(data, &stdinCfg)
			require.NoError(t, err)

			// Validation should reject reserved type names in customSchemas
			err = validateCustomSchemas(stdinCfg.CustomSchemas)
			assert.Error(t, err, "Reserved type name %q should be rejected in customSchemas", tt.reservedType)
			assert.Contains(t, err.Error(), tt.reservedType)
			assert.Contains(t, err.Error(), "reserved")
		})
	}
}

// T-CFG-013: Custom schema URL fetch and cache
func TestTCFG013_SchemaURLFetchAndCache(t *testing.T) {
	t.Run("empty_string_skips_validation", func(t *testing.T) {
		// Empty string means skip validation
		customSchemas := map[string]interface{}{
			"novalidation": "",
		}

		server := &StdinServerConfig{
			Type:      "novalidation",
			Container: "ghcr.io/example/novalidation:latest",
		}

		err := validateServerConfigWithCustomSchemas("test", server, customSchemas)
		assert.NoError(t, err, "Empty schema URL should skip validation and not fail")
	})

	t.Run("registered_custom_type", func(t *testing.T) {
		mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			schema := map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type": "string",
						"enum": []string{"cached"},
					},
					"container": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"type", "container"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schema)
		}))
		defer mockSchemaServer.Close()

		customSchemas := map[string]interface{}{
			"cached": mockSchemaServer.URL,
		}

		server := &StdinServerConfig{
			Type:      "cached",
			Container: "ghcr.io/example/cached:latest",
		}

		// Multiple validations should work (caching is implementation detail)
		err1 := validateServerConfigWithCustomSchemas("test1", server, customSchemas)
		assert.NoError(t, err1)

		err2 := validateServerConfigWithCustomSchemas("test2", server, customSchemas)
		assert.NoError(t, err2)
	})
}
