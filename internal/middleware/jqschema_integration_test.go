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

// integrationPayloadMetadataToMap converts PayloadMetadata to map[string]interface{} for test assertions
func integrationPayloadMetadataToMap(t *testing.T, data interface{}) map[string]interface{} {
	t.Helper()
	pm, ok := data.(PayloadMetadata)
	if !ok {
		t.Fatalf("expected PayloadMetadata, got %T", data)
	}
	jsonBytes, err := json.Marshal(pm)
	require.NoError(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)
	return result
}

// TestMiddlewareIntegration tests the complete middleware flow
func TestMiddlewareIntegration(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	// Create a mock handler that returns GitHub-like search data
	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		// Simulate a GitHub search response
		response := map[string]interface{}{
			"total_count": 1000,
			"items": []interface{}{
				map[string]interface{}{
					"name":        "repo1",
					"id":          12345,
					"stars":       100,
					"private":     false,
					"description": "A test repository",
					"owner": map[string]interface{}{
						"login": "user1",
						"id":    999,
					},
				},
				map[string]interface{}{
					"name":        "repo2",
					"id":          67890,
					"stars":       250,
					"private":     true,
					"description": "Another test repository",
					"owner": map[string]interface{}{
						"login": "user2",
						"id":    888,
					},
				},
			},
		}
		return &sdk.CallToolResult{IsError: false}, response, nil
	}

	// Wrap with middleware
	wrappedHandler := WrapToolHandler(mockHandler, "github___search_repositories", baseDir, 5, testGetSessionID)

	// Call the wrapped handler
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{
		"query": "test",
	})

	// Verify no error
	require.NoError(t, err, "Handler should not return error")
	require.NotNil(t, result, "Result should not be nil")
	assert.False(t, result.IsError, "Result should not indicate error")

	// Verify the result Content field contains the transformed response
	require.NotEmpty(t, result.Content, "Result should have Content")
	textContent, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok, "Content should be TextContent")
	require.NotEmpty(t, textContent.Text, "TextContent should have text")

	// Parse the JSON from Content
	var contentMap map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &contentMap)
	require.NoError(t, err, "Content should be valid JSON")

	// Verify all required fields exist in Content (queryID is excluded from JSON via json:"-")
	assert.Contains(t, contentMap, "payloadPath", "Content should contain payloadPath")
	assert.Contains(t, contentMap, "payloadPreview", "Content should contain payloadPreview")
	assert.Contains(t, contentMap, "payloadSchema", "Content should contain payloadSchema")
	assert.Contains(t, contentMap, "originalSize", "Content should contain originalSize")
	assert.NotContains(t, contentMap, "queryID", "Content should NOT contain queryID (excluded from JSON)")

	// Verify response structure in data return value (for internal use)
	// Access QueryID directly from the struct since it's excluded from JSON
	pm, ok := data.(PayloadMetadata)
	require.True(t, ok, "data should be PayloadMetadata")
	assert.Len(t, pm.QueryID, 32, "QueryID should be 32 hex characters")

	// Verify payload was saved
	assert.FileExists(t, pm.PayloadPath, "Payload file should exist")

	// Verify payload content
	payloadContent, err := os.ReadFile(pm.PayloadPath)
	require.NoError(t, err, "Should read payload file")

	var originalData map[string]interface{}
	err = json.Unmarshal(payloadContent, &originalData)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify original data structure is preserved in file
	assert.Equal(t, float64(1000), originalData["total_count"])
	assert.NotNil(t, originalData["items"])

	// Verify schema structure
	assert.NotNil(t, pm.PayloadSchema, "Schema should not be nil")

	// Convert schema to JSON string for inspection
	schemaJSON, err := json.Marshal(pm.PayloadSchema)
	require.NoError(t, err, "Schema should be marshallable")

	var schema map[string]interface{}
	err = json.Unmarshal(schemaJSON, &schema)
	require.NoError(t, err, "Schema should be valid JSON")

	// Verify schema has the expected structure
	assert.Contains(t, schema, "total_count")
	assert.Contains(t, schema, "items")
	assert.Equal(t, "number", schema["total_count"])

	// Verify items is an array with schema
	items, ok := schema["items"].([]interface{})
	require.True(t, ok, "items should be an array")
	assert.Len(t, items, 1, "items schema should have one element")

	// Verify the item schema
	itemSchema, ok := items[0].(map[string]interface{})
	require.True(t, ok, "item schema should be an object")
	assert.Contains(t, itemSchema, "name")
	assert.Contains(t, itemSchema, "id")
	assert.Contains(t, itemSchema, "stars")
	assert.Contains(t, itemSchema, "private")
	assert.Contains(t, itemSchema, "description")
	assert.Contains(t, itemSchema, "owner")

	// Verify types
	assert.Equal(t, "string", itemSchema["name"])
	assert.Equal(t, "number", itemSchema["id"])
	assert.Equal(t, "number", itemSchema["stars"])
	assert.Equal(t, "boolean", itemSchema["private"])
	assert.Equal(t, "string", itemSchema["description"])

	// Verify nested owner schema
	ownerSchema, ok := itemSchema["owner"].(map[string]interface{})
	require.True(t, ok, "owner schema should be an object")
	assert.Contains(t, ownerSchema, "login")
	assert.Contains(t, ownerSchema, "id")
	assert.Equal(t, "string", ownerSchema["login"])
	assert.Equal(t, "number", ownerSchema["id"])

	// Verify originalSize
	assert.Greater(t, pm.OriginalSize, 0, "Original size should be positive")
}

// TestMiddlewareWithLargePayload tests truncation behavior
func TestMiddlewareWithLargePayload(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	// Create a large payload
	largeItems := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		largeItems[i] = map[string]interface{}{
			"id":          i,
			"name":        "item-" + string(rune(i)),
			"description": "This is a long description to make the payload large enough for truncation testing purposes.",
		}
	}

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{
			"total_count": 100,
			"items":       largeItems,
		}, nil
	}

	wrappedHandler := WrapToolHandler(mockHandler, "test_tool", baseDir, 5, testGetSessionID)
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify Content field has transformed response
	require.NotEmpty(t, result.Content, "Result should have Content")
	textContent, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok, "Content should be TextContent")

	var contentMap map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &contentMap)
	require.NoError(t, err, "Content should be valid JSON")

	// Verify preview truncation in Content field (preview ends with ... when truncated)
	previewInContent := contentMap["payloadPreview"].(string)
	if strings.HasSuffix(previewInContent, "...") {
		assert.True(t, len(previewInContent) <= 503, "Preview in Content should be truncated")
	}

	// Also check data return value
	dataMap := integrationPayloadMetadataToMap(t, data)

	// Verify preview truncation (check if it ends with ...)
	preview := dataMap["payloadPreview"].(string)
	if strings.HasSuffix(preview, "...") {
		assert.True(t, len(preview) <= 503, "Preview should be truncated")
	}

	// Verify payload file has complete data
	payloadPath := dataMap["payloadPath"].(string)
	payloadContent, err := os.ReadFile(payloadPath)
	require.NoError(t, err)

	var completeData map[string]interface{}
	err = json.Unmarshal(payloadContent, &completeData)
	require.NoError(t, err)

	// Verify complete data is in the file
	completeItems := completeData["items"].([]interface{})
	assert.Len(t, completeItems, 100, "File should contain all items")
}

// TestMiddlewareDirectoryCreation tests that directories are created correctly
func TestMiddlewareDirectoryCreation(t *testing.T) {
	// Create temporary directory for test
	baseDir := filepath.Join(os.TempDir(), "test-jq-payloads")
	defer os.RemoveAll(baseDir)

	sessionID := "test-session"

	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, map[string]interface{}{"test": "data"}, nil
	}

	wrappedHandler := WrapToolHandler(mockHandler, "test_tool", baseDir, 5, testGetSessionID)
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify Content field (queryID is excluded from JSON)
	require.NotEmpty(t, result.Content, "Result should have Content")
	textContent, ok := result.Content[0].(*sdk.TextContent)
	require.True(t, ok, "Content should be TextContent")

	var contentMap map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &contentMap)
	require.NoError(t, err, "Content should be valid JSON")

	// queryID should NOT be in the JSON response
	assert.NotContains(t, contentMap, "queryID", "Content should NOT contain queryID (excluded from JSON)")

	// Access QueryID directly from the struct for internal verification
	pm, ok := data.(PayloadMetadata)
	require.True(t, ok, "data should be PayloadMetadata")
	queryID := pm.QueryID

	// Verify directory structure with session ID
	expectedDir := filepath.Join(baseDir, sessionID, queryID)
	assert.DirExists(t, expectedDir, "Query directory should exist")

	payloadPath := pm.PayloadPath
	assert.Equal(t, filepath.Join(expectedDir, "payload.json"), payloadPath, "Payload path should match expected structure")
}
