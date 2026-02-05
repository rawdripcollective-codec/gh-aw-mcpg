package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHTTPConnection_WithCustomHeaders tests that custom headers skip SDK transports
// and use plain JSON-RPC transport directly
func TestNewHTTPConnection_WithCustomHeaders(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Track which transport was attempted
	serverCallCount := 0

	// Create test server that responds to initialize requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCallCount++

		// Verify custom headers are present
		assert.Equal("test-auth-token", r.Header.Get("Authorization"))
		assert.Equal("custom-value", r.Header.Get("X-Custom-Header"))

		// Return a valid initialize response
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "session-123")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create connection with custom headers
	customHeaders := map[string]string{
		"Authorization":   "test-auth-token",
		"X-Custom-Header": "custom-value",
	}

	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, customHeaders)
	require.NoError(err, "Failed to create HTTP connection with custom headers")
	require.NotNil(conn, "Connection should not be nil")
	defer conn.Close()

	// Verify connection properties
	assert.True(conn.IsHTTP(), "Connection should be HTTP")
	assert.Equal(testServer.URL, conn.GetHTTPURL())
	assert.Equal(HTTPTransportPlainJSON, conn.httpTransportType, "Should use plain JSON transport")
	assert.Equal("session-123", conn.httpSessionID, "Session ID should be captured")

	// Verify only one call was made (plain JSON transport, no fallback attempts)
	assert.Equal(1, serverCallCount, "Should only attempt plain JSON transport with custom headers")
}

// TestNewHTTPConnection_WithoutHeaders_FallbackSequence tests connection without custom headers.
// When no custom headers are provided, the code tries transports in order:
// streamable → SSE → plain JSON. If the server responds with valid JSON-RPC
// responses, the streamable transport succeeds since it's tried first.
func TestNewHTTPConnection_WithoutHeaders_FallbackSequence(t *testing.T) {
	require := require.New(t)

	// Create test server that responds to all requests with valid JSON-RPC
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept all POST requests with valid JSON-RPC response
		if r.Method == "POST" {
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "test-session")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Accept GET requests for SSE stream (streamable transport may need this)
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Reject other requests
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	// Create connection without custom headers - streamable transport should succeed first
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, nil)
	require.NoError(err, "Connection should succeed")
	require.NotNil(conn)
	defer conn.Close()

	// Streamable transport is tried first and should succeed since server responds correctly
	require.Equal(HTTPTransportStreamable, conn.httpTransportType)
	require.Equal("test-session", conn.httpSessionID)
}

// TestNewHTTPConnection_AllTransportsFail tests the case where all transports fail
func TestNewHTTPConnection_AllTransportsFail(t *testing.T) {
	require := require.New(t)

	// Create test server that rejects all requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "Service unavailable"}`))
	}))
	defer testServer.Close()

	// Try to create connection without custom headers (will try all transports)
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, nil)

	// Should fail after trying all transports
	require.Error(err, "Should fail when all transports fail")
	require.Nil(conn, "Connection should be nil on failure")
	require.Contains(err.Error(), "failed to connect using any HTTP transport")
}

// TestNewHTTPConnection_ContextCancellation tests that context cancellation is handled properly
func TestNewHTTPConnection_ContextCancellation(t *testing.T) {
	require := require.New(t)

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Create test server with slow responses to allow cancellation
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Cancel context immediately to trigger cancellation during connection
	cancel()

	// Try to create connection with cancelled context
	conn, err := NewHTTPConnection(ctx, "test-server", testServer.URL, map[string]string{"Auth": "token"})

	// Should fail due to context cancellation
	require.Error(err, "Should fail with cancelled context")
	require.Nil(conn, "Connection should be nil")
}

// TestNewHTTPConnection_InvalidURL tests error handling for invalid URLs
func TestNewHTTPConnection_InvalidURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		headers     map[string]string
		expectError bool
	}{
		{
			name:        "malformed URL",
			url:         "://invalid-url",
			headers:     map[string]string{"Auth": "token"},
			expectError: true,
		},
		{
			name:        "unreachable host",
			url:         "http://this-host-does-not-exist-12345.com",
			headers:     map[string]string{"Auth": "token"},
			expectError: true,
		},
		{
			name:        "unreachable port",
			url:         "http://localhost:99999", // Invalid port
			headers:     map[string]string{"Auth": "token"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewHTTPConnection(context.Background(), "test-server", tt.url, tt.headers)

			if tt.expectError {
				assert.Error(t, err, "Expected error for invalid URL")
				assert.Nil(t, conn, "Connection should be nil on error")
			} else {
				assert.NoError(t, err, "Should not error")
				assert.NotNil(t, conn, "Connection should not be nil")
				if conn != nil {
					conn.Close()
				}
			}
		})
	}
}

// TestTryPlainJSONTransport_InitializeFailure tests initialization failures
func TestTryPlainJSONTransport_InitializeFailure(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		errorSubstring string
	}{
		{
			name:           "HTTP 401 unauthorized",
			statusCode:     http.StatusUnauthorized,
			responseBody:   `{"error": "Unauthorized"}`,
			errorSubstring: "status=401",
		},
		{
			name:           "HTTP 403 forbidden",
			statusCode:     http.StatusForbidden,
			responseBody:   `{"error": "Forbidden"}`,
			errorSubstring: "status=403",
		},
		{
			name:           "HTTP 500 server error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "Internal server error"}`,
			errorSubstring: "status=500",
		},
		{
			name:           "invalid JSON response",
			statusCode:     http.StatusOK,
			responseBody:   `this is not valid JSON`,
			errorSubstring: "failed to parse",
		},
		{
			name:       "JSON-RPC error response",
			statusCode: http.StatusOK,
			responseBody: `{
				"jsonrpc": "2.0",
				"id": 1,
				"error": {
					"code": -32600,
					"message": "Invalid request"
				}
			}`,
			errorSubstring: "initialize error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with specific error response
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer testServer.Close()

			// Try to create connection
			conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
				"Authorization": "test-token",
			})

			// Should fail with appropriate error
			require.Error(t, err, "Should fail on initialization error")
			require.Nil(t, conn, "Connection should be nil")
			assert.Contains(t, err.Error(), tt.errorSubstring, "Error should contain expected substring")
		})
	}
}

// TestTryPlainJSONTransport_SSEFormattedResponse tests handling of SSE-formatted responses
func TestTryPlainJSONTransport_SSEFormattedResponse(t *testing.T) {
	require := require.New(t)

	// Create test server that returns SSE-formatted initialize response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return SSE-formatted response (like Tavily backend)
		response := `event: message
data: {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"test-server","version":"1.0.0"}}}

`
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "sse-session-456")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer testServer.Close()

	// Create connection
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
		"Authorization": "test-token",
	})

	require.NoError(err, "Should successfully parse SSE-formatted initialize response")
	require.NotNil(conn)
	defer conn.Close()

	// Verify session was captured
	assert.Equal(t, "sse-session-456", conn.httpSessionID)
	assert.Equal(t, HTTPTransportPlainJSON, conn.httpTransportType)
}

// TestTryPlainJSONTransport_NoSessionIDInResponse tests handling when server doesn't return session ID
func TestTryPlainJSONTransport_NoSessionIDInResponse(t *testing.T) {
	require := require.New(t)

	// Create test server that doesn't return Mcp-Session-Id header
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		// Deliberately not setting Mcp-Session-Id header
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create connection
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
		"Authorization": "test-token",
	})

	require.NoError(err, "Should succeed even without session ID from server")
	require.NotNil(conn)
	defer conn.Close()

	// Should have a temporary session ID
	assert.NotEmpty(t, conn.httpSessionID, "Should have temporary session ID")
	assert.Contains(t, conn.httpSessionID, "awmg-init-", "Should be temporary session ID")
}

// TestNewHTTPConnection_HeadersPropagation tests that custom headers are properly propagated
func TestNewHTTPConnection_HeadersPropagation(t *testing.T) {
	require := require.New(t)

	receivedHeaders := make(map[string]string)

	// Create test server that captures headers
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture all custom headers
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")
		receivedHeaders["X-API-Key"] = r.Header.Get("X-API-Key")
		receivedHeaders["X-Custom-1"] = r.Header.Get("X-Custom-1")
		receivedHeaders["X-Custom-2"] = r.Header.Get("X-Custom-2")

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create connection with multiple custom headers
	customHeaders := map[string]string{
		"Authorization": "Bearer my-token",
		"X-API-Key":     "api-key-123",
		"X-Custom-1":    "value1",
		"X-Custom-2":    "value2",
	}

	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, customHeaders)
	require.NoError(err)
	require.NotNil(conn)
	defer conn.Close()

	// Verify all headers were sent
	for key, expectedValue := range customHeaders {
		actualValue := receivedHeaders[key]
		assert.Equal(t, expectedValue, actualValue, "Header %s should match", key)
	}
}

// TestNewHTTPConnection_EmptyHeaders tests connection with empty header map.
// Empty headers behave the same as nil headers - streamable transport is tried first.
func TestNewHTTPConnection_EmptyHeaders(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept POST requests with valid JSON-RPC response
		if r.Method == "POST" {
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]interface{}{"name": "test"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "empty-headers-session")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Accept GET for SSE stream
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	// Create connection with empty headers - should try SDK transports first
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{})
	require.NoError(err, "Should succeed with empty headers")
	require.NotNil(conn)
	defer conn.Close()

	// Streamable transport is tried first and should succeed
	assert.Equal(t, HTTPTransportStreamable, conn.httpTransportType)
}

// TestNewHTTPConnection_NilHeaders tests connection with nil header map
func TestNewHTTPConnection_NilHeaders(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/" {
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]interface{}{"name": "test"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	// Create connection with nil headers (should try SDK transports first)
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, nil)
	require.NoError(err, "Should succeed with nil headers")
	require.NotNil(conn)
	defer conn.Close()

	// Verify connection is valid
	assert.True(t, conn.IsHTTP())
	assert.Equal(t, testServer.URL, conn.GetHTTPURL())
}

// TestNewHTTPConnection_HTTPClientTimeout tests that HTTP client timeout is properly configured
func TestNewHTTPConnection_HTTPClientTimeout(t *testing.T) {
	require := require.New(t)

	// Create test server with delayed response (but not too long to hit default 120s timeout)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Small delay to verify timeout handling works
		time.Sleep(50 * time.Millisecond)

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create connection
	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, map[string]string{
		"Authorization": "test",
	})

	// Should succeed (delay is within timeout)
	require.NoError(err, "Should succeed within timeout")
	require.NotNil(conn)
	defer conn.Close()

	// Verify HTTP client has proper timeout set
	assert.Equal(t, 120*time.Second, conn.httpClient.Timeout, "HTTP client should have 120s timeout")
}

// TestNewHTTPConnection_ConnectionRefused tests handling of connection refused errors
func TestNewHTTPConnection_ConnectionRefused(t *testing.T) {
	require := require.New(t)

	// Use an unreachable localhost port
	unreachableURL := "http://localhost:54321" // Assuming this port is not in use

	// Try to create connection
	conn, err := NewHTTPConnection(context.Background(), "test-server", unreachableURL, map[string]string{
		"Authorization": "test",
	})

	// Should fail with connection error
	require.Error(err, "Should fail with connection refused")
	require.Nil(conn, "Connection should be nil")
	assert.Contains(t, err.Error(), "failed to connect")
}

// TestNewHTTPConnection_GettersAfterCreation tests that getter methods work correctly
func TestNewHTTPConnection_GettersAfterCreation(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "getter-test-session")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	customHeaders := map[string]string{
		"Authorization": "test-token",
		"X-Custom":      "custom-value",
	}

	conn, err := NewHTTPConnection(context.Background(), "test-server", testServer.URL, customHeaders)
	require.NoError(err)
	require.NotNil(conn)
	defer conn.Close()

	// Test IsHTTP getter
	assert.True(t, conn.IsHTTP(), "IsHTTP() should return true")

	// Test GetHTTPURL getter
	assert.Equal(t, testServer.URL, conn.GetHTTPURL(), "GetHTTPURL() should return correct URL")

	// Test GetHTTPHeaders getter
	returnedHeaders := conn.GetHTTPHeaders()
	assert.Equal(t, len(customHeaders), len(returnedHeaders), "Should return all headers")
	for key, expectedValue := range customHeaders {
		assert.Equal(t, expectedValue, returnedHeaders[key], "Header %s should match", key)
	}
}
