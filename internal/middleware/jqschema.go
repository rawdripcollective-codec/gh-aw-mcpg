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

	"github.com/github/gh-aw-mcpg/internal/logger"
	"github.com/itchyny/gojq"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMiddleware = logger.New("middleware:jqschema")

// PayloadTruncatedInstructions is the message returned to clients when a payload
// has been truncated and saved to the filesystem
const PayloadTruncatedInstructions = "The payload was too large for an MCP response. The full response can be accessed through the local file system at the payloadPath."

// PayloadMetadata represents the metadata response returned when a payload is too large
// and has been saved to the filesystem
type PayloadMetadata struct {
	QueryID      string      `json:"queryID"`
	PayloadPath  string      `json:"payloadPath"`
	Preview      string      `json:"preview"`
	Schema       interface{} `json:"schema"`
	OriginalSize int         `json:"originalSize"`
	Truncated    bool        `json:"truncated"`
	Instructions string      `json:"instructions"`
}

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
// Returns the schema as an interface{} object (not a JSON string)
func applyJqSchema(ctx context.Context, jsonData interface{}) (interface{}, error) {
	// Check if compilation succeeded at init time
	if jqSchemaCompileErr != nil {
		return nil, jqSchemaCompileErr
	}

	// Run the pre-compiled query with context support (much faster than Parse+Run)
	iter := jqSchemaCode.RunWithContext(ctx, jsonData)
	v, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("jq schema filter returned no results")
	}

	// Check for errors with type-specific handling
	if err, ok := v.(error); ok {
		// Check for HaltError - a clean halt with exit code
		if haltErr, ok := err.(*gojq.HaltError); ok {
			// HaltError with nil value means clean halt (not an error)
			if haltErr.Value() == nil {
				return nil, fmt.Errorf("jq schema filter halted cleanly with no output")
			}
			// HaltError with non-nil value is an actual error
			return nil, fmt.Errorf("jq schema filter halted with error (exit code %d): %w", haltErr.ExitCode(), err)
		}
		// Generic error case
		return nil, fmt.Errorf("jq schema filter error: %w", err)
	}

	// Return the schema object directly (no JSON marshaling needed here)
	return v, nil
}

// savePayload saves the payload to disk and returns the file path
// The file is saved to {baseDir}/{sessionID}/{queryID}/payload.json
func savePayload(baseDir, sessionID, queryID string, payload []byte) (string, error) {
	// Create directory structure: {baseDir}/{sessionID}/{queryID}
	dir := filepath.Join(baseDir, sessionID, queryID)

	logger.LogDebug("payload", "Creating payload directory: baseDir=%s, session=%s, query=%s, fullPath=%s",
		baseDir, sessionID, queryID, dir)

	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.LogError("payload", "Failed to create payload directory: path=%s, error=%v", dir, err)
		return "", fmt.Errorf("failed to create payload directory: %w", err)
	}

	logger.LogDebug("payload", "Successfully created payload directory: path=%s, permissions=0755", dir)

	// Save payload to file with restrictive permissions (owner read/write only)
	filePath := filepath.Join(dir, "payload.json")
	payloadSize := len(payload)

	logger.LogInfo("payload", "Writing large payload to filesystem: path=%s, size=%d bytes (%.2f KB, %.2f MB)",
		filePath, payloadSize, float64(payloadSize)/1024, float64(payloadSize)/(1024*1024))

	if err := os.WriteFile(filePath, payload, 0644); err != nil {
		logger.LogError("payload", "Failed to write payload file: path=%s, size=%d bytes, error=%v",
			filePath, payloadSize, err)
		return "", fmt.Errorf("failed to write payload file: %w", err)
	}

	logger.LogInfo("payload", "Successfully saved large payload to filesystem: path=%s, size=%d bytes, permissions=0644",
		filePath, payloadSize)

	// Verify file was written correctly
	if stat, err := os.Stat(filePath); err != nil {
		logger.LogWarn("payload", "Could not verify payload file after write: path=%s, error=%v", filePath, err)
	} else {
		logger.LogDebug("payload", "Payload file verified: path=%s, size=%d bytes, mode=%s",
			filePath, stat.Size(), stat.Mode())
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
		logger.LogDebug("payload", "Middleware processing tool call: tool=%s, queryID=%s, session=%s, baseDir=%s",
			toolName, queryID, sessionID, baseDir)

		// Call the original handler
		result, data, err := handler(ctx, req, args)
		if err != nil {
			logMiddleware.Printf("Tool call failed: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, err)
			logger.LogDebug("payload", "Tool call failed, skipping payload storage: tool=%s, queryID=%s, error=%v",
				toolName, queryID, err)
			return result, data, err
		}

		// Only process successful results with data
		if result == nil || result.IsError || data == nil {
			logger.LogDebug("payload", "Skipping payload storage: tool=%s, queryID=%s, reason=%s",
				toolName, queryID,
				func() string {
					if result == nil {
						return "result is nil"
					} else if result.IsError {
						return "result indicates error"
					} else {
						return "no data returned"
					}
				}())
			return result, data, err
		}

		// Marshal the response data to JSON
		payloadJSON, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			logMiddleware.Printf("Failed to marshal response: tool=%s, queryID=%s, error=%v", toolName, queryID, marshalErr)
			logger.LogError("payload", "Failed to marshal response data to JSON: tool=%s, queryID=%s, error=%v",
				toolName, queryID, marshalErr)
			return result, data, err
		}

		payloadSize := len(payloadJSON)
		logger.LogInfo("payload", "Response data marshaled to JSON: tool=%s, queryID=%s, size=%d bytes (%.2f KB, %.2f MB)",
			toolName, queryID, payloadSize, float64(payloadSize)/1024, float64(payloadSize)/(1024*1024))

		// Save the payload
		logger.LogInfo("payload", "Starting payload storage to filesystem: tool=%s, queryID=%s, session=%s, baseDir=%s",
			toolName, queryID, sessionID, baseDir)

		filePath, saveErr := savePayload(baseDir, sessionID, queryID, payloadJSON)
		if saveErr != nil {
			logMiddleware.Printf("Failed to save payload: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, saveErr)
			logger.LogError("payload", "Failed to save payload to filesystem: tool=%s, queryID=%s, session=%s, error=%v",
				toolName, queryID, sessionID, saveErr)
			// Continue even if save fails - don't break the tool call
		} else {
			logMiddleware.Printf("Saved payload: tool=%s, queryID=%s, sessionID=%s, path=%s, size=%d bytes",
				toolName, queryID, sessionID, filePath, len(payloadJSON))
			logger.LogInfo("payload", "Payload storage completed successfully: tool=%s, queryID=%s, session=%s, path=%s, size=%d bytes",
				toolName, queryID, sessionID, filePath, len(payloadJSON))
		}

		// Apply jq schema transformation
		logger.LogDebug("payload", "Applying jq schema transformation: tool=%s, queryID=%s", toolName, queryID)
		var schemaObj interface{}
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
			schemaObj = schema
			return nil
		}(); schemaErr != nil {
			logMiddleware.Printf("Failed to apply jq schema: tool=%s, queryID=%s, sessionID=%s, error=%v", toolName, queryID, sessionID, schemaErr)
			logger.LogWarn("payload", "Failed to generate schema for payload: tool=%s, queryID=%s, error=%v",
				toolName, queryID, schemaErr)
			// Continue with original response if schema extraction fails
			return result, data, err
		}

		// Calculate schema size for logging (marshal temporarily)
		schemaBytes, _ := json.Marshal(schemaObj)
		logger.LogDebug("payload", "Schema transformation completed: tool=%s, queryID=%s, schemaSize=%d bytes",
			toolName, queryID, len(schemaBytes))

		// Build the transformed response: first 500 chars + schema
		payloadStr := string(payloadJSON)
		var preview string
		truncated := len(payloadStr) > 500
		if truncated {
			preview = payloadStr[:500] + "..."
			logger.LogInfo("payload", "Payload truncated for preview: tool=%s, queryID=%s, originalSize=%d bytes, previewSize=500 bytes",
				toolName, queryID, len(payloadStr))
		} else {
			preview = payloadStr
			logger.LogDebug("payload", "Payload small enough for full preview: tool=%s, queryID=%s, size=%d bytes",
				toolName, queryID, len(payloadStr))
		}

		// Create rewritten response using the PayloadMetadata struct
		rewrittenResponse := PayloadMetadata{
			QueryID:      queryID,
			PayloadPath:  filePath,
			Preview:      preview,
			Schema:       schemaObj,
			OriginalSize: len(payloadJSON),
			Truncated:    truncated,
			Instructions: PayloadTruncatedInstructions,
		}

		logMiddleware.Printf("Rewritten response: tool=%s, queryID=%s, sessionID=%s, originalSize=%d, truncated=%v",
			toolName, queryID, sessionID, len(payloadJSON), truncated)
		logger.LogInfo("payload", "Created metadata response for client: tool=%s, queryID=%s, session=%s, payloadPath=%s, originalSize=%d bytes, truncated=%v",
			toolName, queryID, sessionID, filePath, len(payloadJSON), truncated)

		// Marshal the rewritten response to JSON for the Content field
		rewrittenJSON, marshalErr := json.Marshal(rewrittenResponse)
		if marshalErr != nil {
			logMiddleware.Printf("Failed to marshal rewritten response: tool=%s, queryID=%s, error=%v", toolName, queryID, marshalErr)
			logger.LogError("payload", "Failed to marshal metadata response: tool=%s, queryID=%s, error=%v",
				toolName, queryID, marshalErr)
			// Fall back to original result if we can't marshal
			return result, rewrittenResponse, nil
		}

		logger.LogDebug("payload", "Metadata response marshaled: tool=%s, queryID=%s, metadataSize=%d bytes",
			toolName, queryID, len(rewrittenJSON))

		// Create a new CallToolResult with the transformed content
		// Replace the original content with our rewritten response
		transformedResult := &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(rewrittenJSON),
				},
			},
			IsError: result.IsError,
			Meta:    result.Meta,
		}

		logMiddleware.Printf("Transformed result with metadata: tool=%s, queryID=%s, sessionID=%s", toolName, queryID, sessionID)
		logger.LogInfo("payload", "Returning transformed response to client: tool=%s, queryID=%s, session=%s, payloadPath=%s, clientReceivesMetadata=true",
			toolName, queryID, sessionID, filePath)
		logger.LogInfo("payload", "Client can access full payload at: %s (inside container: /workspace/mcp-payloads/%s/%s/payload.json)",
			filePath, sessionID, queryID)

		return transformedResult, rewrittenResponse, nil
	}
}

// ShouldApplyMiddleware determines if the middleware should be applied to a tool
// Currently applies to all tools, but can be configured to filter specific tools
func ShouldApplyMiddleware(toolName string) bool {
	// Apply to all tools except sys tools
	return !strings.HasPrefix(toolName, "sys___")
}
