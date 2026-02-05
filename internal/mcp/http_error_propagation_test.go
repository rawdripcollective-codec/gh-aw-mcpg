package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPErrorPropagation_Non200Status tests that non-200 HTTP status codes
// are properly converted to JSON-RPC error responses
func TestHTTPErrorPropagation_Non200Status(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   map[string]interface{}
		expectErrorMsg string // Expected substring in error message
	}{
		{
			name:       "HTTP 400 Bad Request",
			statusCode: http.StatusBadRequest,
			responseBody: map[string]interface{}{
				"error": "Invalid parameters",
			},
			expectErrorMsg: "400",
		},
		{
			name:       "HTTP 401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			responseBody: map[string]interface{}{
				"error": "Authentication required",
			},
			expectErrorMsg: "401",
		},
		{
			name:       "HTTP 403 Forbidden",
			statusCode: http.StatusForbidden,
			responseBody: map[string]interface{}{
				"error": "Access denied",
			},
			expectErrorMsg: "403",
		},
		{
			name:       "HTTP 404 Not Found",
			statusCode: http.StatusNotFound,
			responseBody: map[string]interface{}{
				"error": "Resource not found",
			},
			expectErrorMsg: "404",
		},
		{
			name:       "HTTP 500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			responseBody: map[string]interface{}{
				"error": "Database connection failed",
			},
			expectErrorMsg: "500",
		},
		{
			name:       "HTTP 502 Bad Gateway",
			statusCode: http.StatusBadGateway,
			responseBody: map[string]interface{}{
				"error": "Upstream service unavailable",
			},
			expectErrorMsg: "502",
		},
		{
			name:       "HTTP 503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			responseBody: map[string]interface{}{
				"error": "Service temporarily down",
			},
			expectErrorMsg: "503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0

			// Create test server that succeeds on initialize but fails on subsequent requests
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
								"name":    "test-server",
								"version": "1.0.0",
							},
						},
					})
					return
				}

				// Subsequent requests return the configured error status
				w.WriteHeader(tt.statusCode)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer testServer.Close()

			// Create connection with custom headers to use plain JSON transport
			conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
				"Authorization": "test-token",
			})
			require.NoError(t, err, "Failed to create connection")
			defer conn.Close()

			// Send request that will trigger error response
			resp, err := conn.SendRequestWithServerID(context.Background(), "tools/list", nil, "test-server")
			require.NoError(t, err, "SendRequestWithServerID should not return Go error")
			require.NotNil(t, resp, "Response should not be nil")

			// Verify the response contains an error field
			require.NotNil(t, resp.Error, "Response should contain error field")
			assert.Equal(t, -32603, resp.Error.Code, "Error code should be -32603 (Internal error)")
			assert.Contains(t, resp.Error.Message, tt.expectErrorMsg,
				"Error message should contain HTTP status code")

			// Verify error data contains original response body
			if resp.Error.Data != nil {
				var errorData interface{}
				err := json.Unmarshal(resp.Error.Data, &errorData)
				require.NoError(t, err, "Error data should be valid JSON")
			}
		})
	}
}

// TestHTTPErrorPropagation_JSONRPCError tests that HTTP 200 responses with
// JSON-RPC error field are properly returned
func TestHTTPErrorPropagation_JSONRPCError(t *testing.T) {
	requestCount := 0

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		// Return JSON-RPC error with HTTP 200
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      reqBody["id"],
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid Request",
				"data":    "Tool not found",
			},
		})
	}))
	defer testServer.Close()

	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
		"Authorization": "test-token",
	})
	require.NoError(t, err, "Failed to create connection")
	defer conn.Close()

	// Send request
	resp, err := conn.SendRequestWithServerID(context.Background(), "tools/call",
		map[string]interface{}{"name": "unknown"}, "test-server")
	require.NoError(t, err, "SendRequestWithServerID should not return Go error")
	require.NotNil(t, resp, "Response should not be nil")

	// Verify the response contains the JSON-RPC error
	require.NotNil(t, resp.Error, "Response should contain error field")
	assert.Equal(t, -32600, resp.Error.Code, "Error code should match backend error")
	assert.Equal(t, "Invalid Request", resp.Error.Message, "Error message should match backend error")
}

// TestHTTPErrorPropagation_MixedContent tests error responses with mixed content types
func TestHTTPErrorPropagation_MixedContent(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string // Raw response body
		contentType  string
	}{
		{
			name:         "Plain text error",
			statusCode:   http.StatusInternalServerError,
			responseBody: "Internal Server Error",
			contentType:  "text/plain",
		},
		{
			name:         "HTML error page",
			statusCode:   http.StatusNotFound,
			responseBody: "<html><body>404 Not Found</body></html>",
			contentType:  "text/html",
		},
		{
			name:         "JSON error without JSON-RPC structure",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"message": "Bad request", "code": "INVALID_PARAMS"}`,
			contentType:  "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
								"name":    "test-server",
								"version": "1.0.0",
							},
						},
					})
					return
				}

				// Return error with specified content type
				w.WriteHeader(tt.statusCode)
				w.Header().Set("Content-Type", tt.contentType)
				w.Write([]byte(tt.responseBody))
			}))
			defer testServer.Close()

			conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
				"Authorization": "test-token",
			})
			require.NoError(t, err, "Failed to create connection")
			defer conn.Close()

			// Send request
			resp, err := conn.SendRequestWithServerID(context.Background(), "tools/list", nil, "test-server")
			require.NoError(t, err, "SendRequestWithServerID should not return Go error")
			require.NotNil(t, resp, "Response should not be nil")

			// Verify the response contains an error field
			require.NotNil(t, resp.Error, "Response should contain error field")
			assert.Equal(t, -32603, resp.Error.Code, "Error code should be -32603")

			// Verify error data contains original response body
			if resp.Error.Data != nil {
				data := string(resp.Error.Data)
				assert.Contains(t, data, tt.responseBody, "Error data should contain original response")
			}
		})
	}
}

// TestHTTPErrorPropagation_PreservesDetails tests that error details are preserved
func TestHTTPErrorPropagation_PreservesDetails(t *testing.T) {
	requestCount := 0
	originalError := map[string]interface{}{
		"type":    "authentication_error",
		"message": "API key is invalid or expired",
		"details": map[string]interface{}{
			"apiKeyPrefix": "sk-test-****",
			"expiresAt":    "2026-01-01T00:00:00Z",
		},
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		// Return detailed error
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(originalError)
	}))
	defer testServer.Close()

	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
		"Authorization": "test-token",
	})
	require.NoError(t, err, "Failed to create connection")
	defer conn.Close()

	// Send request
	resp, err := conn.SendRequestWithServerID(context.Background(), "tools/list", nil, "test-server")
	require.NoError(t, err, "SendRequestWithServerID should not return Go error")
	require.NotNil(t, resp, "Response should not be nil")

	// Verify error is present
	require.NotNil(t, resp.Error, "Response should contain error field")
	assert.Contains(t, resp.Error.Message, "401", "Error message should contain status code")

	// Verify error data contains original error details
	require.NotNil(t, resp.Error.Data, "Error data should not be nil")

	var errorData map[string]interface{}
	err = json.Unmarshal(resp.Error.Data, &errorData)
	require.NoError(t, err, "Error data should be valid JSON")

	// Verify original error details are preserved
	assert.Equal(t, originalError["type"], errorData["type"], "Error type should be preserved")
	assert.Equal(t, originalError["message"], errorData["message"], "Error message should be preserved")

	details, ok := errorData["details"].(map[string]interface{})
	require.True(t, ok, "Error details should be preserved")
	assert.NotNil(t, details["apiKeyPrefix"], "API key prefix should be preserved")
	assert.NotNil(t, details["expiresAt"], "Expiration time should be preserved")
}
