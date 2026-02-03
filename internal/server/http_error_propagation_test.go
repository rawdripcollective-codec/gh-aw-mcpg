package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/gh-aw-mcpg/internal/config"
	"github.com/github/gh-aw-mcpg/internal/launcher"
)

// TestUnifiedServer_HTTPErrorPropagation tests that HTTP backend errors are
// properly propagated through the unified server to clients
func TestUnifiedServer_HTTPErrorPropagation(t *testing.T) {
	tests := []struct {
		name           string
		backendStatus  int
		backendBody    map[string]interface{}
		expectHasError bool
		expectErrorMsg string // Expected substring in error message
	}{
		{
			name:          "HTTP 500 from backend",
			backendStatus: http.StatusInternalServerError,
			backendBody: map[string]interface{}{
				"error": "Database connection failed",
			},
			expectHasError: true,
			expectErrorMsg: "500",
		},
		{
			name:          "HTTP 503 from backend",
			backendStatus: http.StatusServiceUnavailable,
			backendBody: map[string]interface{}{
				"error": "Service temporarily unavailable",
			},
			expectHasError: true,
			expectErrorMsg: "503",
		},
		{
			name:          "HTTP 401 from backend",
			backendStatus: http.StatusUnauthorized,
			backendBody: map[string]interface{}{
				"error": "Invalid credentials",
			},
			expectHasError: true,
			expectErrorMsg: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0

			// Create mock HTTP backend
			backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++

				var reqBody map[string]interface{}
				json.NewDecoder(r.Body).Decode(&reqBody)
				method, _ := reqBody["method"].(string)

				if method == "initialize" {
					// Initialize succeeds
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      reqBody["id"],
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
				} else if method == "tools/list" && requestCount == 2 {
					// First tools/list (during initialization) succeeds
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      reqBody["id"],
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
					})
					return
				}

				// Subsequent requests return error
				w.WriteHeader(tt.backendStatus)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.backendBody)
			}))
			defer backendServer.Close()

			// Create gateway config
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test-backend": {
						Type: "http",
						URL:  backendServer.URL,
					},
				},
			}

			// Create launcher and unified server
			ctx := context.Background()
			l := launcher.New(ctx, cfg)
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err, "Failed to create unified server")
			defer us.Close()

			// Get connection
			conn, err := launcher.GetOrLaunch(l, "test-backend")
			require.NoError(t, err, "Failed to get connection")

			// Make request that triggers error
			resp, err := conn.SendRequestWithServerID(ctx, "tools/call",
				map[string]interface{}{
					"name":      "test_tool",
					"arguments": map[string]interface{}{},
				}, "test-backend")

			// With the fix, we should get a response with an error field, not a Go error
			require.NoError(t, err, "Should not return Go error")
			require.NotNil(t, resp, "Response should not be nil")

			if tt.expectHasError {
				require.NotNil(t, resp.Error, "Response should contain error field")
				assert.Contains(t, resp.Error.Message, tt.expectErrorMsg,
					"Error message should contain expected substring")
			}
		})
	}
}

// TestUnifiedServer_ChecksErrorBeforeUnmarshal tests that the unified server
// checks for error field before attempting to unmarshal result
func TestUnifiedServer_ChecksErrorBeforeUnmarshal(t *testing.T) {
	requestCount := 0

	// Create mock HTTP backend that returns error-only responses
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		method, _ := reqBody["method"].(string)

		if method == "initialize" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      reqBody["id"],
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
		} else if method == "tools/list" && requestCount == 2 {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      reqBody["id"],
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
			})
			return
		}

		// Return error response without result field
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      reqBody["id"],
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid request",
			},
		})
	}))
	defer backendServer.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-backend": {
				Type: "http",
				URL:  backendServer.URL,
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	conn, err := launcher.GetOrLaunch(us.launcher, "test-backend")
	require.NoError(t, err, "Failed to get connection")

	// Make request that returns error-only response
	resp, err := conn.SendRequestWithServerID(ctx, "tools/call",
		map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{},
		}, "test-backend")

	// Should get response without crashing
	require.NoError(t, err, "Should not return Go error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Error, "Response should contain error field")
}

// TestProxyToServer_HTTPErrorPropagation tests that the legacy proxy function
// properly handles HTTP backend errors
func TestProxyToServer_HTTPErrorPropagation(t *testing.T) {
	requestCount := 0

	// Create mock HTTP backend
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		method, _ := reqBody["method"].(string)

		if method == "initialize" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      reqBody["id"],
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

		// Return HTTP 503 error
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Service unavailable",
		})
	}))
	defer backendServer.Close()

	// Create gateway config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-backend": {
				Type: "http",
				URL:  backendServer.URL,
			},
		},
	}

	// Create launcher
	ctx := context.Background()
	l := launcher.New(ctx, cfg)

	// Create legacy server
	s := New(ctx, l, "routed")

	// Create test request
	jsonReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	reqBody, _ := json.Marshal(jsonReq)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/mcp/test-backend", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Handle request
	s.mux.ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&jsonResp)
	require.NoError(t, err, "Failed to decode response")

	// With the fix, the response should contain error details, not generic "Internal error"
	if errorField, ok := jsonResp["error"].(map[string]interface{}); ok {
		message, _ := errorField["message"].(string)
		// The error message should contain HTTP status information
		assert.Contains(t, message, "503", "Error message should mention HTTP 503 status")
	} else {
		t.Error("Response should contain error field")
	}
}

// TestHTTPBackendError_DataPreservation tests that error data is preserved
// when propagating errors from HTTP backends
func TestHTTPBackendError_DataPreservation(t *testing.T) {
	originalErrorData := map[string]interface{}{
		"type":        "rate_limit_error",
		"message":     "Rate limit exceeded",
		"retry_after": 60,
		"limit":       100,
		"remaining":   0,
	}

	requestCount := 0
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		method, _ := reqBody["method"].(string)

		if method == "initialize" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      reqBody["id"],
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
		} else if method == "tools/list" && requestCount == 2 {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      reqBody["id"],
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
			})
			return
		}

		// Return detailed error
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(originalErrorData)
	}))
	defer backendServer.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-backend": {
				Type: "http",
				URL:  backendServer.URL,
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	conn, err := launcher.GetOrLaunch(us.launcher, "test-backend")
	require.NoError(t, err, "Failed to get connection")

	resp, err := conn.SendRequestWithServerID(ctx, "tools/call",
		map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{},
		}, "test-backend")

	require.NoError(t, err, "Should not return Go error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Error, "Response should contain error field")

	// Verify error data is preserved
	require.NotNil(t, resp.Error.Data, "Error data should not be nil")

	var errorData map[string]interface{}
	err = json.Unmarshal(resp.Error.Data, &errorData)
	require.NoError(t, err, "Error data should be valid JSON")

	// Verify all original error fields are preserved
	assert.Equal(t, originalErrorData["type"], errorData["type"], "Error type should be preserved")
	assert.Equal(t, originalErrorData["message"], errorData["message"], "Error message should be preserved")
	assert.Equal(t, float64(60), errorData["retry_after"], "Retry-after should be preserved")
	assert.Equal(t, float64(100), errorData["limit"], "Limit should be preserved")
	assert.Equal(t, float64(0), errorData["remaining"], "Remaining should be preserved")
}
