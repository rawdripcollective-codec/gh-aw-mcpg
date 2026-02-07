package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/github/gh-aw-mcpg/internal/logger"
)

// getSessionID extracts the MCP session ID from the context
func (us *UnifiedServer) getSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value(SessionIDContextKey).(string); ok && sessionID != "" {
		log.Printf("Extracted session ID from context: %s", sessionID)
		return sessionID
	}
	// No session ID in context - this happens before the SDK assigns one
	// For now, use "default" as a placeholder for single-client scenarios
	// In production multi-agent scenarios, the SDK will provide session IDs after initialize
	log.Printf("No session ID in context, using 'default' (this is normal before SDK session is established)")
	return "default"
}

// ensureSessionDirectory creates the session subdirectory in the payload directory if it doesn't exist
func (us *UnifiedServer) ensureSessionDirectory(sessionID string) error {
	sessionDir := filepath.Join(us.payloadDir, sessionID)

	// Check if directory already exists
	if _, err := os.Stat(sessionDir); err == nil {
		// Directory already exists
		logUnified.Printf("Session directory already exists: %s", sessionDir)
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred while checking
		return fmt.Errorf("failed to check session directory: %w", err)
	}

	// Directory doesn't exist, create it with world-readable permissions (for agent access)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	logUnified.Printf("Created session directory: %s", sessionDir)
	log.Printf("Created payload directory for session: %s", sessionID)
	return nil
}

// requireSession checks that a session has been initialized for this request
// When DIFC is disabled (default), automatically creates a session if one doesn't exist
func (us *UnifiedServer) requireSession(ctx context.Context) error {
	sessionID := us.getSessionID(ctx)
	log.Printf("Checking session for ID: %s", sessionID)

	// If DIFC is disabled (default), use double-checked locking to auto-create session
	if !us.enableDIFC {
		us.sessionMu.RLock()
		session := us.sessions[sessionID]
		us.sessionMu.RUnlock()

		if session == nil {
			// Need to create session - acquire write lock
			us.sessionMu.Lock()
			// Double-check after acquiring write lock to avoid race condition
			if us.sessions[sessionID] == nil {
				log.Printf("DIFC disabled: auto-creating session for ID: %s", sessionID)
				us.sessions[sessionID] = NewSession(sessionID, "")
				log.Printf("Session auto-created for ID: %s", sessionID)

				// Ensure session directory exists in payload mount point
				// This is done after releasing the lock to avoid holding it during I/O
				us.sessionMu.Unlock()
				if err := us.ensureSessionDirectory(sessionID); err != nil {
					logger.LogWarn("client", "Failed to create session directory for session=%s: %v", sessionID, err)
					// Don't fail - payloads will attempt to create the directory when needed
				}
				return nil
			}
			us.sessionMu.Unlock()
		}
		return nil
	}

	// DIFC is enabled - require explicit session initialization
	us.sessionMu.RLock()
	session := us.sessions[sessionID]
	us.sessionMu.RUnlock()

	if session == nil {
		log.Printf("Session not found for ID: %s. Available sessions: %v", sessionID, us.getSessionKeys())
		return fmt.Errorf("sys___init must be called before any other tool calls")
	}

	log.Printf("Session validated for ID: %s", sessionID)
	return nil
}

// getSessionKeys returns a list of active session IDs for debugging
func (us *UnifiedServer) getSessionKeys() []string {
	us.sessionMu.RLock()
	defer us.sessionMu.RUnlock()

	keys := make([]string, 0, len(us.sessions))
	for k := range us.sessions {
		keys = append(keys, k)
	}
	return keys
}
