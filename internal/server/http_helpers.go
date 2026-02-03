package server

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/github/gh-aw-mcpg/internal/auth"
	"github.com/github/gh-aw-mcpg/internal/logger"
	"github.com/github/gh-aw-mcpg/internal/mcp"
)

var logHelpers = logger.New("server:helpers")

// extractAndValidateSession extracts the session ID from the Authorization header
// and logs connection details. Returns empty string if validation fails.
func extractAndValidateSession(r *http.Request) string {
	logHelpers.Printf("Extracting session from request: remote=%s, path=%s", r.RemoteAddr, r.URL.Path)

	authHeader := r.Header.Get("Authorization")
	sessionID := auth.ExtractSessionID(authHeader)

	if sessionID == "" {
		logHelpers.Printf("Session extraction failed: no Authorization header, remote=%s", r.RemoteAddr)
		logger.LogError("client", "Rejected MCP client connection: no Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
		log.Printf("[%s] %s %s - REJECTED: No Authorization header", r.RemoteAddr, r.Method, r.URL.Path)
		return ""
	}

	logHelpers.Printf("Session extracted successfully: sessionID=%s, remote=%s", sessionID, r.RemoteAddr)
	return sessionID
}

// logHTTPRequestBody logs the request body for debugging purposes.
// It reads the body, logs it, and restores it so it can be read again.
// The backendID parameter is optional and can be empty for unified mode.
func logHTTPRequestBody(r *http.Request, sessionID, backendID string) {
	logHelpers.Printf("Checking request body: method=%s, hasBody=%v, sessionID=%s", r.Method, r.Body != nil, sessionID)

	if r.Method != "POST" || r.Body == nil {
		logHelpers.Printf("Skipping body logging: not a POST request or no body present")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 {
		logHelpers.Printf("Body read failed or empty: err=%v, size=%d", err, len(bodyBytes))
		return
	}

	logHelpers.Printf("Request body read: size=%d bytes, sessionID=%s, backendID=%s", len(bodyBytes), sessionID, backendID)

	// Log with backend context if provided (routed mode)
	if backendID != "" {
		logger.LogDebug("client", "MCP client request body, backend=%s, body=%s", backendID, string(bodyBytes))
	} else {
		logger.LogDebug("client", "MCP request body, session=%s, body=%s", sessionID, string(bodyBytes))
	}
	log.Printf("Request body: %s", string(bodyBytes))

	// Restore body for subsequent reads
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	logHelpers.Print("Request body restored for subsequent reads")
}

// injectSessionContext stores the session ID and optional backend ID into the request context.
// If backendID is empty, only session ID is injected (unified mode).
// Returns the modified request with updated context.
func injectSessionContext(r *http.Request, sessionID, backendID string) *http.Request {
	logHelpers.Printf("Injecting session context: sessionID=%s, backendID=%s", sessionID, backendID)

	ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)

	if backendID != "" {
		logHelpers.Printf("Adding backend ID to context: backendID=%s", backendID)
		ctx = context.WithValue(ctx, mcp.ContextKey("backend-id"), backendID)
	}

	logHelpers.Print("Session context injected successfully")
	return r.WithContext(ctx)
}
