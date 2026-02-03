package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallTool_ArgumentsPassed tests that tool arguments are correctly passed to the backend
func TestCallTool_ArgumentsPassed(t *testing.T) {
	tests := []struct {
		name              string
		inputParams       map[string]interface{}
		expectedArguments map[string]interface{}
	}{
		{
			name: "simple string argument",
			inputParams: map[string]interface{}{
				"name": "test_tool",
				"arguments": map[string]interface{}{
					"query": "test query",
				},
			},
			expectedArguments: map[string]interface{}{
				"query": "test query",
			},
		},
		{
			name: "multiple arguments",
			inputParams: map[string]interface{}{
				"name": "list_issues",
				"arguments": map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
					"state": "open",
				},
			},
			expectedArguments: map[string]interface{}{
				"owner": "github",
				"repo":  "gh-aw-mcpg",
				"state": "open",
			},
		},
		{
			name: "nested object arguments",
			inputParams: map[string]interface{}{
				"name": "complex_tool",
				"arguments": map[string]interface{}{
					"config": map[string]interface{}{
						"timeout": 30,
						"retry":   true,
					},
					"filters": []string{"tag1", "tag2"},
				},
			},
			expectedArguments: map[string]interface{}{
				"config": map[string]interface{}{
					"timeout": float64(30), // JSON numbers are float64
					"retry":   true,
				},
				"filters": []interface{}{"tag1", "tag2"},
			},
		},
		{
			name: "empty arguments",
			inputParams: map[string]interface{}{
				"name":      "no_args_tool",
				"arguments": map[string]interface{}{},
			},
			expectedArguments: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track what arguments the backend received
			var receivedArguments map[string]interface{}

			// Create a mock backend server
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read the request body
				bodyBytes, err := io.ReadAll(r.Body)
				require.NoError(t, err, "Failed to read request body")

				var request map[string]interface{}
				err = json.Unmarshal(bodyBytes, &request)
				require.NoError(t, err, "Failed to parse request JSON")

				method, _ := request["method"].(string)

				if method == "initialize" {
					// Return success for initialize
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Mcp-Session-Id", "test-session-123")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      request["id"],
						"result": map[string]interface{}{
							"protocolVersion": "2024-11-05",
							"serverInfo": map[string]interface{}{
								"name":    "test-server",
								"version": "1.0.0",
							},
						},
					})
					return
				}

				if method == "tools/call" {
					// Extract the arguments from the params
					params, ok := request["params"].(map[string]interface{})
					require.True(t, ok, "params should be a map")

					// Store the arguments we received
					if args, ok := params["arguments"].(map[string]interface{}); ok {
						receivedArguments = args
					} else {
						receivedArguments = map[string]interface{}{}
					}

					// Return success
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      request["id"],
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

				// Unknown method
				http.Error(w, "Unknown method", http.StatusBadRequest)
			}))
			defer testServer.Close()

			// Create connection
			conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
				"Authorization": "test-token",
			})
			require.NoError(t, err, "Failed to create HTTP connection")
			defer conn.Close()

			// Send the tool call request
			_, err = conn.SendRequestWithServerID(context.Background(), "tools/call", tt.inputParams, "test-server")
			require.NoError(t, err, "Tool call should succeed")

			// Verify the arguments were passed correctly
			assert.Equal(t, tt.expectedArguments, receivedArguments, "Arguments should match expected values")
		})
	}
}

// TestCallTool_MissingArguments tests behavior when arguments field is missing
func TestCallTool_MissingArguments(t *testing.T) {
	var receivedParams map[string]interface{}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var request map[string]interface{}
		json.Unmarshal(bodyBytes, &request)

		method, _ := request["method"].(string)

		if method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "test-session-123")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		if method == "tools/call" {
			params, _ := request["params"].(map[string]interface{})
			receivedParams = params

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Success",
						},
					},
				},
			})
		}
	}))
	defer testServer.Close()

	conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
		"Authorization": "test-token",
	})
	require.NoError(t, err)
	defer conn.Close()

	// Test 1: Send request with arguments field omitted entirely
	t.Run("omitted arguments field", func(t *testing.T) {
		receivedParams = nil
		params := map[string]interface{}{
			"name": "test_tool",
			// No "arguments" field
		}

		_, err = conn.SendRequestWithServerID(context.Background(), "tools/call", params, "test-server")
		require.NoError(t, err)

		// Verify arguments field exists in the request sent to backend
		assert.NotNil(t, receivedParams, "Params should be sent to backend")
		assert.Contains(t, receivedParams, "name", "Name should be present")

		// The arguments field should be present (even if empty)
		// This is the key: the MCP spec requires arguments to be present
		_, hasArguments := receivedParams["arguments"]
		assert.True(t, hasArguments, "Arguments field should be present in backend request even if not provided by client")
	})

	// Test 2: Send request with explicit null arguments
	t.Run("null arguments", func(t *testing.T) {
		receivedParams = nil
		params := map[string]interface{}{
			"name":      "test_tool",
			"arguments": nil,
		}

		_, err = conn.SendRequestWithServerID(context.Background(), "tools/call", params, "test-server")
		require.NoError(t, err)

		assert.NotNil(t, receivedParams, "Params should be sent to backend")

		// Arguments should be present, even if nil/empty
		_, hasArguments := receivedParams["arguments"]
		assert.True(t, hasArguments, "Arguments field should be present even if nil")
	})

	// Test 3: Send request with empty arguments object
	t.Run("empty arguments object", func(t *testing.T) {
		receivedParams = nil
		params := map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{},
		}

		_, err = conn.SendRequestWithServerID(context.Background(), "tools/call", params, "test-server")
		require.NoError(t, err)

		assert.NotNil(t, receivedParams, "Params should be sent to backend")

		// Arguments should be present as an object
		arguments, hasArguments := receivedParams["arguments"]
		assert.True(t, hasArguments, "Arguments field should be present")

		// It should be an empty map
		if argsMap, ok := arguments.(map[string]interface{}); ok {
			assert.Empty(t, argsMap, "Arguments should be an empty map")
		}
	})
}
