// Package logger provides structured logging for the MCP Gateway.
//
// This file contains helper functions for processing RPC message payloads.
//
// Functions in this file:
//
// - truncateAndSanitize: Combines secret sanitization with length truncation
// - extractEssentialFields: Extracts key JSON-RPC fields for compact logging
// - getMapKeys: Utility for extracting map keys without values
// - isEffectivelyEmpty: Checks if data is effectively empty (e.g., only params: null)
// - ExtractErrorMessage: Extracts clean error messages from log lines
//
// These helpers are used by the RPC logging system to safely and efficiently
// process message payloads before logging them.
package logger

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/github/gh-aw-mcpg/internal/logger/sanitize"
)

// Pre-compiled regexes for performance (avoid recompiling in hot paths).
var (
	// Timestamp patterns for log cleanup
	// Pattern 1: ISO 8601 with T or space separator (e.g., "2024-01-01T12:00:00.123Z " or "2024-01-01 12:00:00 ").
	timestampPattern1 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:\d{2}|Z)?\s*`)
	// Pattern 2: Bracketed date-time (e.g., "[2024-01-01 12:00:00] ").
	timestampPattern2 = regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\]\s*`)
	// Pattern 3: Bracketed time only (e.g., "[12:00:00] ").
	timestampPattern3 = regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\]\s+`)
	// Pattern 4: Time only with optional milliseconds (e.g., "12:00:00.123 ").
	timestampPattern4 = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(\.\d+)?\s+`)

	// Log level pattern for message cleanup (case-insensitive).
	logLevelPattern = regexp.MustCompile(`(?i)^\[?(ERROR|WARNING|WARN|INFO|DEBUG)\]?\s*[:-]?\s*`)
)

// truncateAndSanitize truncates the payload to max length and sanitizes secrets
func truncateAndSanitize(payload string, maxLength int) string {
	// First sanitize secrets
	sanitized := sanitize.SanitizeString(payload)

	// Then truncate if needed
	if len(sanitized) > maxLength {
		return sanitized[:maxLength] + "..."
	}
	return sanitized
}

// extractEssentialFields extracts key fields from the payload for logging
func extractEssentialFields(payload []byte) map[string]interface{} {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil
	}

	// Extract only essential fields
	essential := make(map[string]interface{})

	// Common JSON-RPC fields
	if method, ok := data["method"].(string); ok {
		essential["method"] = method
	}
	if id, ok := data["id"]; ok {
		essential["id"] = id
	}
	if jsonrpc, ok := data["jsonrpc"].(string); ok {
		essential["jsonrpc"] = jsonrpc
	}

	// For responses, include error info
	if errData, ok := data["error"]; ok {
		essential["error"] = errData
	}

	// For requests, include params summary (but not full params)
	if params, ok := data["params"]; ok {
		if paramsMap, ok := params.(map[string]interface{}); ok {
			// Include param count and keys, but not values
			essential["params_keys"] = getMapKeys(paramsMap)
		}
	}

	return essential
}

// getMapKeys returns the keys of a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isEffectivelyEmpty checks if the data is effectively empty (only contains params: null)
func isEffectivelyEmpty(data map[string]interface{}) bool {
	// If empty, it's empty
	if len(data) == 0 {
		return true
	}

	// If only one field and it's "params" with null value, it's empty
	if len(data) == 1 {
		if params, ok := data["params"]; ok && params == nil {
			return true
		}
	}

	return false
}

// ExtractErrorMessage extracts a clean error message from a log line.
// It removes timestamps, log level prefixes, and other common noise.
// If the message is longer than 200 characters, it will be truncated.
func ExtractErrorMessage(line string) string {
	// Remove common timestamp patterns using pre-compiled regexes
	cleanedLine := line
	cleanedLine = timestampPattern1.ReplaceAllString(cleanedLine, "")
	cleanedLine = timestampPattern2.ReplaceAllString(cleanedLine, "")
	cleanedLine = timestampPattern3.ReplaceAllString(cleanedLine, "")
	cleanedLine = timestampPattern4.ReplaceAllString(cleanedLine, "")

	// Remove common log level prefixes using pre-compiled regex
	cleanedLine = logLevelPattern.ReplaceAllString(cleanedLine, "")

	// Trim whitespace
	cleanedLine = strings.TrimSpace(cleanedLine)

	// If the line is too long (>200 chars), truncate it
	if len(cleanedLine) > 200 {
		cleanedLine = cleanedLine[:197] + "..."
	}

	return cleanedLine
}
