package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/itchyny/gojq"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMiddleware = logger.New("middleware:jqschema")

// jqSchemaFilter is the jq filter that transforms JSON to schema
// This is the same logic as in gh-aw shared/jqschema.md
const jqSchemaFilter = `
def walk(f):
  . as $in |
  if type == "object" then
    reduce keys[] as $k ({}; . + {($k): ($in[$k] | walk(f))})
  elif type == "array" then
    if length == 0 then [] else [.[0] | walk(f)] end
  else
    type
  end;
walk(.)
`

// Pre-compiled jq query code for performance
// This is compiled once at package initialization and reused for all requests
var (
	jqSchemaCode       *gojq.Code
	jqSchemaCompileErr error
)

// init compiles the jq schema filter at startup for better performance
// Following gojq best practices: compile once, run many times
func init() {
	query, err := gojq.Parse(jqSchemaFilter)
	if err != nil {
		jqSchemaCompileErr = fmt.Errorf("failed to parse jq schema filter: %w", err)
		logMiddleware.Printf("Failed to parse jq schema filter at init: %v", err)
		return
	}

	jqSchemaCode, jqSchemaCompileErr = gojq.Compile(query)
	if jqSchemaCompileErr != nil {
		logMiddleware.Printf("Failed to compile jq schema filter at init: %v", jqSchemaCompileErr)
		return
	}

	logMiddleware.Printf("Successfully compiled jq schema filter at init")
}

// generateRandomID generates a random ID for payload storage
func generateRandomID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("fallback-%d", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}

// applyJqSchema applies the jq schema transformation to JSON data
// Uses pre-compiled query code for better performance (3-10x faster than parsing on each request)
// Accepts a context for timeout and cancellation support
func applyJqSchema(ctx context.Context, jsonData interface{}) (string, error) {
	// Check if compilation succeeded at init time
	if jqSchemaCompileErr != nil {
		return "", jqSchemaCompileErr
	}

	// Run the pre-compiled query with context support (much faster than Parse+Run)
	iter := jqSchemaCode.RunWithContext(ctx, jsonData)
	v, ok := iter.Next()
	if !ok {
		return "", fmt.Errorf("jq schema filter returned no results")
	}

	// Check for errors with type-specific handling
	if err, ok := v.(error); ok {
		// Check for HaltError - a clean halt with exit code
		if haltErr, ok := err.(*gojq.HaltError); ok {
			// HaltError with nil value means clean halt (not an error)
			if haltErr.Value() == nil {
				return "", fmt.Errorf("jq schema filter halted cleanly with no output")
			}
			// HaltError with non-nil value is an actual error
			return "", fmt.Errorf("jq schema filter halted with error (exit code %d): %w", haltErr.ExitCode(), err)
		}
		// Generic error case
		return "", fmt.Errorf("jq schema filter error: %w", err)
	}

	// Convert result to JSON
	schemaJSON, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema result: %w", err)
	}

	return string(schemaJSON), nil
}

// savePayload saves the payload to disk and returns the file path
// The file is saved to {baseDir}/{sessionID}/{queryID}/payload.json
func savePayload(baseDir, sessionID, queryID string, payload []byte) (string, error) {
	// Create directory structure: {baseDir}/{sessionID}/{queryID}
	dir := filepath.Join(baseDir, sessionID, queryID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create payload directory: %w", err)
	}

	// Save payload to file with restrictive permissions (owner read/write only)
	filePath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(filePath, payload, 0600); err != nil {
		return "", fmt.Errorf("failed to write payload file: %w", err)
	}

	return filePath, nil
}

// WrapToolHandler wraps a tool handler with jqschema middleware
// This middleware:
// 1. Generates a random ID for the query
// 2. Extracts session ID from context (or uses "default")
// 3. Saves the response payload to {baseDir}/{sessionID}/{queryID}/payload.json
// 4. Returns first 500 chars of payload + jq inferred schema
func WrapToolHandler(
	handler func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error),
	toolName string,
	baseDir string,
	getSessionID func(context.Context) string,
) func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error) {
	return func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		// Generate random query ID
		queryID := generateRandomID()

		// Get session ID from context
		sessionID := getSessionID(ctx)
		if sessionID == "" {
			sessionID = "default"
		}

		logMiddleware.Printf("Processing tool call: tool=%s, queryID=%s, sessionID=%s", toolName, queryID, sessionID)

		// Call the original handler
		result, data, err := handler(ctx, req, args)
		if err != nil {
			logMiddleware.Printf("Tool call failed: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, err)
			return result, data, err
		}

		// Only process successful results with data
		if result == nil || result.IsError || data == nil {
			return result, data, err
		}

		// Marshal the response data to JSON
		payloadJSON, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			logMiddleware.Printf("Failed to marshal response: tool=%s, queryID=%s, error=%v", toolName, queryID, marshalErr)
			return result, data, err
		}

		// Save the payload
		filePath, saveErr := savePayload(baseDir, sessionID, queryID, payloadJSON)
		if saveErr != nil {
			logMiddleware.Printf("Failed to save payload: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, saveErr)
			// Continue even if save fails - don't break the tool call
		} else {
			logMiddleware.Printf("Saved payload: tool=%s, queryID=%s, sessionID=%s, path=%s, size=%d bytes",
				toolName, queryID, sessionID, filePath, len(payloadJSON))
		}

		// Apply jq schema transformation
		var schemaJSON string
		if schemaErr := func() error {
			// Unmarshal to interface{} for jq processing
			var jsonData interface{}
			if err := json.Unmarshal(payloadJSON, &jsonData); err != nil {
				return fmt.Errorf("failed to unmarshal for schema: %w", err)
			}

			schema, err := applyJqSchema(ctx, jsonData)
			if err != nil {
				return err
			}
			schemaJSON = schema
			return nil
		}(); schemaErr != nil {
			logMiddleware.Printf("Failed to apply jq schema: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, schemaErr)
			// Continue with original response if schema extraction fails
			return result, data, err
		}

		// Build the transformed response: first 500 chars + schema
		payloadStr := string(payloadJSON)
		var preview string
		if len(payloadStr) > 500 {
			preview = payloadStr[:500] + "..."
		} else {
			preview = payloadStr
		}

		// Create rewritten response
		rewrittenResponse := map[string]interface{}{
			"queryID":      queryID,
			"payloadPath":  filePath,
			"preview":      preview,
			"schema":       schemaJSON,
			"originalSize": len(payloadJSON),
			"truncated":    len(payloadStr) > 500,
		}

		logMiddleware.Printf("Rewritten response: tool=%s, queryID=%s, sessionID=%s, originalSize=%d, truncated=%v",
			toolName, queryID, sessionID, len(payloadJSON), len(payloadStr) > 500)

		// Parse the schema JSON string back to an object for cleaner display
		var schemaObj interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err == nil {
			rewrittenResponse["schema"] = schemaObj
		}

		return result, rewrittenResponse, nil
	}
}

// ShouldApplyMiddleware determines if the middleware should be applied to a tool
// Currently applies to all tools, but can be configured to filter specific tools
func ShouldApplyMiddleware(toolName string) bool {
	// Apply to all tools except sys tools
	return !strings.HasPrefix(toolName, "sys___")
}
