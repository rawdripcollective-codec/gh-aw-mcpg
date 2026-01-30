package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware tests the authMiddleware function with various scenarios
func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name               string
		configuredAPIKey   string
		authHeader         string
		expectStatusCode   int
		expectNextCalled   bool
		expectErrorMessage string
	}{
		{
			name:               "ValidAPIKey",
			configuredAPIKey:   "test-api-key-123",
			authHeader:         "test-api-key-123",
			expectStatusCode:   http.StatusOK,
			expectNextCalled:   true,
			expectErrorMessage: "",
		},
		{
			name:               "MissingAuthorizationHeader",
			configuredAPIKey:   "test-api-key-123",
			authHeader:         "",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: missing Authorization header",
		},
		{
			name:               "InvalidAPIKey",
			configuredAPIKey:   "correct-key",
			authHeader:         "wrong-key",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: invalid API key",
		},
		{
			name:               "EmptyAPIKeyWithEmptyHeader",
			configuredAPIKey:   "",
			authHeader:         "",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: missing Authorization header",
		},
		{
			name:               "EmptyConfiguredKeyWithValidHeader",
			configuredAPIKey:   "",
			authHeader:         "some-key",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: invalid API key",
		},
		{
			name:               "CaseSensitiveKey",
			configuredAPIKey:   "MyAPIKey",
			authHeader:         "myapikey",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: invalid API key",
		},
		{
			name:               "WhitespaceNotTrimmed",
			configuredAPIKey:   "test-key",
			authHeader:         " test-key ",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: invalid API key",
		},
		{
			name:               "BearerSchemeNotSupported",
			configuredAPIKey:   "test-key",
			authHeader:         "Bearer test-key",
			expectStatusCode:   http.StatusUnauthorized,
			expectNextCalled:   false,
			expectErrorMessage: "Unauthorized: invalid API key",
		},
		{
			name:               "LongAPIKey",
			configuredAPIKey:   "this-is-a-very-long-api-key-with-many-characters-1234567890",
			authHeader:         "this-is-a-very-long-api-key-with-many-characters-1234567890",
			expectStatusCode:   http.StatusOK,
			expectNextCalled:   true,
			expectErrorMessage: "",
		},
		{
			name:               "SpecialCharactersInKey",
			configuredAPIKey:   "key!@#$%^&*()_+-=[]{}|;':\",./<>?",
			authHeader:         "key!@#$%^&*()_+-=[]{}|;':\",./<>?",
			expectStatusCode:   http.StatusOK,
			expectNextCalled:   true,
			expectErrorMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track whether the next handler was called
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Create the middleware-wrapped handler
			handler := authMiddleware(tt.configuredAPIKey, nextHandler)

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Execute the handler
			handler(rr, req)

			// Assert status code
			assert.Equal(t, tt.expectStatusCode, rr.Code, "Status code should match expected")

			// Assert next handler was called (or not)
			assert.Equal(t, tt.expectNextCalled, nextCalled, "Next handler call status should match expected")

			// Assert error message if expected
			if tt.expectErrorMessage != "" {
				assert.Contains(t, rr.Body.String(), tt.expectErrorMessage, "Response body should contain expected error message")
			}
		})
	}
}

// TestAuthMiddleware_RequestPropagation tests that the request is properly propagated to the next handler
func TestAuthMiddleware_RequestPropagation(t *testing.T) {
	const apiKey = "test-api-key"

	// Create a handler that inspects the request
	var receivedRequest *http.Request
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	// Create the middleware-wrapped handler
	handler := authMiddleware(apiKey, nextHandler)

	// Create a test request with custom headers and path
	req := httptest.NewRequest(http.MethodPost, "/api/test?param=value", nil)
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("X-Request-ID", "req-123")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the handler
	handler(rr, req)

	// Verify the request was passed through correctly
	require.NotNil(t, receivedRequest, "Request should be passed to next handler")
	assert.Equal(t, http.MethodPost, receivedRequest.Method, "Method should be preserved")
	assert.Equal(t, "/api/test", receivedRequest.URL.Path, "Path should be preserved")
	assert.Equal(t, "param=value", receivedRequest.URL.RawQuery, "Query params should be preserved")
	assert.Equal(t, "custom-value", receivedRequest.Header.Get("X-Custom-Header"), "Custom headers should be preserved")
	assert.Equal(t, "req-123", receivedRequest.Header.Get("X-Request-ID"), "Request ID should be preserved")
}

// TestAuthMiddleware_ResponseWriter tests that the response writer is properly propagated
func TestAuthMiddleware_ResponseWriter(t *testing.T) {
	const apiKey = "test-api-key"

	// Create a handler that writes custom response
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Response", "test-value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("custom response body"))
	})

	// Create the middleware-wrapped handler
	handler := authMiddleware(apiKey, nextHandler)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", apiKey)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the handler
	handler(rr, req)

	// Verify the response from the next handler
	assert.Equal(t, http.StatusCreated, rr.Code, "Status code from next handler should be preserved")
	assert.Equal(t, "test-value", rr.Header().Get("X-Custom-Response"), "Custom response headers should be preserved")
	assert.Equal(t, "custom response body", rr.Body.String(), "Response body should be preserved")
}

// TestAuthMiddleware_ConcurrentRequests tests that the middleware is safe for concurrent use
func TestAuthMiddleware_ConcurrentRequests(t *testing.T) {
	const apiKey = "test-api-key"
	const numRequests = 100

	// Create a simple next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create the middleware-wrapped handler
	handler := authMiddleware(apiKey, nextHandler)

	// Create a channel to synchronize goroutines
	done := make(chan bool, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(reqNum int) {
			defer func() { done <- true }()

			// Half with valid keys, half with invalid keys
			authHeader := apiKey
			if reqNum%2 == 1 {
				authHeader = "invalid-key"
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", authHeader)

			rr := httptest.NewRecorder()
			handler(rr, req)

			// Verify expected status code
			if reqNum%2 == 0 {
				assert.Equal(t, http.StatusOK, rr.Code, "Valid requests should succeed")
			} else {
				assert.Equal(t, http.StatusUnauthorized, rr.Code, "Invalid requests should fail")
			}
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

// TestLogRuntimeError tests the logRuntimeError function
func TestLogRuntimeError(t *testing.T) {
	tests := []struct {
		name       string
		errorType  string
		detail     string
		requestID  string
		serverName *string
		path       string
		method     string
	}{
		{
			name:       "BasicError",
			errorType:  "authentication_failed",
			detail:     "missing_auth_header",
			requestID:  "req-123",
			serverName: nil,
			path:       "/api/test",
			method:     "GET",
		},
		{
			name:       "ErrorWithServerName",
			errorType:  "backend_error",
			detail:     "connection_failed",
			requestID:  "req-456",
			serverName: stringPtr("test-server"),
			path:       "/mcp/test",
			method:     "POST",
		},
		{
			name:       "ErrorWithoutRequestID",
			errorType:  "validation_error",
			detail:     "invalid_input",
			requestID:  "",
			serverName: nil,
			path:       "/health",
			method:     "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.requestID != "" {
				req.Header.Set("X-Request-ID", tt.requestID)
			}

			// Call logRuntimeError (it logs to stdout via log.Printf)
			// We can't easily capture the log output in tests, but we can verify it doesn't panic
			assert.NotPanics(t, func() {
				logRuntimeError(tt.errorType, tt.detail, req, tt.serverName)
			}, "logRuntimeError should not panic")
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
