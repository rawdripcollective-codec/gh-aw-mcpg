package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitJSONLLogger(t *testing.T) {
	require := require.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Test successful initialization
	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Verify log file was created
	logPath := filepath.Join(logDir, "test.jsonl")
	_, err = os.Stat(logPath)
	require.NoError(err, "Log file should exist at %s", logPath)
}

func TestJSONLLoggerClose(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")

	// Test closing
	err = CloseJSONLLogger()
	assert.NoError(err, "CloseJSONLLogger should not error")

	// Test closing again (should not error)
	err = CloseJSONLLogger()
	assert.NoError(err, "CloseJSONLLogger should not error on second call")
}

func TestLogRPCMessageJSONL(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log a request
	requestPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "tools/list", requestPayload, nil)

	// Log a response
	responsePayload := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`)
	LogRPCMessageJSONL(RPCDirectionInbound, RPCMessageResponse, "github", "", responsePayload, nil)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify the log file
	logPath := filepath.Join(logDir, "test.jsonl")
	file, err := os.Open(logPath)
	require.NoError(err, "Failed to open log file")
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var entry JSONLRPCMessage
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(err, "Failed to parse JSONL line %d: %s", lineCount, line)

		// Verify common fields
		assert.NotEmpty(entry.Timestamp, "Line %d: missing timestamp", lineCount)
		assert.NotEmpty(entry.Direction, "Line %d: missing direction", lineCount)
		assert.NotEmpty(entry.Type, "Line %d: missing type", lineCount)
		assert.NotEmpty(entry.ServerID, "Line %d: missing server_id", lineCount)
		assert.NotNil(entry.Payload, "Line %d: missing payload", lineCount)

		// Verify line-specific fields
		switch lineCount {
		case 1:
			// First line should be a REQUEST
			assert.Equal("REQUEST", entry.Type, "Line 1: expected type REQUEST")
			assert.Equal("tools/list", entry.Method, "Line 1: expected method tools/list")
			assert.Equal("OUT", entry.Direction, "Line 1: expected direction OUT")
		case 2:
			// Second line should be a RESPONSE
			assert.Equal("RESPONSE", entry.Type, "Line 2: expected type RESPONSE")
			assert.Equal("IN", entry.Direction, "Line 2: expected direction IN")
		}
	}

	err = scanner.Err()
	require.NoError(err, "Error reading log file")

	assert.Equal(2, lineCount, "Expected 2 log entries")
}

func TestSanitizePayload(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectRedacted bool
		checkField     string
	}{
		{
			name:           "token in payload",
			input:          `{"token":"ghp_1234567890123456789012345678901234567890"}`,
			expectRedacted: true,
			checkField:     "token",
		},
		{
			name:           "nested token in params",
			input:          `{"params":{"auth":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.sig"}}`,
			expectRedacted: true,
			checkField:     "params.auth",
		},
		{
			name:           "password field",
			input:          `{"password":"supersecret123"}`,
			expectRedacted: true,
			checkField:     "password",
		},
		{
			name:           "clean payload",
			input:          `{"method":"tools/list","id":1}`,
			expectRedacted: false,
			checkField:     "method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			result := sanitize.SanitizeJSON([]byte(tt.input))
			require.NotNil(result, "sanitize.SanitizeJSON returned nil")

			// The result is already a sanitized string
			sanitizedStr := string(result)

			if tt.expectRedacted {
				// Should contain [REDACTED]
				assert.Contains(sanitizedStr, "[REDACTED]", "Expected sanitized payload to contain [REDACTED]")

				// Should NOT contain the original secret patterns
				assert.NotContains(sanitizedStr, "ghp_", "Sanitized payload should not contain GitHub token")
				assert.NotContains(sanitizedStr, "Bearer eyJ", "Sanitized payload should not contain Bearer token")
				assert.NotContains(sanitizedStr, "supersecret", "Sanitized payload should not contain password")
			} else {
				// Should not contain [REDACTED] for clean payloads
				assert.NotContains(sanitizedStr, "[REDACTED]", "Clean payload should not be redacted")
			}
		})
	}
}

func TestSanitizePayloadWithNestedStructures(t *testing.T) {
	assert := assert.New(t)
	input := `{
		"params": {
			"credentials": {
				"apiKey": "test_fake_api_key_1234567890abcdefghij",
				"token": "ghp_1234567890123456789012345678901234567890"
			},
			"data": {
				"items": [
					{"name": "item1", "secret": "password123"},
					{"name": "item2", "value": "safe"}
				]
			}
		}
	}`

	result := sanitize.SanitizeJSON([]byte(input))

	// The result is already a sanitized string
	sanitizedStr := string(result)

	// Should redact all secrets at all levels
	assert.Contains(sanitizedStr, "[REDACTED]", "Expected [REDACTED] in sanitized output")

	// Should NOT contain original secrets
	assert.NotContains(sanitizedStr, "test_fake_api_key", "API key should be sanitized")
	assert.NotContains(sanitizedStr, "ghp_", "GitHub token should be sanitized")
	assert.NotContains(sanitizedStr, "password123", "Password should be sanitized")

	// Should preserve non-secret values
	assert.Contains(sanitizedStr, "item1", "Non-secret value 'item1' should be preserved")
	assert.Contains(sanitizedStr, "safe", "Non-secret value 'safe' should be preserved")
}

func TestLogRPCMessageJSONLWithError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log a response with error
	responsePayload := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`)
	testErr := fmt.Errorf("backend connection failed")
	LogRPCMessageJSONL(RPCDirectionInbound, RPCMessageResponse, "github", "", responsePayload, testErr)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	require.NoError(err, "Failed to read log file")

	var entry JSONLRPCMessage
	err = json.Unmarshal(content, &entry)
	require.NoError(err, "Failed to parse JSONL")

	assert.Equal("backend connection failed", entry.Error, "Error field should match")
}

func TestLogRPCMessageJSONLWithInvalidJSON(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log invalid JSON
	invalidPayload := []byte(`{invalid json}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", invalidPayload, nil)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	require.NoError(err, "Failed to read log file")

	var entry JSONLRPCMessage
	err = json.Unmarshal(content, &entry)
	require.NoError(err, "Failed to parse JSONL")

	// The payload should be wrapped in a valid JSON object with an error marker
	var payloadObj map[string]interface{}
	err = json.Unmarshal(entry.Payload, &payloadObj)
	require.NoError(err, "Failed to parse payload")

	assert.Equal("invalid JSON", payloadObj["_error"], "Expected _error field in payload")
	assert.Contains(fmt.Sprintf("%v", payloadObj["_raw"]), "invalid", "Expected _raw field to contain original invalid JSON")
}

func TestJSONLLoggerNotInitialized(t *testing.T) {
	// Ensure no global logger is set
	CloseJSONLLogger()

	// Should not panic when logging without initialization
	requestPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", requestPayload, nil)
	// Test passes if no panic occurs
}

func TestMultipleMessagesInJSONL(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log multiple messages
	messages := []struct {
		direction RPCMessageDirection
		msgType   RPCMessageType
		serverID  string
		method    string
		payload   string
	}{
		{RPCDirectionOutbound, RPCMessageRequest, "github", "tools/list", `{"jsonrpc":"2.0","method":"tools/list"}`},
		{RPCDirectionInbound, RPCMessageResponse, "github", "", `{"jsonrpc":"2.0","result":{}}`},
		{RPCDirectionOutbound, RPCMessageRequest, "backend", "tools/call", `{"jsonrpc":"2.0","method":"tools/call"}`},
		{RPCDirectionInbound, RPCMessageResponse, "backend", "", `{"jsonrpc":"2.0","result":{}}`},
	}

	for _, msg := range messages {
		LogRPCMessageJSONL(msg.direction, msg.msgType, msg.serverID, msg.method, []byte(msg.payload), nil)
	}

	// Close to flush
	CloseJSONLLogger()

	// Read and verify all lines
	logPath := filepath.Join(logDir, "test.jsonl")
	file, err := os.Open(logPath)
	require.NoError(err, "Failed to open log file")
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var entry JSONLRPCMessage
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(err, "Failed to parse JSONL line %d", lineCount)

		// Each line should be valid JSONL with required fields
		assert.NotEmpty(entry.Timestamp, "Line %d: missing timestamp", lineCount)
		assert.NotEmpty(entry.ServerID, "Line %d: missing server_id", lineCount)
	}

	assert.Equal(len(messages), lineCount, "Expected %d log entries", len(messages))
}

func TestSanitizePayloadCompactsJSON(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Test that multi-line JSON is compacted to a single line
	multilineJSON := `{
		"jsonrpc": "2.0",
		"method": "test",
		"params": {
			"nested": {
				"value": "test"
			}
		}
	}`

	result := sanitize.SanitizeJSON([]byte(multilineJSON))
	resultStr := string(result)

	// The result should not contain newlines
	assert.NotContains(resultStr, "\n", "Result should not contain newlines")

	// Should still be valid JSON
	var tmp interface{}
	err := json.Unmarshal(result, &tmp)
	require.NoError(err, "Result should be valid JSON")

	// Should contain the expected values
	assert.Contains(resultStr, "jsonrpc", "Result should contain 'jsonrpc'")
	assert.Contains(resultStr, "test", "Result should contain 'test'")
}

func TestInitJSONLLoggerWithInvalidPath(t *testing.T) {
	assert := assert.New(t)

	// Test initialization with an invalid directory path (permission denied scenario)
	// Using /proc/self as it's read-only and will fail to create subdirectories
	err := InitJSONLLogger("/proc/self/invalid", "test.jsonl")
	assert.Error(err, "InitJSONLLogger should fail with invalid directory path")
}

func TestLogRPCMessageJSONLDirectionTypes(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	tests := []struct {
		name      string
		direction RPCMessageDirection
		msgType   RPCMessageType
		expected  map[string]string
	}{
		{
			name:      "outbound request",
			direction: RPCDirectionOutbound,
			msgType:   RPCMessageRequest,
			expected:  map[string]string{"direction": "OUT", "type": "REQUEST"},
		},
		{
			name:      "inbound request",
			direction: RPCDirectionInbound,
			msgType:   RPCMessageRequest,
			expected:  map[string]string{"direction": "IN", "type": "REQUEST"},
		},
		{
			name:      "outbound response",
			direction: RPCDirectionOutbound,
			msgType:   RPCMessageResponse,
			expected:  map[string]string{"direction": "OUT", "type": "RESPONSE"},
		},
		{
			name:      "inbound response",
			direction: RPCDirectionInbound,
			msgType:   RPCMessageResponse,
			expected:  map[string]string{"direction": "IN", "type": "RESPONSE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPayload := []byte(`{"jsonrpc":"2.0","id":1}`)

			// Clear previous log file
			logPath := filepath.Join(logDir, "test.jsonl")
			os.Remove(logPath)

			LogRPCMessageJSONL(tt.direction, tt.msgType, "test-server", "test-method", testPayload, nil)
			CloseJSONLLogger()

			// Re-init for next iteration
			if t.Name() != tests[len(tests)-1].name {
				InitJSONLLogger(logDir, "test.jsonl")
			}

			// Read and verify
			content, err := os.ReadFile(logPath)
			if err != nil {
				return // File might not exist yet
			}

			var entry JSONLRPCMessage
			json.Unmarshal(content, &entry)

			assert.Equal(tt.expected["direction"], entry.Direction, "Direction should match")
			assert.Equal(tt.expected["type"], entry.Type, "Type should match")
		})
	}
}

func TestLogRPCMessageJSONLEmptyPayload(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log with empty payload
	emptyPayload := []byte(`{}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", emptyPayload, nil)

	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	require.NoError(err, "Failed to read log file")

	var entry JSONLRPCMessage
	err = json.Unmarshal(content, &entry)
	require.NoError(err, "Failed to parse JSONL")

	// Should still have a valid payload field
	assert.NotNil(entry.Payload, "Payload should not be nil even when empty")
	assert.NotEmpty(entry.Timestamp, "Timestamp should be present")
	assert.Equal("github", entry.ServerID, "ServerID should match")
}

func TestLogRPCMessageJSONLWithNilError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	err := InitJSONLLogger(logDir, "test.jsonl")
	require.NoError(err, "InitJSONLLogger failed")
	defer CloseJSONLLogger()

	// Log with nil error (normal case)
	payload := []byte(`{"jsonrpc":"2.0","id":1}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", payload, nil)

	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	require.NoError(err, "Failed to read log file")

	var entry JSONLRPCMessage
	err = json.Unmarshal(content, &entry)
	require.NoError(err, "Failed to parse JSONL")

	// Error field should be empty when nil error is passed
	assert.Empty(entry.Error, "Error field should be empty when no error")
}
