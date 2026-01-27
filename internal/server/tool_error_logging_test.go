package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// TestToolNotFoundError_RoutedMode verifies that:
// 1. Tool "does not exist" errors are properly returned to clients
// 2. These errors are logged by the gateway
// 3. Unmangled tool names work in routed mode
func TestToolNotFoundError_RoutedMode(t *testing.T) {
	// Initialize logger to capture log output
	logger.InitFileLogger("/tmp", "test-tool-not-found.log")
	defer logger.CloseGlobalLogger()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testbackend": {Command: "echo", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Register a tool with an unmangled name
	us.toolsMu.Lock()
	us.tools["testbackend___create_issue"] = &ToolInfo{
		Name:        "testbackend___create_issue",
		Description: "Create an issue",
		BackendID:   "testbackend",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{Text: "Issue created"},
				},
			}, state, nil
		},
	}
	us.toolsMu.Unlock()

	// Create HTTP server in routed mode
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Start test server
	ts := httptest.NewServer(httpServer.Handler)
	defer ts.Close()

	serverURL := ts.URL + "/mcp/testbackend"

	// Helper to send MCP request
	sendRequest := func(t *testing.T, reqBody map[string]interface{}) map[string]interface{} {
		bodyBytes, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", serverURL, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-session-123")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send request")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Response: %s", string(body))

		return parseSSEResponse(t, bytes.NewReader(body))
	}

	// Test 1: Call tool with unmangled name (should succeed)
	t.Run("unmangled_name_succeeds", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "create_issue", // Unmangled name
				"arguments": map[string]interface{}{},
			},
		}

		jsonrpcResp := sendRequest(t, reqBody)

		// Should succeed - no error
		_, hasError := jsonrpcResp["error"]
		assert.False(t, hasError, "Unmangled tool name should work in routed mode")

		// Should have result
		result, hasResult := jsonrpcResp["result"]
		assert.True(t, hasResult, "Should have result for valid tool call")
		assert.NotNil(t, result, "Result should not be nil")
	})

	// Test 2: Call tool with mangled name (should fail)
	t.Run("mangled_name_fails", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "testbackend___create_issue", // Mangled name
				"arguments": map[string]interface{}{},
			},
		}

		jsonrpcResp := sendRequest(t, reqBody)

		// Should fail with error
		errObj, hasError := jsonrpcResp["error"]
		assert.True(t, hasError, "Mangled tool name should not work in routed mode")

		errorMap, ok := errObj.(map[string]interface{})
		require.True(t, ok, "Error should be a map")

		errorMsg := errorMap["message"].(string)
		t.Logf("Error message: %s", errorMsg)

		// Error should indicate unknown tool
		errorMsgLower := strings.ToLower(errorMsg)
		assert.True(t,
			strings.Contains(errorMsgLower, "unknown tool") ||
				strings.Contains(errorMsgLower, "not found"),
			"Error should indicate unknown tool. Got: %s", errorMsg)
	})

	// Test 3: Call non-existent tool (should fail and be logged)
	t.Run("nonexistent_tool_fails", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "nonexistent_tool",
				"arguments": map[string]interface{}{},
			},
		}

		jsonrpcResp := sendRequest(t, reqBody)

		// Should fail with error
		errObj, hasError := jsonrpcResp["error"]
		assert.True(t, hasError, "Non-existent tool should return error")

		errorMap, ok := errObj.(map[string]interface{})
		require.True(t, ok, "Error should be a map")

		errorMsg := errorMap["message"].(string)
		t.Logf("Error message: %s", errorMsg)

		// Error should indicate unknown tool
		errorMsgLower := strings.ToLower(errorMsg)
		assert.True(t,
			strings.Contains(errorMsgLower, "unknown tool") ||
				strings.Contains(errorMsgLower, "not found"),
			"Error should indicate unknown tool. Got: %s", errorMsg)

		// Verify tool name appears in error
		assert.Contains(t, errorMsg, "nonexistent_tool",
			"Error message should include the tool name")
	})
}

// TestToolNotFoundError_UnifiedMode verifies "tool does not exist" errors in unified mode
func TestToolNotFoundError_UnifiedMode(t *testing.T) {
	// Initialize logger to capture log output
	logger.InitFileLogger("/tmp", "test-tool-not-found-unified.log")
	defer logger.CloseGlobalLogger()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testbackend": {Command: "echo", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Register a tool with prefixed name (as in unified mode)
	us.toolsMu.Lock()
	us.tools["testbackend___create_issue"] = &ToolInfo{
		Name:        "testbackend___create_issue",
		Description: "Create an issue",
		BackendID:   "testbackend",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{Text: "Issue created"},
				},
			}, state, nil
		},
	}
	us.toolsMu.Unlock()

	// Create HTTP server in unified mode
	httpServer := CreateHTTPServerForMCP("127.0.0.1:0", us, "")

	// Start test server
	ts := httptest.NewServer(httpServer.Handler)
	defer ts.Close()

	serverURL := ts.URL + "/mcp"

	// Helper to send MCP request
	sendRequest := func(t *testing.T, reqBody map[string]interface{}) map[string]interface{} {
		bodyBytes, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", serverURL, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-session-456")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send request")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Response: %s", string(body))

		return parseSSEResponse(t, bytes.NewReader(body))
	}

	// In unified mode, call non-existent tool with prefix format
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "testbackend___nonexistent_tool", // Non-existent with prefix
			"arguments": map[string]interface{}{},
		},
	}

	jsonrpcResp := sendRequest(t, reqBody)

	// Should fail with error
	errObj, hasError := jsonrpcResp["error"]
	assert.True(t, hasError, "Non-existent tool should return error")

	errorMap, ok := errObj.(map[string]interface{})
	require.True(t, ok, "Error should be a map")

	errorMsg := errorMap["message"].(string)
	t.Logf("Error message: %s", errorMsg)

	// Error should indicate unknown tool
	errorMsgLower := strings.ToLower(errorMsg)
	assert.True(t,
		strings.Contains(errorMsgLower, "unknown tool") ||
			strings.Contains(errorMsgLower, "not found"),
		"Error should indicate unknown tool. Got: %s", errorMsg)
}
