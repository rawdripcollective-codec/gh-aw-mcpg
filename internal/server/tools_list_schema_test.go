package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/json"
	"github.com/github/gh-aw-mcpg/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
)

// TestToolsListIncludesInputSchema verifies that tools/list responses include
// inputSchema for all tools, which is required for clients to understand
// the parameter structure.
func TestToolsListIncludesInputSchema(t *testing.T) {
	// Create a mock backend that returns a tool with inputSchema
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		var request map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &request); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		method, _ := request["method"].(string)
		requestID := request["id"]

		if method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "backend-session-123")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-backend",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		if method == "tools/list" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "test_tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"body": map[string]interface{}{
										"type":        "string",
										"description": "The body parameter",
									},
								},
								"required": []string{"body"},
							},
						},
					},
				},
			})
			return
		}

		http.Error(w, "Unknown method", http.StatusBadRequest)
	}))
	defer mockBackend.Close()

	// Create gateway configuration with the mock backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {
				Type: "http",
				URL:  mockBackend.URL,
				Headers: map[string]string{
					"Authorization": "test-auth",
				},
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Check that tools registered in the UnifiedServer have InputSchema
	us.toolsMu.RLock()
	tools := us.tools
	us.toolsMu.RUnlock()

	require.NotEmpty(t, tools, "Should have registered tools")

	// Find our test tool
	var testTool *ToolInfo
	for name, tool := range tools {
		if tool.BackendID == "testserver" {
			testTool = tool
			t.Logf("Found tool: %s", name)
			break
		}
	}

	require.NotNil(t, testTool, "Should have found test tool")

	// Verify the tool has InputSchema
	assert.NotNil(t, testTool.InputSchema, "Tool MUST have InputSchema")
	assert.NotEmpty(t, testTool.InputSchema, "InputSchema should not be empty")

	// Verify the schema structure
	assert.Equal(t, "object", testTool.InputSchema["type"], "InputSchema should have type: object")
	assert.Contains(t, testTool.InputSchema, "properties", "InputSchema should have properties")

	propertiesValue := testTool.InputSchema["properties"]
	require.NotNil(t, propertiesValue, "properties value should not be nil")
	properties, ok := propertiesValue.(map[string]interface{})
	require.True(t, ok, "properties should be a map[string]interface{}")
	assert.Contains(t, properties, "body", "InputSchema should define the 'body' parameter")

	t.Logf("✓ Tool has proper InputSchema: %+v", testTool.InputSchema)
}

// TestGitHubGetCommitSchemaPatch verifies that the GitHub get_commit tool's
// broken schema (missing properties field) is properly patched by schema normalization.
// This is a regression test for the issue where GitHub's get_commit returns
// {"type": "object"} without the required "properties" field.
func TestGitHubGetCommitSchemaPatch(t *testing.T) {
	// Create a mock GitHub backend that returns get_commit with broken schema
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		var request map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &request); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		method, _ := request["method"].(string)
		requestID := request["id"]

		if method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "github-session-456")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "github-mcp-server",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		if method == "tools/list" {
			w.Header().Set("Content-Type", "application/json")
			// Simulate GitHub backend returning get_commit with broken schema
			// This matches the actual broken schema from GitHub's MCP server
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "get_commit",
							"description": "Get details for a commit from a GitHub repository",
							// Broken schema: has "type": "object" but missing "properties"
							"inputSchema": map[string]interface{}{
								"type": "object",
								// Missing "properties" field - this is the bug!
							},
						},
					},
				},
			})
			return
		}

		http.Error(w, "Unknown method", http.StatusBadRequest)
	}))
	defer mockGitHub.Close()

	// Create gateway configuration with the mock GitHub backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {
				Type: "http",
				URL:  mockGitHub.URL,
				Headers: map[string]string{
					"Authorization": "test-github-token",
				},
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Check that the get_commit tool was registered
	us.toolsMu.RLock()
	tools := us.tools
	us.toolsMu.RUnlock()

	require.NotEmpty(t, tools, "Should have registered tools")

	// Find the get_commit tool
	var getCommitTool *ToolInfo
	for name, tool := range tools {
		if tool.BackendID == "github" {
			getCommitTool = tool
			t.Logf("Found GitHub tool: %s", name)
			break
		}
	}

	require.NotNil(t, getCommitTool, "Should have found get_commit tool")

	// Verify the tool has InputSchema
	require.NotNil(t, getCommitTool.InputSchema, "get_commit MUST have InputSchema")
	assert.NotEmpty(t, getCommitTool.InputSchema, "InputSchema should not be empty")

	// Verify the schema was properly patched
	assert.Equal(t, "object", getCommitTool.InputSchema["type"], "InputSchema should have type: object")

	// THIS IS THE KEY ASSERTION: The broken schema should have been patched
	// to include a "properties" field
	assert.Contains(t, getCommitTool.InputSchema, "properties", "InputSchema MUST have properties field (should be patched)")

	// Verify properties is a valid empty map (since the original had no properties defined)
	propertiesValue := getCommitTool.InputSchema["properties"]
	require.NotNil(t, propertiesValue, "properties value should not be nil")
	properties, ok := propertiesValue.(map[string]interface{})
	require.True(t, ok, "properties should be a map[string]interface{}")

	// The properties should be an empty map since the original schema had none
	assert.Empty(t, properties, "properties should be an empty map for patched schema")

	t.Logf("✓ GitHub get_commit schema properly patched: %+v", getCommitTool.InputSchema)
	t.Log("✓ Schema normalization successfully fixed the broken GitHub get_commit schema")
}
