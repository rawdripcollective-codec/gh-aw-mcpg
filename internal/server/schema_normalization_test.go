package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/gh-aw-mcpg/internal/config"
	"github.com/github/gh-aw-mcpg/internal/mcp"
)

// TestSchemaNormalization_Integration tests that broken schemas from backends
// are automatically normalized when tools are registered.
func TestSchemaNormalization_Integration(t *testing.T) {
	ctx := context.Background()

	// Create a minimal config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test": {
				Command: "echo", // Dummy command
				Args:    []string{},
			},
		},
	}

	// Create unified server
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	testCases := []struct {
		name           string
		toolName       string
		inputSchema    map[string]interface{}
		expectedSchema map[string]interface{}
	}{
		{
			name:     "broken object schema without properties",
			toolName: "get_commit",
			inputSchema: map[string]interface{}{
				"type": "object",
			},
			expectedSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			name:     "valid object schema with properties",
			toolName: "issue_read",
			inputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type": "string",
					},
					"repo": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"owner", "repo"},
			},
			expectedSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type": "string",
					},
					"repo": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"owner", "repo"},
			},
		},
		{
			name:     "object schema with additionalProperties",
			toolName: "list_items",
			inputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			},
			expectedSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate registering a tool with the given schema
			prefixedName := "test___" + tc.toolName

			// Use the NormalizeInputSchema function directly
			normalized := mcp.NormalizeInputSchema(tc.inputSchema, prefixedName)

			// Store the normalized tool
			us.toolsMu.Lock()
			us.tools[prefixedName] = &ToolInfo{
				Name:        prefixedName,
				Description: "Test tool",
				BackendID:   "test",
				InputSchema: normalized,
			}
			us.toolsMu.Unlock()

			// Retrieve the tool and verify the schema
			us.toolsMu.RLock()
			tool, exists := us.tools[prefixedName]
			us.toolsMu.RUnlock()

			require.True(t, exists, "Tool should exist")
			assert.Equal(t, tc.expectedSchema, tool.InputSchema, "Schema should match expected normalized version")

			// Clean up
			us.toolsMu.Lock()
			delete(us.tools, prefixedName)
			us.toolsMu.Unlock()
		})
	}
}

// TestSchemaNormalization_PreservesOriginal verifies that the normalization
// doesn't modify the original schema object
func TestSchemaNormalization_PreservesOriginal(t *testing.T) {
	original := map[string]interface{}{
		"type": "object",
	}

	// Make a copy to compare later
	originalCopy := make(map[string]interface{})
	for k, v := range original {
		originalCopy[k] = v
	}

	// Normalize the schema
	normalized := mcp.NormalizeInputSchema(original, "test-tool")

	// Verify original is unchanged
	assert.Equal(t, originalCopy, original, "Original schema should not be modified")

	// Verify normalized has properties
	_, hasProperties := normalized["properties"]
	assert.True(t, hasProperties, "Normalized schema should have properties")

	// Verify original still doesn't have properties
	_, originalHasProperties := original["properties"]
	assert.False(t, originalHasProperties, "Original schema should not have properties")
}
