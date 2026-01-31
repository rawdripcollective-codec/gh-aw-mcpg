package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAndValidateSession(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectedID    string
		shouldBeEmpty bool
	}{
		{
			name:          "Valid plain API key",
			authHeader:    "test-session-123",
			expectedID:    "test-session-123",
			shouldBeEmpty: false,
		},
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer my-token-456",
			expectedID:    "my-token-456",
			shouldBeEmpty: false,
		},
		{
			name:          "Empty Authorization header",
			authHeader:    "",
			expectedID:    "",
			shouldBeEmpty: true,
		},
		{
			name:          "Whitespace only header",
			authHeader:    "   ",
			expectedID:    "   ",
			shouldBeEmpty: false,
		},
		{
			name:          "Long session ID",
			authHeader:    "very-long-session-id-with-many-characters-1234567890",
			expectedID:    "very-long-session-id-with-many-characters-1234567890",
			shouldBeEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			sessionID := extractAndValidateSession(req)

			if tt.shouldBeEmpty {
				assert.Empty(t, sessionID, "Expected empty session ID")
			} else {
				assert.Equal(t, tt.expectedID, sessionID, "Session ID mismatch")
			}
		})
	}
}

func TestLogHTTPRequestBody(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		sessionID  string
		backendID  string
		shouldLog  bool
	}{
		{
			name:      "POST request with body and backend",
			method:    "POST",
			body:      `{"method":"initialize"}`,
			sessionID: "session-123",
			backendID: "backend-1",
			shouldLog: true,
		},
		{
			name:      "POST request with body without backend",
			method:    "POST",
			body:      `{"method":"tools/call"}`,
			sessionID: "session-456",
			backendID: "",
			shouldLog: true,
		},
		{
			name:      "GET request (no body logging)",
			method:    "GET",
			body:      "",
			sessionID: "session-789",
			backendID: "backend-2",
			shouldLog: false,
		},
		{
			name:      "POST request with empty body",
			method:    "POST",
			body:      "",
			sessionID: "session-abc",
			backendID: "backend-3",
			shouldLog: false,
		},
		{
			name:      "POST request with nil body",
			method:    "POST",
			body:      "",
			sessionID: "session-def",
			backendID: "",
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/mcp", bytes.NewBufferString(tt.body))
			} else if tt.method == "POST" {
				req = httptest.NewRequest(tt.method, "/mcp", nil)
			} else {
				req = httptest.NewRequest(tt.method, "/mcp", nil)
			}

			// Call the function
			logHTTPRequestBody(req, tt.sessionID, tt.backendID)

			// Verify body can still be read after logging
			if tt.body != "" {
				bodyBytes, err := io.ReadAll(req.Body)
				require.NoError(t, err, "Should be able to read body after logging")
				assert.Equal(t, tt.body, string(bodyBytes), "Body content should be preserved")
			}
		})
	}
}

func TestInjectSessionContext(t *testing.T) {
	tests := []struct {
		name              string
		sessionID         string
		backendID         string
		expectBackendID   bool
	}{
		{
			name:            "Inject session and backend ID (routed mode)",
			sessionID:       "session-123",
			backendID:       "github",
			expectBackendID: true,
		},
		{
			name:            "Inject session ID only (unified mode)",
			sessionID:       "session-456",
			backendID:       "",
			expectBackendID: false,
		},
		{
			name:            "Long session ID with backend",
			sessionID:       "very-long-session-id-1234567890",
			backendID:       "slack",
			expectBackendID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			
			// Inject context
			modifiedReq := injectSessionContext(req, tt.sessionID, tt.backendID)

			// Verify session ID is in context
			sessionIDFromCtx := modifiedReq.Context().Value(SessionIDContextKey)
			require.NotNil(t, sessionIDFromCtx, "Session ID should be in context")
			assert.Equal(t, tt.sessionID, sessionIDFromCtx, "Session ID mismatch")

			// Verify backend ID if expected
			if tt.expectBackendID {
				backendIDFromCtx := modifiedReq.Context().Value(mcp.ContextKey("backend-id"))
				require.NotNil(t, backendIDFromCtx, "Backend ID should be in context")
				assert.Equal(t, tt.backendID, backendIDFromCtx, "Backend ID mismatch")
			} else {
				backendIDFromCtx := modifiedReq.Context().Value(mcp.ContextKey("backend-id"))
				assert.Nil(t, backendIDFromCtx, "Backend ID should not be in context for unified mode")
			}

			// Verify original request is not modified
			originalSessionID := req.Context().Value(SessionIDContextKey)
			assert.Nil(t, originalSessionID, "Original request context should not be modified")
		})
	}
}

func TestInjectSessionContext_PreservesExistingContext(t *testing.T) {
	// Create a request with existing context values
	req := httptest.NewRequest("POST", "/mcp", nil)
	ctx := context.WithValue(req.Context(), "existing-key", "existing-value")
	req = req.WithContext(ctx)

	// Inject session context
	modifiedReq := injectSessionContext(req, "session-123", "backend-1")

	// Verify both values are present
	sessionID := modifiedReq.Context().Value(SessionIDContextKey)
	assert.Equal(t, "session-123", sessionID, "Session ID should be present")

	backendID := modifiedReq.Context().Value(mcp.ContextKey("backend-id"))
	assert.Equal(t, "backend-1", backendID, "Backend ID should be present")

	existingValue := modifiedReq.Context().Value("existing-key")
	assert.Equal(t, "existing-value", existingValue, "Existing context value should be preserved")
}
