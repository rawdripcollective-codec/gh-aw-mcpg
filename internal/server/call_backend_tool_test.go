package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/gh-aw-mcpg/internal/config"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallBackendTool_ReturnsNonNilCallToolResult tests the critical bug fix
// This test verifies that callBackendTool returns a proper *sdk.CallToolResult
// instead of nil on successful tool calls.
//
// THE BUG: Before the fix, callBackendTool returned (nil, finalResult, nil)
// THE FIX: Now it returns (&CallToolResult{...}, finalResult, nil)
//
// This test will FAIL with the old buggy code and PASS with the fix.
func TestCallBackendTool_ReturnsNonNilCallToolResult(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Create a mock HTTP backend that returns a successful tool response
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		method, ok := req["method"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch method {
		case "initialize":
			// Return initialization response
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-backend",
						"version": "1.0.0",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "tools/list":
			// Return a single test tool
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "test_tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{
								"type":       "object",
								"properties": map[string]interface{}{},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "tools/call":
			// Return successful tool response with content
			// This is the critical part - the backend returns content array
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "Success! Tool executed correctly.",
						},
					},
					"isError": false,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer backend.Close()

	// Create unified server with the mock backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-backend": {
				Type: "http",
				URL:  backend.URL,
			},
		},
		EnableDIFC: false, // Disable DIFC for simpler testing
	}

	us, err := NewUnified(context.Background(), cfg)
	require.NoError(err)
	require.NotNil(us)

	// Create context with session
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	// Call the backend tool directly
	result, data, err := us.callBackendTool(ctx, "test-backend", "test_tool", map[string]interface{}{})

	// ===== THE CRITICAL ASSERTION =====
	// This assertion will FAIL with the old buggy code (which returned nil)
	// and PASS with the fix (which returns a proper CallToolResult)
	require.NotNil(result, "CRITICAL BUG: callBackendTool MUST return non-nil CallToolResult on success!")

	// Additional validations
	require.NoError(err, "Tool call should succeed without error")
	require.NotNil(data, "Data should not be nil")

	// Verify the result has proper structure
	assert.False(result.IsError, "Result should not be marked as error")
	require.NotNil(result.Content, "Result Content should not be nil")
	assert.Greater(len(result.Content), 0, "Result should have at least one content item")

	// Verify content is properly converted
	if len(result.Content) > 0 {
		textContent, ok := result.Content[0].(*sdk.TextContent)
		require.True(ok, "Content should be TextContent type")
		assert.Equal("Success! Tool executed correctly.", textContent.Text)
	}

	t.Log("✓ PASS: callBackendTool returns non-nil CallToolResult on success")
}

// TestCallBackendTool_ErrorStillReturnsCallToolResult verifies that even
// on error, we return a CallToolResult (with IsError: true), not nil
func TestCallBackendTool_ErrorStillReturnsCallToolResult(t *testing.T) {
	require := require.New(t)

	// Create a mock HTTP backend that returns an error
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		method, ok := req["method"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch method {
		case "initialize":
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "error-backend",
						"version": "1.0.0",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "tools/list":
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "error_tool",
							"description": "A tool that errors",
							"inputSchema": map[string]interface{}{
								"type":       "object",
								"properties": map[string]interface{}{},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "tools/call":
			// Return error from backend
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"error": map[string]interface{}{
					"code":    -32603,
					"message": "Internal error in tool",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"error-backend": {
				Type: "http",
				URL:  backend.URL,
			},
		},
		EnableDIFC: false,
	}

	us, err := NewUnified(context.Background(), cfg)
	require.NoError(err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	result, _, err := us.callBackendTool(ctx, "error-backend", "error_tool", map[string]interface{}{})

	// Even on error, should return a CallToolResult (not nil)
	require.NotNil(result, "Even on error, should return non-nil CallToolResult")
	require.Error(err, "Should return an error")
	require.True(result.IsError, "Result should be marked as error")

	t.Log("✓ PASS: callBackendTool returns non-nil CallToolResult even on error")
}
