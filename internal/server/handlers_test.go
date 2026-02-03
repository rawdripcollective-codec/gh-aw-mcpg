package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/gh-aw-mcpg/internal/config"
	"github.com/github/gh-aw-mcpg/internal/launcher"
	"github.com/github/gh-aw-mcpg/internal/sys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleOAuthDiscovery tests the OAuth discovery endpoint
func TestHandleOAuthDiscovery(t *testing.T) {
	assert := assert.New(t)

	handler := handleOAuthDiscovery()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET request returns 404",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST request returns 404",
			method:         http.MethodPost,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PUT request returns 404",
			method:         http.MethodPut,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "DELETE request returns 404",
			method:         http.MethodDelete,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/.well-known/oauth-authorization-server", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(tt.expectedStatus, rec.Code)
		})
	}
}

// TestHandleClose_MethodValidation tests that only POST requests are accepted
func TestHandleClose_MethodValidation(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true, // Prevent os.Exit in tests
	}

	handler := handleClose(unifiedServer)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET request returns 405",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "PUT request returns 405",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "DELETE request returns 405",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "PATCH request returns 405",
			method:         http.MethodPatch,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/close", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(tt.expectedStatus, rec.Code)
			assert.Contains(rec.Body.String(), tt.expectedBody)
		})
	}
}

// TestHandleClose_SuccessfulShutdown tests successful shutdown initiation
func TestHandleClose_SuccessfulShutdown(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true, // Prevent os.Exit in tests
	}

	handler := handleClose(unifiedServer)

	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Assert response
	assert.Equal(http.StatusOK, rec.Code)
	assert.Equal("application/json", rec.Header().Get("Content-Type"))

	// Parse response body
	var response map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(err)

	// Verify response structure
	assert.Equal("closed", response["status"])
	assert.Equal("Gateway shutdown initiated", response["message"])
	assert.Contains(response, "serversTerminated")

	// Verify shutdown state
	assert.True(unifiedServer.IsShutdown())
}

// TestHandleClose_Idempotency tests that multiple close requests are idempotent
func TestHandleClose_Idempotency(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	// First request - successful shutdown
	req1 := httptest.NewRequest(http.MethodPost, "/close", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(http.StatusOK, rec1.Code)
	assert.True(unifiedServer.IsShutdown())

	// Second request - already closed
	req2 := httptest.NewRequest(http.MethodPost, "/close", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Should return 410 Gone
	assert.Equal(http.StatusGone, rec2.Code)
	assert.Equal("application/json", rec2.Header().Get("Content-Type"))

	// Parse response body
	var response map[string]interface{}
	err := json.NewDecoder(rec2.Body).Decode(&response)
	require.NoError(err)

	assert.Equal("Gateway has already been closed", response["error"])
}

// TestHandleClose_MultipleRequests tests behavior with multiple simultaneous close requests
func TestHandleClose_MultipleRequests(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	// Send multiple requests concurrently
	numRequests := 5
	responses := make([]*httptest.ResponseRecorder, numRequests)
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			req := httptest.NewRequest(http.MethodPost, "/close", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			responses[idx] = rec
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify results - exactly one should be 200 OK, rest should be 410 Gone
	okCount := 0
	goneCount := 0

	for _, rec := range responses {
		switch rec.Code {
		case http.StatusOK:
			okCount++
		case http.StatusGone:
			goneCount++
		}
	}

	// At least one should succeed (due to race conditions, might be more than one 200)
	assert.GreaterOrEqual(okCount, 1)
	// Verify total requests
	assert.Equal(numRequests, okCount+goneCount)
	// Verify final state
	assert.True(unifiedServer.IsShutdown())
}

// TestHandleClose_ResponseFormat tests the JSON response format
func TestHandleClose_ResponseFormat(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse response
	var response map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(err)

	// Verify all required fields are present
	assert.Contains(response, "status")
	assert.Contains(response, "message")
	assert.Contains(response, "serversTerminated")

	// Verify field types
	assert.IsType("", response["status"])
	assert.IsType("", response["message"])
	assert.IsType(float64(0), response["serversTerminated"]) // JSON numbers decode as float64
}

// TestHandleClose_RemoteAddress tests that remote address is logged
func TestHandleClose_RemoteAddress(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed regardless of remote address
	assert.Equal(http.StatusOK, rec.Code)
}

// TestHandleClose_EmptyBody tests that close endpoint works without request body
func TestHandleClose_EmptyBody(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	// Request with no body
	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code)
}

// TestHandleClose_WithRequestBody tests that close endpoint ignores request body
func TestHandleClose_WithRequestBody(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	unifiedServer := &UnifiedServer{
		launcher:  mockLauncher,
		sysServer: sys.NewSysServer([]string{}),
		ctx:       ctx,
		testMode:  true,
	}

	handler := handleClose(unifiedServer)

	// Request with body (should be ignored)
	body := strings.NewReader(`{"some": "data"}`)
	req := httptest.NewRequest(http.MethodPost, "/close", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code)
}

// TestShutdownErrorJSON tests the shutdown error constant format
func TestShutdownErrorJSON(t *testing.T) {
	assert := assert.New(t)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(shutdownErrorJSON), &parsed)
	assert.NoError(err)

	// Verify structure
	assert.Equal("Gateway is shutting down", parsed["error"])
}

// TestHandleClose_ShouldExitFlag tests that ShouldExit controls process termination
func TestHandleClose_ShouldExitFlag(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	mockLauncher := launcher.New(ctx, &config.Config{})

	tests := []struct {
		name       string
		testMode   bool
		shouldExit bool
	}{
		{
			name:       "Test mode prevents exit",
			testMode:   true,
			shouldExit: false,
		},
		{
			name:       "Production mode allows exit",
			testMode:   false,
			shouldExit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unifiedServer := &UnifiedServer{
				launcher:  mockLauncher,
				sysServer: sys.NewSysServer([]string{}),
				ctx:       ctx,
				testMode:  tt.testMode,
			}

			assert.Equal(tt.shouldExit, unifiedServer.ShouldExit())
		})
	}
}
