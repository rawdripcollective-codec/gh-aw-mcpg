package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeInputSchema_NilSchema(t *testing.T) {
	result := NormalizeInputSchema(nil, "test-tool")
	// When backend provides no schema, we return a default empty object schema
	// This is required by the SDK's Server.AddTool method and allows clients
	// to see that the tool accepts parameters (though any are allowed)
	require.NotNil(t, result, "Nil schema should return default empty object schema")
	assert.Equal(t, "object", result["type"], "Default schema should have type 'object'")
	assert.Contains(t, result, "properties", "Default schema should have properties field")
	properties := result["properties"].(map[string]interface{})
	assert.Empty(t, properties, "Default schema properties should be empty")
}

func TestNormalizeInputSchema_EmptySchema(t *testing.T) {
	schema := map[string]interface{}{}
	result := NormalizeInputSchema(schema, "test-tool")
	// Empty schema should be normalized to a valid object schema since SDK requires type: object
	expected := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	assert.Equal(t, expected, result, "Empty schema should be normalized to object schema")
}

func TestNormalizeInputSchema_NonObjectType(t *testing.T) {
	testCases := []struct {
		name   string
		schema map[string]interface{}
	}{
		{
			name: "string type",
			schema: map[string]interface{}{
				"type": "string",
			},
		},
		{
			name: "number type",
			schema: map[string]interface{}{
				"type": "number",
			},
		},
		{
			name: "array type",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		{
			name: "boolean type",
			schema: map[string]interface{}{
				"type": "boolean",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeInputSchema(tc.schema, "test-tool")
			assert.Equal(t, tc.schema, result, "Non-object schema should be unchanged")
		})
	}
}

func TestNormalizeInputSchema_ObjectWithProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "number",
			},
		},
		"required": []interface{}{"name"},
	}

	result := NormalizeInputSchema(schema, "test-tool")
	assert.Equal(t, schema, result, "Object schema with properties should be unchanged")
}

func TestNormalizeInputSchema_ObjectWithAdditionalProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": true,
	}

	result := NormalizeInputSchema(schema, "test-tool")
	assert.Equal(t, schema, result, "Object schema with additionalProperties should be unchanged")
}

func TestNormalizeInputSchema_ObjectWithBothPropertiesAndAdditionalProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"additionalProperties": false,
	}

	result := NormalizeInputSchema(schema, "test-tool")
	assert.Equal(t, schema, result, "Object schema with both properties and additionalProperties should be unchanged")
}

func TestNormalizeInputSchema_ObjectWithoutPropertiesBroken(t *testing.T) {
	// This is the broken schema case from GitHub MCP Server
	schema := map[string]interface{}{
		"type": "object",
	}

	result := NormalizeInputSchema(schema, "github-get_commit")

	require.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, "object", result["type"], "Type should remain object")

	// Check that properties was added
	properties, hasProperties := result["properties"]
	require.True(t, hasProperties, "Properties field should be added")

	// Check that properties is an empty map
	propertiesMap, isMap := properties.(map[string]interface{})
	require.True(t, isMap, "Properties should be a map")
	assert.Empty(t, propertiesMap, "Properties should be an empty map")
}

func TestNormalizeInputSchema_ObjectWithoutPropertiesDoesNotModifyOriginal(t *testing.T) {
	// Verify that the original schema is not modified
	schema := map[string]interface{}{
		"type": "object",
	}

	result := NormalizeInputSchema(schema, "test-tool")

	// Original should not have properties
	_, originalHasProperties := schema["properties"]
	assert.False(t, originalHasProperties, "Original schema should not be modified")

	// Result should have properties
	_, resultHasProperties := result["properties"]
	assert.True(t, resultHasProperties, "Result should have properties field")
}

func TestNormalizeInputSchema_ComplexObjectWithoutProperties(t *testing.T) {
	// Object schema with other fields but missing properties
	schema := map[string]interface{}{
		"type":        "object",
		"title":       "Complex Schema",
		"description": "A complex schema without properties",
		"examples": []interface{}{
			map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	result := NormalizeInputSchema(schema, "complex-tool")

	require.NotNil(t, result, "Result should not be nil")

	// Check all original fields are preserved
	assert.Equal(t, "object", result["type"])
	assert.Equal(t, "Complex Schema", result["title"])
	assert.Equal(t, "A complex schema without properties", result["description"])
	assert.NotNil(t, result["examples"])

	// Check that properties was added
	properties, hasProperties := result["properties"]
	require.True(t, hasProperties, "Properties field should be added")
	assert.Empty(t, properties.(map[string]interface{}), "Properties should be empty")
}

func TestNormalizeInputSchema_NonStringType(t *testing.T) {
	// Edge case: type field is not a string
	schema := map[string]interface{}{
		"type": 123, // invalid, but should not crash
	}

	result := NormalizeInputSchema(schema, "test-tool")
	assert.Equal(t, schema, result, "Schema with non-string type should be unchanged")
}

func TestNormalizeInputSchema_MultipleToolNames(t *testing.T) {
	// Verify that different tool names work correctly
	schema := map[string]interface{}{
		"type": "object",
	}

	tools := []string{"tool-1", "tool-2", "github-get_commit", "issue_read"}
	for _, toolName := range tools {
		result := NormalizeInputSchema(schema, toolName)
		_, hasProperties := result["properties"]
		assert.True(t, hasProperties, "Properties should be added for tool: "+toolName)
	}
}
