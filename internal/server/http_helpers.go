package server

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/githubnext/gh-aw-mcpg/internal/auth"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
)

// extractAndValidateSession extracts the session ID from the Authorization header
// and logs connection details. Returns empty string if validation fails.
func extractAndValidateSession(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	sessionID := auth.ExtractSessionID(authHeader)

	if sessionID == "" {
		logger.LogError("client", "Rejected MCP client connection: no Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
		log.Printf("[%s] %s %s - REJECTED: No Authorization header", r.RemoteAddr, r.Method, r.URL.Path)
		return ""
	}

	return sessionID
}

// logHTTPRequestBody logs the request body for debugging purposes.
// It reads the body, logs it, and restores it so it can be read again.
// The backendID parameter is optional and can be empty for unified mode.
func logHTTPRequestBody(r *http.Request, sessionID, backendID string) {
	if r.Method != "POST" || r.Body == nil {
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 {
		return
	}

	// Log with backend context if provided (routed mode)
	if backendID != "" {
		logger.LogDebug("client", "MCP client request body, backend=%s, body=%s", backendID, string(bodyBytes))
	} else {
		logger.LogDebug("client", "MCP request body, session=%s, body=%s", sessionID, string(bodyBytes))
	}
	log.Printf("Request body: %s", string(bodyBytes))

	// Restore body for subsequent reads
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

// injectSessionContext stores the session ID and optional backend ID into the request context.
// If backendID is empty, only session ID is injected (unified mode).
// Returns the modified request with updated context.
func injectSessionContext(r *http.Request, sessionID, backendID string) *http.Request {
	ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
	
	if backendID != "" {
		ctx = context.WithValue(ctx, mcp.ContextKey("backend-id"), backendID)
	}
	
	return r.WithContext(ctx)
}
