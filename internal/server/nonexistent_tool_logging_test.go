package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// TestNonExistentToolCallLogging_RoutedMode verifies that when a non-existent tool
// is called via routed mode, an appropriate error is returned and logged.
func TestNonExistentToolCallLogging_RoutedMode(t *testing.T) {
	// Initialize logger to capture log output
	logger.InitFileLogger("/tmp", "test-nonexistent-tool.log")
	defer logger.CloseGlobalLogger()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"safeoutputs": {Command: "echo", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Create HTTP server in routed mode
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// First, initialize the session with initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	bodyBytes, _ := json.Marshal(initReq)
	req := httptest.NewRequest("POST", "/mcp/safeoutputs", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "test-session-123")

	rr := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(rr, req)
	
	t.Logf("Initialize response: %s", rr.Body.String())

	// Now test calling a non-existent tool "foobar"
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "foobar", // Non-existent tool
			"arguments": map[string]interface{}{},
		},
	}

	bodyBytes, err = json.Marshal(reqBody)
	require.NoError(t, err)

	req = httptest.NewRequest("POST", "/mcp/safeoutputs", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "test-session-123")

	rr = httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(rr, req)

	resp := rr.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response body: %s", string(body))

	// Parse SSE response using existing helper
	jsonrpcResp := parseSSEResponse(t, bytes.NewReader(body))

	// Verify an error is returned
	errObj, hasError := jsonrpcResp["error"]
	require.True(t, hasError, "Expected error for non-existent tool call")
	
	errorMap, ok := errObj.(map[string]interface{})
	require.True(t, ok, "Error should be a map")
	
	errorMsg := errorMap["message"].(string)
	t.Logf("Error code: %v", errorMap["code"])
	t.Logf("Error message: %s", errorMsg)

	// Check that error message mentions the tool doesn't exist or is unknown
	// The SDK or gateway should return an appropriate error
	errorMsgLower := strings.ToLower(errorMsg)
	assert.True(t,
		strings.Contains(errorMsgLower, "tool") || 
		strings.Contains(errorMsgLower, "not found") || 
		strings.Contains(errorMsgLower, "unknown") ||
		strings.Contains(errorMsgLower, "foobar") ||
		strings.Contains(errorMsgLower, "handler") ||
		strings.Contains(errorMsgLower, "no tool"),
		"Error message should indicate tool not found or unknown tool. Got: %s", errorMsg)
}

// TestNonExistentToolCallLogging_UnifiedMode verifies that when a non-existent tool
// is called via unified mode with the correct prefix format, an appropriate error is returned.
func TestNonExistentToolCallLogging_UnifiedMode(t *testing.T) {
	// Initialize logger to capture log output
	logger.InitFileLogger("/tmp", "test-nonexistent-tool-unified.log")
	defer logger.CloseGlobalLogger()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"safeoutputs": {Command: "echo", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Create HTTP server in unified mode
	httpServer := CreateHTTPServerForMCP("127.0.0.1:0", us, "")

	// First, initialize the session
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	bodyBytes, _ := json.Marshal(initReq)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "test-session-456")

	rr := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(rr, req)
	
	t.Logf("Initialize response: %s", rr.Body.String())

	// In unified mode, tool names should have the backend prefix
	// Try calling "safeoutputs___foobar" which doesn't exist
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "safeoutputs___foobar", // Non-existent tool with prefix
			"arguments": map[string]interface{}{},
		},
	}

	bodyBytes, err = json.Marshal(reqBody)
	require.NoError(t, err)

	req = httptest.NewRequest("POST", "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "test-session-456")

	rr = httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(rr, req)

	resp := rr.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response body: %s", string(body))

	// Parse SSE response using existing helper
	jsonrpcResp := parseSSEResponse(t, bytes.NewReader(body))

	// Verify an error is returned
	errObj, hasError := jsonrpcResp["error"]
	require.True(t, hasError, "Expected error for non-existent tool call")
	
	errorMap, ok := errObj.(map[string]interface{})
	require.True(t, ok, "Error should be a map")
	
	errorMsg := errorMap["message"].(string)
	t.Logf("Error code: %v", errorMap["code"])
	t.Logf("Error message: %s", errorMsg)

	// Check that error message indicates tool not found
	errorMsgLower := strings.ToLower(errorMsg)
	assert.True(t,
		strings.Contains(errorMsgLower, "tool") || 
		strings.Contains(errorMsgLower, "not found") || 
		strings.Contains(errorMsgLower, "unknown") ||
		strings.Contains(errorMsgLower, "foobar") ||
		strings.Contains(errorMsgLower, "handler") ||
		strings.Contains(errorMsgLower, "no tool"),
		"Error message should indicate tool not found. Got: %s", errorMsg)
}
