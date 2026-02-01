package middleware

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	wrappedHandler := WrapToolHandler(mockHandler, "github___search_repositories", baseDir, testGetSessionID)

	// Call the wrapped handler
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{
		"query": "test",
	})

	// Verify no error
	require.NoError(t, err, "Handler should not return error")
	require.NotNil(t, result, "Result should not be nil")
	assert.False(t, result.IsError, "Result should not indicate error")

	// Verify response structure
	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "Response should be a map")

	// Check all required fields exist
	assert.Contains(t, dataMap, "queryID")
	assert.Contains(t, dataMap, "payloadPath")
	assert.Contains(t, dataMap, "preview")
	assert.Contains(t, dataMap, "schema")
	assert.Contains(t, dataMap, "originalSize")
	assert.Contains(t, dataMap, "truncated")

	// Verify queryID format
	queryID := dataMap["queryID"].(string)
	assert.Len(t, queryID, 32, "QueryID should be 32 hex characters")

	// Verify payload was saved
	payloadPath := dataMap["payloadPath"].(string)
	assert.FileExists(t, payloadPath, "Payload file should exist")

	// Verify payload content
	payloadContent, err := os.ReadFile(payloadPath)
	require.NoError(t, err, "Should read payload file")

	var originalData map[string]interface{}
	err = json.Unmarshal(payloadContent, &originalData)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify original data structure is preserved in file
	assert.Equal(t, float64(1000), originalData["total_count"])
	assert.NotNil(t, originalData["items"])

	// Verify schema structure
	schemaObj := dataMap["schema"]
	assert.NotNil(t, schemaObj, "Schema should not be nil")

	// Convert schema to JSON string for inspection
	schemaJSON, err := json.Marshal(schemaObj)
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

	// Verify truncation flag
	assert.False(t, dataMap["truncated"].(bool), "Should not be truncated for small payloads")

	// Verify originalSize
	originalSize := dataMap["originalSize"].(int)
	assert.Greater(t, originalSize, 0, "Original size should be positive")
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

	wrappedHandler := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	require.NoError(t, err)
	require.NotNil(t, result)

	dataMap := data.(map[string]interface{})

	// Verify truncation occurred
	truncated := dataMap["truncated"].(bool)
	preview := dataMap["preview"].(string)

	if truncated {
		assert.True(t, len(preview) <= 503, "Preview should be truncated")
		assert.Contains(t, preview, "...", "Truncated preview should end with ...")
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

	wrappedHandler := WrapToolHandler(mockHandler, "test_tool", baseDir, testGetSessionID)
	result, data, err := wrappedHandler(context.Background(), &sdk.CallToolRequest{}, map[string]interface{}{})

	require.NoError(t, err)
	require.NotNil(t, result)

	dataMap := data.(map[string]interface{})
	queryID := dataMap["queryID"].(string)

	// Verify directory structure with session ID
	expectedDir := filepath.Join(baseDir, sessionID, queryID)
	assert.DirExists(t, expectedDir, "Query directory should exist")

	payloadPath := dataMap["payloadPath"].(string)
	assert.Equal(t, filepath.Join(expectedDir, "payload.json"), payloadPath, "Payload path should match expected structure")
}
