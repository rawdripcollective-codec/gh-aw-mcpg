package logger

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ISO 8601 timestamp with T separator and Z",
			input:    "2024-01-01T12:00:00.123Z Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "ISO 8601 timestamp with T separator and timezone offset",
			input:    "2024-01-01T12:00:00.123+00:00 Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Date-time with space separator",
			input:    "2024-01-01 12:00:00 Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Date-time with space separator and milliseconds",
			input:    "2024-01-01 12:00:00.456 Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Bracketed date-time",
			input:    "[2024-01-01 12:00:00] Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Bracketed time only",
			input:    "[12:00:00] Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Time only with milliseconds",
			input:    "12:00:00.123 Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Time only without milliseconds",
			input:    "12:00:00 Error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "ERROR prefix with colon",
			input:    "ERROR: connection failed",
			expected: "connection failed",
		},
		{
			name:     "ERROR prefix without colon",
			input:    "ERROR connection failed",
			expected: "connection failed",
		},
		{
			name:     "Bracketed ERROR prefix",
			input:    "[ERROR] connection failed",
			expected: "connection failed",
		},
		{
			name:     "Bracketed ERROR prefix with colon",
			input:    "[ERROR]: connection failed",
			expected: "connection failed",
		},
		{
			name:     "WARNING prefix",
			input:    "WARNING: disk space low",
			expected: "disk space low",
		},
		{
			name:     "WARN prefix",
			input:    "WARN: deprecated API used",
			expected: "deprecated API used",
		},
		{
			name:     "INFO prefix",
			input:    "INFO: service started",
			expected: "service started",
		},
		{
			name:     "DEBUG prefix",
			input:    "DEBUG: processing request",
			expected: "processing request",
		},
		{
			name:     "Case insensitive log level",
			input:    "error: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Combined timestamp and log level",
			input:    "2024-01-01 12:00:00 ERROR: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Combined ISO timestamp with Z and log level",
			input:    "2024-01-01T12:00:00Z ERROR: connection failed",
			expected: "connection failed",
		},
		{
			name:     "Multiple timestamps - only first is removed",
			input:    "[12:00:00] 2024-01-01 12:00:00 ERROR: connection failed",
			expected: "2024-01-01 12:00:00 ERROR: connection failed",
		},
		{
			name:     "No timestamp or log level",
			input:    "connection failed",
			expected: "connection failed",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "Truncation at 200 chars",
			input:    "ERROR: " + strings.Repeat("a", 250),
			expected: strings.Repeat("a", 197) + "...",
		},
		{
			name:     "Exactly 200 chars - no truncation",
			input:    "ERROR: " + strings.Repeat("a", 193),
			expected: strings.Repeat("a", 193),
		},
		{
			name:     "Real world example from metrics.go",
			input:    "2024-01-15 14:30:22 ERROR: Failed to connect to database",
			expected: "Failed to connect to database",
		},
		{
			name:     "Real world example from copilot_agent.go",
			input:    "2024-01-15T14:30:22.123Z ERROR: API request failed",
			expected: "API request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractErrorMessage(tt.input)
			assert.Equal(t, tt.expected, result, "ExtractErrorMessage(%q)", tt.input)
		})
	}
}

func BenchmarkExtractErrorMessage(b *testing.B) {
	testLine := "2024-01-01T12:00:00.123Z ERROR: connection failed to remote server"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractErrorMessage(testLine)
	}
}

func BenchmarkExtractErrorMessageLong(b *testing.B) {
	testLine := "2024-01-01T12:00:00.123Z ERROR: " + strings.Repeat("very long error message ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractErrorMessage(testLine)
	}
}

// TestTruncateAndSanitize tests the truncateAndSanitize function
func TestTruncateAndSanitize(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		maxLength int
		want      string
	}{
		{
			name:      "short string no truncation",
			payload:   "Hello, World!",
			maxLength: 50,
			want:      "Hello, World!",
		},
		{
			name:      "exact max length",
			payload:   "Hello",
			maxLength: 5,
			want:      "Hello",
		},
		{
			name:      "truncation needed",
			payload:   "This is a very long string that needs to be truncated",
			maxLength: 20,
			want:      "This is a very long ...",
		},
		{
			name:      "empty string",
			payload:   "",
			maxLength: 10,
			want:      "",
		},
		{
			name:      "zero max length",
			payload:   "test",
			maxLength: 0,
			want:      "...",
		},
		{
			name:      "sanitize secrets - GitHub token",
			payload:   "token: ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			maxLength: 100,
			want:      "token=[REDACTED]",
		},
		{
			name:      "sanitize and truncate",
			payload:   "auth bearer ghp_1234567890abcdefghijklmnopqrstuvwxyz " + strings.Repeat("x", 100),
			maxLength: 50,
			want:      "auth bearer [REDACTED] " + strings.Repeat("x", 27) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateAndSanitize(tt.payload, tt.maxLength)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestExtractEssentialFields tests the extractEssentialFields function
func TestExtractEssentialFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    map[string]interface{}
	}{
		{
			name:    "valid JSON-RPC request",
			payload: `{"jsonrpc":"2.0","method":"tools/list","id":1,"params":{"name":"test"}}`,
			want: map[string]interface{}{
				"jsonrpc":     "2.0",
				"method":      "tools/list",
				"id":          float64(1),
				"params_keys": []string{"name"},
			},
		},
		{
			name:    "JSON-RPC response with error",
			payload: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`,
			want: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"error": map[string]interface{}{
					"code":    float64(-32600),
					"message": "Invalid Request",
				},
			},
		},
		{
			name:    "minimal request with method only",
			payload: `{"method":"initialize"}`,
			want: map[string]interface{}{
				"method": "initialize",
			},
		},
		{
			name:    "request with null params",
			payload: `{"jsonrpc":"2.0","method":"test","id":2,"params":null}`,
			want: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "test",
				"id":      float64(2),
			},
		},
		{
			name:    "request with complex params",
			payload: `{"method":"call","params":{"arg1":"val1","arg2":"val2","arg3":"val3"}}`,
			want: map[string]interface{}{
				"method":      "call",
				"params_keys": []string{"arg1", "arg2", "arg3"},
			},
		},
		{
			name:    "invalid JSON",
			payload: `{invalid json}`,
			want:    nil,
		},
		{
			name:    "empty JSON object",
			payload: `{}`,
			want:    map[string]interface{}{},
		},
		{
			name:    "empty string",
			payload: ``,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEssentialFields([]byte(tt.payload))

			if tt.want == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.want["jsonrpc"], result["jsonrpc"])
			assert.Equal(t, tt.want["method"], result["method"])
			assert.Equal(t, tt.want["id"], result["id"])
			assert.Equal(t, tt.want["error"], result["error"])

			// Special handling for params_keys since order may vary
			if expectedKeys, ok := tt.want["params_keys"].([]string); ok {
				actualKeys, ok := result["params_keys"].([]string)
				require.True(t, ok, "params_keys should be []string")
				assert.ElementsMatch(t, expectedKeys, actualKeys)
			}
		})
	}
}

// TestGetMapKeys tests the getMapKeys function
func TestGetMapKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want []string
	}{
		{
			name: "normal map",
			m: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			want: []string{"key1", "key2", "key3"},
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: []string{},
		},
		{
			name: "single key",
			m: map[string]interface{}{
				"only": "value",
			},
			want: []string{"only"},
		},
		{
			name: "nil values",
			m: map[string]interface{}{
				"null1": nil,
				"null2": nil,
			},
			want: []string{"null1", "null2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMapKeys(tt.m)
			assert.ElementsMatch(t, tt.want, result, "keys should match regardless of order")
			assert.Len(t, result, len(tt.want), "should have correct number of keys")
		})
	}
}

// TestIsEffectivelyEmpty tests the isEffectivelyEmpty function
func TestIsEffectivelyEmpty(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want bool
	}{
		{
			name: "truly empty map",
			data: map[string]interface{}{},
			want: true,
		},
		{
			name: "only params with null value",
			data: map[string]interface{}{
				"params": nil,
			},
			want: true,
		},
		{
			name: "params with non-null value",
			data: map[string]interface{}{
				"params": "some value",
			},
			want: false,
		},
		{
			name: "multiple fields including params",
			data: map[string]interface{}{
				"params": nil,
				"method": "test",
			},
			want: false,
		},
		{
			name: "single non-params field",
			data: map[string]interface{}{
				"method": "test",
			},
			want: false,
		},
		{
			name: "params with empty map",
			data: map[string]interface{}{
				"params": map[string]interface{}{},
			},
			want: false,
		},
		{
			name: "params with empty string",
			data: map[string]interface{}{
				"params": "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEffectivelyEmpty(tt.data)
			assert.Equal(t, tt.want, result)
		})
	}
}
