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

	// Verify the result Content field contains the transformed response
	require.NotEmpty(t, result.Content, "Result should have Content")
	textContent, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok, "Content should be TextContent")
	require.NotEmpty(t, textContent.Text, "TextContent should have text")

	// Parse the JSON from Content
	var contentMap map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &contentMap)
	require.NoError(t, err, "Content should be valid JSON")

	// Verify transformed response in Content field
	assert.Contains(t, contentMap, "queryID", "Content should contain queryID")
	assert.Contains(t, contentMap, "payloadPath", "Content should contain payloadPath")
	assert.Contains(t, contentMap, "preview", "Content should contain preview")
	assert.Contains(t, contentMap, "schema", "Content should contain schema")
	assert.Contains(t, contentMap, "originalSize", "Content should contain originalSize")
	assert.Contains(t, contentMap, "truncated", "Content should contain truncated")

	// Verify queryID is a valid hex string
	queryID, ok := contentMap["queryID"].(string)
	require.True(t, ok, "queryID should be a string")
	assert.NotEmpty(t, queryID, "queryID should not be empty")

	// Verify schema is present
	schema := contentMap["schema"]
	assert.NotNil(t, schema, "Schema should not be nil")

	// Also verify rewritten response in data return value (for internal use)
	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "Data should be a map")
	assert.Contains(t, dataMap, "queryID", "Data should contain queryID")
	assert.Contains(t, dataMap, "payloadPath", "Data should contain payloadPath")

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

	// Verify Content field contains the transformed response
	require.NotEmpty(t, result.Content, "Result should have Content")
	textContent, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok, "Content should be TextContent")

	// Parse the JSON from Content
	var contentMap map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &contentMap)
	require.NoError(t, err, "Content should be valid JSON")

	// Verify truncation in Content field
	assert.True(t, contentMap["truncated"].(bool), "Should indicate truncation in Content")
	preview := contentMap["preview"].(string)
	assert.LessOrEqual(t, len(preview), 503, "Preview should be truncated to ~500 chars + '...'")
	assert.True(t, strings.HasSuffix(preview, "..."), "Preview should end with '...'")

	// Also verify in data return value
	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "Data should be a map")
	assert.True(t, dataMap["truncated"].(bool), "Should indicate truncation in data")
}

// TestPayloadStorage_SessionIsolation verifies that payloads are stored in session-specific directories
func TestPayloadStorage_SessionIsolation(t *testing.T) {
	// Create temporary directory for test
	baseDir := t.TempDir()

	// Define two different session IDs
	session1 := "session-alpha-123"
	session2 := "session-beta-456"

	// Create a mock handler
	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{
			"data": "test-payload",
		}, nil
	}

	// Create session ID getters for each session
	getSession1 := func(ctx context.Context) string { return session1 }
	getSession2 := func(ctx context.Context) string { return session2 }

	// Call handler for session 1
	wrapped1 := WrapToolHandler(mockHandler, "test_tool", baseDir, getSession1)
	_, data1, err := wrapped1(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
	require.NoError(t, err)

	// Call handler for session 2
	wrapped2 := WrapToolHandler(mockHandler, "test_tool", baseDir, getSession2)
	_, data2, err := wrapped2(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
	require.NoError(t, err)

	// Extract payload paths
	dataMap1 := data1.(map[string]interface{})
	dataMap2 := data2.(map[string]interface{})

	payloadPath1 := dataMap1["payloadPath"].(string)
	payloadPath2 := dataMap2["payloadPath"].(string)

	// Verify paths are different
	assert.NotEqual(t, payloadPath1, payloadPath2, "Different sessions should have different payload paths")

	// Verify session directories exist and are isolated
	session1Dir := filepath.Join(baseDir, session1)
	session2Dir := filepath.Join(baseDir, session2)

	assert.DirExists(t, session1Dir, "Session 1 directory should exist")
	assert.DirExists(t, session2Dir, "Session 2 directory should exist")

	// Verify payload paths contain respective session IDs
	assert.Contains(t, payloadPath1, session1, "Payload path 1 should contain session 1 ID")
	assert.Contains(t, payloadPath2, session2, "Payload path 2 should contain session 2 ID")

	// Verify payloads are not in each other's directories
	assert.NotContains(t, payloadPath1, session2, "Payload 1 should not be in session 2 directory")
	assert.NotContains(t, payloadPath2, session1, "Payload 2 should not be in session 1 directory")

	// Verify files exist at the correct paths
	assert.FileExists(t, payloadPath1, "Payload file 1 should exist")
	assert.FileExists(t, payloadPath2, "Payload file 2 should exist")
}

// TestPayloadStorage_LargePayloadPreserved verifies that the complete large payload is stored to disk
func TestPayloadStorage_LargePayloadPreserved(t *testing.T) {
	baseDir := t.TempDir()

	// Create a large payload (> 500 chars to trigger truncation)
	largeContent := strings.Repeat("This is a large payload content. ", 100) // ~3400 chars
	largePayload := map[string]interface{}{
		"total_count": 1000,
		"items": []interface{}{
			map[string]interface{}{
				"id":          12345,
				"name":        "test-item",
				"description": largeContent,
				"metadata": map[string]interface{}{
					"author": "test-author",
					"tags":   []interface{}{"tag1", "tag2", "tag3"},
				},
			},
		},
	}

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, largePayload, nil
	}

	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
	_, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
	require.NoError(t, err)

	dataMap := data.(map[string]interface{})

	// Verify truncation occurred
	assert.True(t, dataMap["truncated"].(bool), "Large payload should be truncated")

	// Get the payload path
	payloadPath := dataMap["payloadPath"].(string)
	assert.FileExists(t, payloadPath, "Payload file should exist")

	// Read the stored payload
	storedContent, err := os.ReadFile(payloadPath)
	require.NoError(t, err, "Should be able to read stored payload")

	// Verify stored payload is valid JSON
	var storedPayload map[string]interface{}
	err = json.Unmarshal(storedContent, &storedPayload)
	require.NoError(t, err, "Stored payload should be valid JSON")

	// Verify complete payload structure is preserved
	assert.Equal(t, float64(1000), storedPayload["total_count"], "total_count should be preserved")

	items := storedPayload["items"].([]interface{})
	require.Len(t, items, 1, "Should have 1 item")

	item := items[0].(map[string]interface{})
	assert.Equal(t, float64(12345), item["id"], "Item ID should be preserved")
	assert.Equal(t, "test-item", item["name"], "Item name should be preserved")
	assert.Equal(t, largeContent, item["description"], "Complete large description should be preserved")

	metadata := item["metadata"].(map[string]interface{})
	assert.Equal(t, "test-author", metadata["author"], "Metadata author should be preserved")

	// Verify originalSize matches stored content size
	originalSize := dataMap["originalSize"].(int)
	assert.Equal(t, len(storedContent), originalSize, "originalSize should match stored content size")
}

// TestPayloadStorage_DirectoryStructure verifies the directory structure {baseDir}/{sessionID}/{queryID}/payload.json
func TestPayloadStorage_DirectoryStructure(t *testing.T) {
	baseDir := t.TempDir()
	sessionID := "test-session-dir-check"

	getSessionID := func(ctx context.Context) string { return sessionID }

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{"test": "data"}, nil
	}

	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, getSessionID)
	_, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
	require.NoError(t, err)

	dataMap := data.(map[string]interface{})

	payloadPath := dataMap["payloadPath"].(string)
	queryID := dataMap["queryID"].(string)

	// Verify the expected directory structure
	expectedDir := filepath.Join(baseDir, sessionID, queryID)
	expectedPath := filepath.Join(expectedDir, "payload.json")

	assert.Equal(t, expectedPath, payloadPath, "Payload path should match expected structure")
	assert.DirExists(t, expectedDir, "Query directory should exist")

	// Verify the file is named payload.json
	assert.True(t, strings.HasSuffix(payloadPath, "payload.json"), "File should be named payload.json")

	// Verify queryID is a valid hex string (32 chars = 16 bytes in hex)
	assert.Len(t, queryID, 32, "Query ID should be 32 hex characters")
}

// TestPayloadStorage_MultipleRequestsSameSession verifies that multiple requests from the same session
// create separate query directories
func TestPayloadStorage_MultipleRequestsSameSession(t *testing.T) {
	baseDir := t.TempDir()
	sessionID := "same-session"

	getSessionID := func(ctx context.Context) string { return sessionID }

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{"request": "data"}, nil
	}

	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, getSessionID)

	// Make multiple requests
	var payloadPaths []string
	var queryIDs []string

	for i := 0; i < 5; i++ {
		_, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
		require.NoError(t, err)

		dataMap := data.(map[string]interface{})
		payloadPaths = append(payloadPaths, dataMap["payloadPath"].(string))
		queryIDs = append(queryIDs, dataMap["queryID"].(string))
	}

	// Verify all payload paths are unique
	pathSet := make(map[string]bool)
	for _, path := range payloadPaths {
		assert.False(t, pathSet[path], "Payload paths should be unique")
		pathSet[path] = true
	}

	// Verify all query IDs are unique
	idSet := make(map[string]bool)
	for _, id := range queryIDs {
		assert.False(t, idSet[id], "Query IDs should be unique")
		idSet[id] = true
	}

	// Verify all files exist
	for _, path := range payloadPaths {
		assert.FileExists(t, path, "Each payload file should exist")
	}

	// Verify all are in the same session directory but different query directories
	sessionDir := filepath.Join(baseDir, sessionID)
	for _, path := range payloadPaths {
		assert.True(t, strings.HasPrefix(path, sessionDir), "All payloads should be in session directory")
	}
}

// TestPayloadStorage_FilePermissions verifies that payload directories and files have secure permissions
func TestPayloadStorage_FilePermissions(t *testing.T) {
	baseDir := t.TempDir()
	sessionID := "test-permissions"
	queryID := "test-query-perms"
	payload := []byte(`{"secure": "data"}`)

	filePath, err := savePayload(baseDir, sessionID, queryID, payload)
	require.NoError(t, err)

	// Check directory permissions
	dirPath := filepath.Dir(filePath)
	dirInfo, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(), "Directory should have 0700 permissions")

	// Check file permissions
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm(), "File should have 0600 permissions")
}

// TestPayloadStorage_DefaultSessionID verifies behavior when session ID is empty
func TestPayloadStorage_DefaultSessionID(t *testing.T) {
	baseDir := t.TempDir()

	// Return empty string to trigger default behavior
	getEmptySessionID := func(ctx context.Context) string { return "" }

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{"test": "data"}, nil
	}

	wrapped := WrapToolHandler(mockHandler, "test_tool", baseDir, getEmptySessionID)
	_, data, err := wrapped(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})
	require.NoError(t, err)

	dataMap := data.(map[string]interface{})
	payloadPath := dataMap["payloadPath"].(string)

	// Verify the payload is stored in "default" session directory
	assert.Contains(t, payloadPath, "default", "Empty session should use 'default' directory")
	assert.FileExists(t, payloadPath, "Payload file should exist")
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
