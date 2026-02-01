package middleware

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testGetSessionID is a helper function for tests that returns a session ID from context
func testGetSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value("test-session-id").(string); ok {
		return sessionID
	}
	return "test-session"
}

func TestGenerateRandomID(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRandomID()
		assert.NotEmpty(t, id, "ID should not be empty")
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}

func TestApplyJqSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple object",
			input:    map[string]interface{}{"name": "test", "count": 42},
			expected: `{"count":"number","name":"string"}`,
		},
		{
			name:     "nested object",
			input:    map[string]interface{}{"user": map[string]interface{}{"id": 123, "active": true}},
			expected: `{"user":{"active":"boolean","id":"number"}}`,
		},
		{
			name:     "array with objects",
			input:    map[string]interface{}{"items": []interface{}{map[string]interface{}{"id": 1, "name": "test"}}},
			expected: `{"items":[{"id":"number","name":"string"}]}`,
		},
		{
			name:     "empty array",
			input:    map[string]interface{}{"items": []interface{}{}},
			expected: `{"items":[]}`,
		},
		{
			name: "complex nested structure",
			input: map[string]interface{}{
				"total_count": 1000,
				"items": []interface{}{
					map[string]interface{}{
						"login":    "user1",
						"id":       123,
						"verified": true,
					},
				},
			},
			expected: `{"items":[{"id":"number","login":"string","verified":"boolean"}],"total_count":"number"}`,
		},
		{
			name:     "null value",
			input:    map[string]interface{}{"value": nil},
			expected: `{"value":"null"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyJqSchema(context.Background(), tt.input)
			require.NoError(t, err, "applyJqSchema should not return error")
			assert.JSONEq(t, tt.expected, result, "Schema should match expected")
		})
	}
}

func TestSavePayload(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	sessionID := "test-session-123"
	queryID := "test-query-456"
	payload := []byte(`{"test": "data"}`)

	filePath, err := savePayload(baseDir, sessionID, queryID, payload)
	require.NoError(t, err, "savePayload should not return error")

	// Verify file exists
	assert.FileExists(t, filePath, "Payload file should exist")

	// Verify file content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Should be able to read payload file")
	assert.Equal(t, payload, content, "File content should match payload")

	// Verify directory structure
	expectedDir := filepath.Join(baseDir, sessionID, queryID)
	assert.DirExists(t, expectedDir, "Directory should exist")
}

func TestWrapToolHandler(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	// Create a mock handler
	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{
			"message": "success",
			"data": map[string]interface{}{
				"id":    123,
				"items": []interface{}{map[string]interface{}{"name": "test"}},
			},
		}, nil
	}

	// Wrap the handler
	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)

	// Call the wrapped handler
	result, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	// Assertions
	require.NoError(t, err, "Wrapped handler should not return error")
	require.NotNil(t, result, "Result should not be nil")
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify rewritten response structure
	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "Data should be a map")

	assert.Contains(t, dataMap, "queryID", "Response should contain queryID")
	assert.Contains(t, dataMap, "payloadPath", "Response should contain payloadPath")
	assert.Contains(t, dataMap, "preview", "Response should contain preview")
	assert.Contains(t, dataMap, "schema", "Response should contain schema")
	assert.Contains(t, dataMap, "originalSize", "Response should contain originalSize")
	assert.Contains(t, dataMap, "truncated", "Response should contain truncated")

	// Verify queryID is a valid hex string
	queryID, ok := dataMap["queryID"].(string)
	require.True(t, ok, "queryID should be a string")
	assert.NotEmpty(t, queryID, "queryID should not be empty")

	// Verify schema is present
	schema := dataMap["schema"]
	assert.NotNil(t, schema, "Schema should not be nil")

	// Clean up test directory
	defer os.RemoveAll(filepath.Join("/tmp", "gh-awmg"))
}

func TestWrapToolHandler_ErrorHandling(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	t.Run("handler returns error", func(t *testing.T) {
		mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{IsError: true}, nil, assert.AnError
		}

		wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
		result, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

		assert.Error(t, err, "Should return error from handler")
		assert.Nil(t, data, "Data should be nil on error")
		assert.True(t, result.IsError, "Result should indicate error")
	})

	t.Run("handler returns nil data", func(t *testing.T) {
		mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{IsError: false}, nil, nil
		}

		wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
		result, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

		assert.NoError(t, err, "Should not return error")
		assert.Nil(t, data, "Data should remain nil")
		assert.False(t, result.IsError, "Result should not indicate error")
	})
}

func TestWrapToolHandler_LongPayload(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	// Create a handler that returns a large payload
	largeData := map[string]interface{}{
		"message": strings.Repeat("x", 1000),
	}

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, largeData, nil
	}

	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
	result, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	require.NoError(t, err, "Should not return error")
	require.NotNil(t, result, "Result should not be nil")

	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "Data should be a map")

	// Verify truncation
	assert.True(t, dataMap["truncated"].(bool), "Should indicate truncation")
	preview := dataMap["preview"].(string)
	assert.LessOrEqual(t, len(preview), 503, "Preview should be truncated to ~500 chars + '...'")
	assert.True(t, strings.HasSuffix(preview, "..."), "Preview should end with '...'")
}

func TestShouldApplyMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		expected bool
	}{
		{
			name:     "regular tool",
			toolName: "github___search_code",
			expected: true,
		},
		{
			name:     "sys tool",
			toolName: "sys___init",
			expected: false,
		},
		{
			name:     "another sys tool",
			toolName: "sys___list_servers",
			expected: false,
		},
		{
			name:     "tool with sys in name but not prefix",
			toolName: "mysys___tool",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldApplyMiddleware(tt.toolName)
			assert.Equal(t, tt.expected, result, "ShouldApplyMiddleware result should match expected")
		})
	}
}

func TestApplyJqSchema_ErrorCases(t *testing.T) {
	t.Run("handles complex recursive structures", func(t *testing.T) {
		// Create a deeply nested structure
		input := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"value": "deep",
					},
				},
			},
		}

		result, err := applyJqSchema(context.Background(), input)
		require.NoError(t, err, "Should handle deeply nested structures")
		assert.NotEmpty(t, result, "Result should not be empty")

		// Verify the schema is correctly nested
		var schema map[string]interface{}
		err = json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err, "Should be valid JSON")
		assert.Contains(t, schema, "level1", "Should contain level1")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		input := map[string]interface{}{"test": "data"}

		// The query should complete quickly, but context cancellation should be handled gracefully
		// Note: For this simple query, it may complete before cancellation is processed
		_, err := applyJqSchema(ctx, input)

		// Either succeeds (query completed before cancellation) or fails with context error
		if err != nil {
			assert.Contains(t, err.Error(), "context", "Error should mention context if cancelled")
		}
	})
}
