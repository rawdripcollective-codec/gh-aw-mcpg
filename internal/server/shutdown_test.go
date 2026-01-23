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

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestShutdownBehavior_RoutedMode tests that MCP endpoints return 503 after /close in routed mode
func TestShutdownBehavior_RoutedMode(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		method           string
		expectStatusCode int
		expectError      string
	}{
		{
			name:             "MCP endpoint rejected with 503 after shutdown",
			endpoint:         "/mcp/testserver",
			method:           "POST",
			expectStatusCode: http.StatusServiceUnavailable,
			expectError:      "Gateway is shutting down",
		},
		{
			name:             "MCP endpoint with trailing slash rejected with 503",
			endpoint:         "/mcp/testserver/",
			method:           "POST",
			expectStatusCode: http.StatusServiceUnavailable,
			expectError:      "Gateway is shutting down",
		},
		{
			name:             "Health endpoint still works during shutdown",
			endpoint:         "/health",
			method:           "GET",
			expectStatusCode: http.StatusOK,
			expectError:      "",
		},
		{
			name:             "Close endpoint returns 410 on subsequent calls",
			endpoint:         "/close",
			method:           "POST",
			expectStatusCode: http.StatusGone,
			expectError:      "Gateway has already been closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with a test server
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"testserver": {
						Command: "echo",
						Args:    []string{},
					},
				},
			}

			// Create unified server
			ctx := context.Background()
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err, "Failed to create unified server")
			defer us.Close()

			// Enable test mode to prevent exit
			us.SetTestMode(true)

			// Create HTTP server in routed mode
			httpServer := CreateHTTPServerForRoutedMode(":0", us, "")

			// Call /close to initiate shutdown
			closeReq := httptest.NewRequest("POST", "/close", nil)
			closeW := httptest.NewRecorder()
			httpServer.Handler.ServeHTTP(closeW, closeReq)

			// Verify close endpoint returned 200
			assert.Equal(t, http.StatusOK, closeW.Code, "Close endpoint should return 200 OK")

			// Now test the endpoint behavior after shutdown
			req := httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBufferString(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			httpServer.Handler.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.expectStatusCode, w.Code, "Unexpected status code")

			// Verify error message if expected
			if tt.expectError != "" {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err, "Failed to decode JSON response")
				if errorMsg, ok := response["error"].(string); ok {
					assert.Equal(t, tt.expectError, errorMsg, "Unexpected error message")
				} else {
					t.Errorf("Expected error field in response")
				}
			}
		})
	}
}

// TestShutdownBehavior_UnifiedMode tests that MCP endpoints return 503 after /close in unified mode
func TestShutdownBehavior_UnifiedMode(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		method           string
		expectStatusCode int
		expectError      string
	}{
		{
			name:             "MCP endpoint rejected with 503 after shutdown",
			endpoint:         "/mcp",
			method:           "POST",
			expectStatusCode: http.StatusServiceUnavailable,
			expectError:      "Gateway is shutting down",
		},
		{
			name:             "MCP endpoint with trailing slash rejected with 503",
			endpoint:         "/mcp/",
			method:           "POST",
			expectStatusCode: http.StatusServiceUnavailable,
			expectError:      "Gateway is shutting down",
		},
		{
			name:             "Health endpoint still works during shutdown",
			endpoint:         "/health",
			method:           "GET",
			expectStatusCode: http.StatusOK,
			expectError:      "",
		},
		{
			name:             "Close endpoint returns 410 on subsequent calls",
			endpoint:         "/close",
			method:           "POST",
			expectStatusCode: http.StatusGone,
			expectError:      "Gateway has already been closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{},
			}

			// Create unified server
			ctx := context.Background()
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err, "Failed to create unified server")
			defer us.Close()

			// Enable test mode to prevent exit
			us.SetTestMode(true)

			// Create HTTP server in unified mode
			httpServer := CreateHTTPServerForMCP(":0", us, "")

			// Call /close to initiate shutdown
			closeReq := httptest.NewRequest("POST", "/close", nil)
			closeW := httptest.NewRecorder()
			httpServer.Handler.ServeHTTP(closeW, closeReq)

			// Verify close endpoint returned 200
			assert.Equal(t, http.StatusOK, closeW.Code, "Close endpoint should return 200 OK")

			// Now test the endpoint behavior after shutdown
			req := httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBufferString(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			httpServer.Handler.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.expectStatusCode, w.Code, "Unexpected status code")

			// Verify error message if expected
			if tt.expectError != "" {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err, "Failed to decode JSON response")
				if errorMsg, ok := response["error"].(string); ok {
					assert.Equal(t, tt.expectError, errorMsg, "Unexpected error message")
				} else {
					t.Errorf("Expected error field in response")
				}
			}
		})
	}
}

// TestShutdownBehavior_WithAuth tests shutdown behavior when API key auth is enabled
func TestShutdownBehavior_WithAuth(t *testing.T) {
	// Create config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {
				Command: "echo",
				Args:    []string{},
			},
		},
	}

	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Enable test mode to prevent exit
	us.SetTestMode(true)

	apiKey := "test-api-key"

	// Create HTTP server with auth in routed mode
	httpServer := CreateHTTPServerForRoutedMode(":0", us, apiKey)

	// Call /close to initiate shutdown (with auth)
	closeReq := httptest.NewRequest("POST", "/close", nil)
	closeReq.Header.Set("Authorization", apiKey)
	closeW := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(closeW, closeReq)

	// Verify close endpoint returned 200
	assert.Equal(t, http.StatusOK, closeW.Code, "Close endpoint should return 200 OK")

	// Try to access MCP endpoint with valid auth after shutdown
	req := httptest.NewRequest("POST", "/mcp/testserver", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Verify 503 is returned even with valid auth (shutdown takes precedence)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "MCP endpoint should return 503 even with valid auth")

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err, "Failed to decode JSON response")
	assert.Equal(t, "Gateway is shutting down", response["error"], "Unexpected error message")
}

// TestShutdownBehavior_BeforeShutdown tests that endpoints work normally before shutdown
func TestShutdownBehavior_BeforeShutdown(t *testing.T) {
	// Create config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {
				Command: "echo",
				Args:    []string{},
			},
		},
	}

	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Create HTTP server in routed mode
	httpServer := CreateHTTPServerForRoutedMode(":0", us, "")

	// Try to access MCP endpoint BEFORE shutdown
	// Note: Without actual backend, this will fail with different errors,
	// but importantly it should NOT return 503
	req := httptest.NewRequest("POST", "/mcp/testserver", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "test-session-id")
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Verify NOT 503 (should be different error or success)
	assert.NotEqual(t, http.StatusServiceUnavailable, w.Code, "MCP endpoint should not return 503 before shutdown")
}
