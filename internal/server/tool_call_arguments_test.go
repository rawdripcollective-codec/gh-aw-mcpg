package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/gh-aw-mcpg/internal/config"
)

// TestUnifiedServer_ToolCallArguments tests that tool arguments are correctly passed through the gateway
func TestUnifiedServer_ToolCallArguments(t *testing.T) {
	// Track what the mock backend received
	var receivedToolCalls []map[string]interface{}
	var mu sync.Mutex

	// Create a mock MCP backend server
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Failed to read request body: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		var request map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &request); err != nil {
			t.Logf("Failed to unmarshal request: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		method, _ := request["method"].(string)
		requestID := request["id"]

		t.Logf("Backend received: method=%s, id=%v", method, requestID)

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
									"param1": map[string]interface{}{"type": "string"},
									"param2": map[string]interface{}{"type": "number"},
								},
							},
						},
					},
				},
			})
			return
		}

		if method == "tools/call" {
			params, _ := request["params"].(map[string]interface{})

			// Log the entire params structure
			mu.Lock()
			receivedToolCalls = append(receivedToolCalls, params)
			mu.Unlock()

			paramsJSON, _ := json.Marshal(params)
			t.Logf("Backend received tools/call params: %s", string(paramsJSON))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Success",
						},
					},
				},
			})
			return
		}

		http.Error(w, fmt.Sprintf("Unknown method: %s", method), http.StatusBadRequest)
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

	// Simulate a tool call with arguments
	testArgs := map[string]interface{}{
		"param1": "test_value",
		"param2": float64(42),
		"param3": map[string]interface{}{
			"nested": "value",
		},
	}

	// Call the tool through the unified server
	var callErr error
	_, _, callErr = us.callBackendTool(ctx, "testserver", "test_tool", testArgs)

	// Verify the backend received the tool call first (this is the critical test)
	mu.Lock()
	defer mu.Unlock()

	require.GreaterOrEqual(t, len(receivedToolCalls), 1, "Backend should have received at least one tool call")

	// Check the most recent tool call
	lastCall := receivedToolCalls[len(receivedToolCalls)-1]
	t.Logf("Last tool call received by backend: %+v", lastCall)

	// Verify the tool name was passed
	assert.Equal(t, "test_tool", lastCall["name"], "Tool name should match")

	// Verify the arguments were passed and not empty
	arguments, ok := lastCall["arguments"].(map[string]interface{})
	require.True(t, ok, "Arguments should be a map")
	require.NotEmpty(t, arguments, "Arguments should not be empty")

	// Verify specific argument values
	assert.Equal(t, "test_value", arguments["param1"], "param1 should match")
	assert.Equal(t, float64(42), arguments["param2"], "param2 should match")

	nestedMap, ok := arguments["param3"].(map[string]interface{})
	require.True(t, ok, "param3 should be a nested map")
	assert.Equal(t, "value", nestedMap["nested"], "Nested value should match")

	// Now check the result
	if callErr != nil {
		t.Logf("Error calling tool: %v", callErr)
	}
	// Note: We don't require no error since we've already verified arguments were passed correctly
}
