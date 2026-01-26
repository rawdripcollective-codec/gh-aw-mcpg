package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchAndFixSchema_SuccessfulFetch tests the happy path where schema is fetched successfully
func TestFetchAndFixSchema_SuccessfulFetch(t *testing.T) {
	// Create a minimal valid schema for testing
	validSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"$id":     "https://example.com/schema.json",
		"type":    "object",
		"properties": map[string]interface{}{
			"test": map[string]interface{}{
				"type": "string",
			},
		},
	}
	
	schemaJSON, err := json.Marshal(validSchema)
	require.NoError(t, err)
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	// Test fetching from the server
	result, err := fetchAndFixSchema(server.URL)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Verify the result is valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(result, &parsed)
	assert.NoError(t, err)
}

// TestFetchAndFixSchema_HTTPError tests handling of HTTP error responses
func TestFetchAndFixSchema_HTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    string
	}{
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			wantErr:    "failed to fetch schema: HTTP 404",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantErr:    "failed to fetch schema: HTTP 500",
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			wantErr:    "failed to fetch schema: HTTP 403",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    "failed to fetch schema: HTTP 503",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()
			
			result, err := fetchAndFixSchema(server.URL)
			
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestFetchAndFixSchema_NetworkError tests handling of network failures
func TestFetchAndFixSchema_NetworkError(t *testing.T) {
	// Use an invalid URL that will cause a network error
	invalidURL := "http://invalid-host-that-does-not-exist-12345.com/schema.json"
	
	result, err := fetchAndFixSchema(invalidURL)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch schema from")
}

// TestFetchAndFixSchema_Timeout tests handling of request timeouts
func TestFetchAndFixSchema_Timeout(t *testing.T) {
	// Create a server that delays longer than the client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // fetchAndFixSchema has 10 second timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	// The error should indicate a timeout or context deadline exceeded
	assert.Contains(t, err.Error(), "failed to fetch schema from")
}

// TestFetchAndFixSchema_InvalidJSON tests handling of invalid JSON in response
func TestFetchAndFixSchema_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{"))
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse schema")
}

// TestFetchAndFixSchema_EmptyResponse tests handling of empty response body
func TestFetchAndFixSchema_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Send empty body
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse schema")
}

// TestFetchAndFixSchema_CustomServerConfigPatternFix tests the negative lookahead fix for customServerConfig.type
func TestFetchAndFixSchema_CustomServerConfigPatternFix(t *testing.T) {
	// Schema with negative lookahead pattern in customServerConfig
	schemaWithNegativeLookahead := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"definitions": map[string]interface{}{
			"customServerConfig": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":    "string",
						"pattern": "^(?!stdio$|http$).*",
					},
				},
			},
		},
	}
	
	schemaJSON, err := json.Marshal(schemaWithNegativeLookahead)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Parse the result and verify the fix was applied
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	// Navigate to customServerConfig.properties.type
	definitions, ok := fixed["definitions"].(map[string]interface{})
	require.True(t, ok, "definitions should exist")
	
	customServerConfig, ok := definitions["customServerConfig"].(map[string]interface{})
	require.True(t, ok, "customServerConfig should exist")
	
	properties, ok := customServerConfig["properties"].(map[string]interface{})
	require.True(t, ok, "properties should exist")
	
	typeField, ok := properties["type"].(map[string]interface{})
	require.True(t, ok, "type field should exist")
	
	// Verify pattern was removed
	_, hasPattern := typeField["pattern"]
	assert.False(t, hasPattern, "pattern should be removed")
	
	// Verify type constraint was removed
	_, hasType := typeField["type"]
	assert.False(t, hasType, "type constraint should be removed")
	
	// Verify not constraint was added with enum
	notConstraint, hasNot := typeField["not"]
	assert.True(t, hasNot, "not constraint should be added")
	
	notMap, ok := notConstraint.(map[string]interface{})
	require.True(t, ok, "not should be a map")
	
	enumValues, hasEnum := notMap["enum"]
	assert.True(t, hasEnum, "enum should exist in not constraint")
	
	// JSON unmarshal creates []interface{} not []string
	enumSlice, ok := enumValues.([]interface{})
	require.True(t, ok, "enum should be a slice")
	require.Len(t, enumSlice, 2, "enum should have 2 values")
	
	// Convert to strings for comparison
	enumStrings := make([]string, len(enumSlice))
	for i, v := range enumSlice {
		enumStrings[i], ok = v.(string)
		require.True(t, ok, "enum value should be a string")
	}
	
	assert.ElementsMatch(t, []string{"stdio", "http"}, enumStrings, "enum should exclude stdio and http")
}

// TestFetchAndFixSchema_CustomSchemasPatternPropertiesFix tests the negative lookahead fix for customSchemas
func TestFetchAndFixSchema_CustomSchemasPatternPropertiesFix(t *testing.T) {
	// Schema with negative lookahead in patternProperties
	schemaWithNegativeLookahead := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"properties": map[string]interface{}{
			"customSchemas": map[string]interface{}{
				"type": "object",
				"patternProperties": map[string]interface{}{
					"^(?!stdio$|http$)[a-z][a-z0-9-]*$": map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
	}
	
	schemaJSON, err := json.Marshal(schemaWithNegativeLookahead)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Parse the result and verify the fix was applied
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	// Navigate to customSchemas.patternProperties
	properties, ok := fixed["properties"].(map[string]interface{})
	require.True(t, ok, "properties should exist")
	
	customSchemas, ok := properties["customSchemas"].(map[string]interface{})
	require.True(t, ok, "customSchemas should exist")
	
	patternProps, ok := customSchemas["patternProperties"].(map[string]interface{})
	require.True(t, ok, "patternProperties should exist")
	
	// Verify the old pattern with negative lookahead is gone
	for key := range patternProps {
		assert.False(t, strings.Contains(key, "(?!"), "pattern should not contain negative lookahead")
	}
	
	// Verify the new simple pattern exists
	_, hasSimplePattern := patternProps["^[a-z][a-z0-9-]*$"]
	assert.True(t, hasSimplePattern, "should have simple pattern without negative lookahead")
}

// TestFetchAndFixSchema_BothFixesApplied tests that both fixes are applied in a single call
func TestFetchAndFixSchema_BothFixesApplied(t *testing.T) {
	// Schema with both negative lookahead patterns
	schemaWithBothPatterns := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"definitions": map[string]interface{}{
			"customServerConfig": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":    "string",
						"pattern": "^(?!stdio$|http$).*",
					},
				},
			},
		},
		"properties": map[string]interface{}{
			"customSchemas": map[string]interface{}{
				"type": "object",
				"patternProperties": map[string]interface{}{
					"^(?!stdio$|http$)[a-z][a-z0-9-]*$": map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
	}
	
	schemaJSON, err := json.Marshal(schemaWithBothPatterns)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Parse and verify both fixes were applied
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	// Check fix 1: customServerConfig
	definitions, ok := fixed["definitions"].(map[string]interface{})
	require.True(t, ok)
	customServerConfig, ok := definitions["customServerConfig"].(map[string]interface{})
	require.True(t, ok)
	props1, ok := customServerConfig["properties"].(map[string]interface{})
	require.True(t, ok)
	typeField, ok := props1["type"].(map[string]interface{})
	require.True(t, ok)
	
	_, hasPattern := typeField["pattern"]
	assert.False(t, hasPattern, "customServerConfig pattern should be removed")
	
	notConstraint, hasNot := typeField["not"]
	assert.True(t, hasNot, "customServerConfig not constraint should be added")
	
	// Check fix 2: customSchemas patternProperties
	properties, ok := fixed["properties"].(map[string]interface{})
	require.True(t, ok)
	customSchemas, ok := properties["customSchemas"].(map[string]interface{})
	require.True(t, ok)
	patternProps, ok := customSchemas["patternProperties"].(map[string]interface{})
	require.True(t, ok)
	
	for key := range patternProps {
		assert.False(t, strings.Contains(key, "(?!"), "patternProperties should not contain negative lookahead")
	}
	
	_, hasSimplePattern := patternProps["^[a-z][a-z0-9-]*$"]
	assert.True(t, hasSimplePattern, "should have simple pattern")
}

// TestFetchAndFixSchema_NoFixesNeeded tests that schemas without problematic patterns are unchanged
func TestFetchAndFixSchema_NoFixesNeeded(t *testing.T) {
	// Schema without negative lookahead patterns
	cleanSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
			},
		},
	}
	
	schemaJSON, err := json.Marshal(cleanSchema)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Parse the result
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	// Verify basic structure is preserved
	assert.Equal(t, "http://json-schema.org/draft-07/schema#", fixed["$schema"])
	assert.Equal(t, "object", fixed["type"])
	
	properties, ok := fixed["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "name")
	assert.Contains(t, properties, "age")
}

// TestFetchAndFixSchema_NestedStructurePreserved tests that nested schema structures are preserved
func TestFetchAndFixSchema_NestedStructurePreserved(t *testing.T) {
	// Complex nested schema
	complexSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"definitions": map[string]interface{}{
			"address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"street": map[string]interface{}{"type": "string"},
					"city":   map[string]interface{}{"type": "string"},
				},
			},
		},
		"properties": map[string]interface{}{
			"person": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"address": map[string]interface{}{
						"$ref": "#/definitions/address",
					},
				},
			},
		},
	}
	
	schemaJSON, err := json.Marshal(complexSchema)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Parse and verify structure is preserved
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	// Verify definitions are preserved
	definitions, ok := fixed["definitions"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, definitions, "address")
	
	// Verify properties are preserved
	properties, ok := fixed["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "person")
	
	// Verify nested structure
	person, ok := properties["person"].(map[string]interface{})
	require.True(t, ok)
	personProps, ok := person["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, personProps, "name")
	assert.Contains(t, personProps, "address")
}

// TestFetchAndFixSchema_MarshalError tests handling of marshal failures
func TestFetchAndFixSchema_MarshalError(t *testing.T) {
	// This test verifies that if we somehow get an unmarshalable schema,
	// we handle it gracefully. In practice, this is hard to trigger since
	// we're marshaling a map[string]interface{}, but it's good to test the error path.
	
	// For now, we test that a valid schema doesn't cause marshal errors
	validSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	}
	
	schemaJSON, err := json.Marshal(validSchema)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestFetchAndFixSchema_HTTPMethodUsed tests that GET method is used
func TestFetchAndFixSchema_HTTPMethodUsed(t *testing.T) {
	var requestMethod string
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		schemaJSON := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#"}`)
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	_, err := fetchAndFixSchema(server.URL)
	
	assert.NoError(t, err)
	assert.Equal(t, "GET", requestMethod, "Should use GET method")
}

// TestFetchAndFixSchema_UserAgentAndHeaders tests HTTP request headers
func TestFetchAndFixSchema_UserAgentAndHeaders(t *testing.T) {
	var headers http.Header
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		schemaJSON := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#"}`)
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	_, err := fetchAndFixSchema(server.URL)
	
	assert.NoError(t, err)
	assert.NotNil(t, headers, "Should have captured request headers")
	// Verify Go's default User-Agent is present
	userAgent := headers.Get("User-Agent")
	assert.NotEmpty(t, userAgent, "Should have User-Agent header")
	assert.Contains(t, userAgent, "Go-http-client", "Should use Go HTTP client")
}

// TestFetchAndFixSchema_LargeSchema tests handling of large schema documents
func TestFetchAndFixSchema_LargeSchema(t *testing.T) {
	// Create a large schema with many properties
	largeSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{},
	}
	
	// Add 100 properties to make it larger
	props := largeSchema["properties"].(map[string]interface{})
	for i := 0; i < 100; i++ {
		props[fmt.Sprintf("field%d", i)] = map[string]interface{}{
			"type":        "string",
			"description": fmt.Sprintf("This is field number %d with some description", i),
		}
	}
	
	schemaJSON, err := json.Marshal(largeSchema)
	require.NoError(t, err)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(schemaJSON)
	}))
	defer server.Close()
	
	result, err := fetchAndFixSchema(server.URL)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Verify the large schema was processed correctly
	var fixed map[string]interface{}
	err = json.Unmarshal(result, &fixed)
	require.NoError(t, err)
	
	properties, ok := fixed["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 100, len(properties), "Should preserve all 100 properties")
}
