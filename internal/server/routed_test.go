package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/gh-aw-mcpg/internal/config"
)

// TestCloseEndpoint_Success tests the successful shutdown flow
func TestCloseEndpoint_Success(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
			"fetch":  {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Create test request
	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	w := httptest.NewRecorder()

	// Send request
	httpServer.Handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "Close endpoint should return 200 OK")

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response), "Failed to decode response")

	// Check response fields
	assert.Equal(t, "closed", response["status"], "Expected status 'closed'")
	assert.Equal(t, "Gateway shutdown initiated", response["message"], "Expected shutdown message")

	// Should report 2 servers terminated
	serversTerminated, ok := response["serversTerminated"].(float64)
	require.True(t, ok, "serversTerminated should be a number")
	assert.InDelta(t, 2.0, serversTerminated, 0.01, "Expected 2 servers terminated")

	// Verify server is marked as shutdown
	assert.True(t, us.IsShutdown(), "Expected server to be marked as shutdown")
}

// TestCloseEndpoint_Idempotency tests that subsequent calls return 410 Gone
func TestCloseEndpoint_Idempotency(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// First call
	req1 := httptest.NewRequest(http.MethodPost, "/close", nil)
	w1 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code, "First call should return 200 OK")

	// Second call (should be idempotent)
	req2 := httptest.NewRequest(http.MethodPost, "/close", nil)
	w2 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w2, req2)

	// Should return 410 Gone
	assert.Equal(t, http.StatusGone, w2.Code, "Second call should return 410 Gone")

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&response), "Failed to decode response")

	errMsg, ok := response["error"].(string)
	require.True(t, ok, "Expected error field to be a string")
	assert.Equal(t, "Gateway has already been closed", errMsg, "Expected specific error message")
}

// TestCloseEndpoint_MethodNotAllowed tests that non-POST requests are rejected
func TestCloseEndpoint_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try GET request
	req := httptest.NewRequest(http.MethodGet, "/close", nil)
	w := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "GET request should return 405 Method Not Allowed")
}

// TestCloseEndpoint_RequiresAuth tests that authentication is enforced when configured
func TestCloseEndpoint_RequiresAuth(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	apiKey := "test-secret-key"

	// Create routed mode server with API key
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, apiKey)

	// Request without auth header
	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	w := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Request without auth header should return 401")

	// Request with correct auth header
	req2 := httptest.NewRequest(http.MethodPost, "/close", nil)
	req2.Header.Set("Authorization", apiKey)
	w2 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code, "Request with correct auth should return 200")
}

func TestCreateFilteredServer_ToolFiltering(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Add test tools - Handler is not tested directly, just use nil
	us.toolsMu.Lock()
	us.tools["github___issue_read"] = &ToolInfo{
		Name:        "github___issue_read",
		Description: "Read an issue",
		BackendID:   "github",
		Handler:     nil,
	}
	us.tools["github___repo_list"] = &ToolInfo{
		Name:        "github___repo_list",
		Description: "List repos",
		BackendID:   "github",
		Handler:     nil,
	}
	us.tools["fetch___get"] = &ToolInfo{
		Name:        "fetch___get",
		Description: "Fetch URL",
		BackendID:   "fetch",
		Handler:     nil,
	}
	us.toolsMu.Unlock()

	// Create filtered server for github backend
	filteredServer := createFilteredServer(us, "github")

	// We can't easily inspect the filtered server's tools without SDK internals,
	// but we can verify GetToolsForBackend returns correct filtered list
	tools := us.GetToolsForBackend("github")
	assert.Len(t, tools, 2, "Expected 2 tools for github backend")

	// Verify tool names have prefix stripped
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	assert.Contains(t, toolNames, "issue_read", "Expected tool 'issue_read' to be present")
	assert.Contains(t, toolNames, "repo_list", "Expected tool 'repo_list' to be present")
	assert.NotContains(t, toolNames, "get", "Tool 'get' from fetch backend should not be in github filtered server")

	_ = filteredServer // Use variable to avoid unused error
}

func TestGetToolHandler(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create a mock handler with correct signature
	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, state, nil
	}

	// Add test tool with handler
	us.toolsMu.Lock()
	us.tools["github___test_tool"] = &ToolInfo{
		Name:        "github___test_tool",
		Description: "Test tool",
		BackendID:   "github",
		Handler:     mockHandler,
	}
	us.toolsMu.Unlock()

	// Test retrieval with non-prefixed name (routed mode format)
	handler := us.GetToolHandler("github", "test_tool")
	require.NotNil(t, handler, "GetToolHandler() returned nil for non-prefixed tool name")

	// Test non-existent tool
	handler = us.GetToolHandler("github", "nonexistent_tool")
	assert.Nil(t, handler, "GetToolHandler() should return nil for non-existent tool")

	// Test wrong backend (test_tool belongs to github, not fetch)
	handler = us.GetToolHandler("fetch", "test_tool")
	assert.Nil(t, handler, "GetToolHandler() should return nil when backend doesn't match")
}

func TestCreateHTTPServerForRoutedMode_ServerIDs(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
			"fetch":  {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:8000", us, "")
	require.NotNil(t, httpServer, "CreateHTTPServerForRoutedMode() returned nil")

	// Verify server IDs are correctly set up
	serverIDs := us.GetServerIDs()
	assert.Len(t, serverIDs, 2, "Expected 2 server IDs")

	assert.Contains(t, serverIDs, "github", "Expected 'github' server ID")
	assert.Contains(t, serverIDs, "fetch", "Expected 'fetch' server ID")
}

func TestRoutedMode_SysToolsBackend_DIFCDisabled(t *testing.T) {
	// When DIFC is disabled (default), sys tools should NOT be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: false, // Explicitly disable DIFC (this is the default)
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Verify sys tools are NOT registered when DIFC is disabled
	sysTools := us.GetToolsForBackend("sys")
	assert.Empty(t, sysTools, "Expected no sys tools when DIFC is disabled")
}

func TestRoutedMode_SysToolsBackend_DIFCEnabled(t *testing.T) {
	// When DIFC is enabled, sys tools SHOULD be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: true, // Enable DIFC
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Verify sys tools exist when DIFC is enabled
	sysTools := us.GetToolsForBackend("sys")
	assert.NotEmpty(t, sysTools, "Expected sys tools to be registered when DIFC is enabled")

	// Check for expected sys tools
	toolNames := make([]string, 0, len(sysTools))
	for _, tool := range sysTools {
		toolNames = append(toolNames, tool.Name)
	}

	expectedSysTools := []string{"init", "list_servers"}
	for _, expectedTool := range expectedSysTools {
		assert.Contains(t, toolNames, expectedTool, "Expected sys tool '%s'", expectedTool)
	}

	// Verify sys tools have correct backend ID
	for _, tool := range sysTools {
		assert.Equal(t, "sys", tool.BackendID, "Expected BackendID 'sys' for tool '%s'", tool.Name)
	}
}

func TestRoutedMode_SysRouteNotExposed_DIFCDisabled(t *testing.T) {
	// When DIFC is disabled (default), /mcp/sys route should NOT be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		// EnableDIFC defaults to false, but explicitly set here for test clarity
		EnableDIFC: false,
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try to access /mcp/sys route - should get 404
	req := httptest.NewRequest(http.MethodGet, "/mcp/sys", nil)
	req.Header.Set("Authorization", "test-session")
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Should return 404 because the route is not registered
	assert.Equal(t, http.StatusNotFound, w.Code, "Expected 404 for /mcp/sys when DIFC is disabled")
}

func TestRoutedMode_SysRouteExposed_DIFCEnabled(t *testing.T) {
	// When DIFC is enabled, /mcp/sys route SHOULD be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: true, // Enable DIFC
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try to access /mcp/sys route - should NOT get 404
	req := httptest.NewRequest(http.MethodGet, "/mcp/sys", nil)
	req.Header.Set("Authorization", "test-session")
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Should NOT return 404 because the route should be registered
	assert.NotEqual(t, http.StatusNotFound, w.Code, "Expected /mcp/sys route to be registered when DIFC is enabled")
}

// TestCloseEndpoint_EdgeCases tests edge cases for the close endpoint
func TestCloseEndpoint_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		authHeader     string
		apiKey         string
		expectedStatus int
		description    string
	}{
		{
			name:           "PUT method not allowed",
			method:         http.MethodPut,
			authHeader:     "",
			apiKey:         "",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "PUT requests should be rejected",
		},
		{
			name:           "DELETE method not allowed",
			method:         http.MethodDelete,
			authHeader:     "",
			apiKey:         "",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "DELETE requests should be rejected",
		},
		{
			name:           "PATCH method not allowed",
			method:         http.MethodPatch,
			authHeader:     "",
			apiKey:         "",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "PATCH requests should be rejected",
		},
		{
			name:           "Missing auth with API key configured",
			method:         http.MethodPost,
			authHeader:     "",
			apiKey:         "secret-key",
			expectedStatus: http.StatusUnauthorized,
			description:    "Missing auth header should be rejected when API key is configured",
		},
		{
			name:           "Wrong auth key",
			method:         http.MethodPost,
			authHeader:     "wrong-key",
			apiKey:         "secret-key",
			expectedStatus: http.StatusUnauthorized,
			description:    "Wrong auth key should be rejected",
		},
		{
			name:           "Empty auth header with API key configured",
			method:         http.MethodPost,
			authHeader:     "",
			apiKey:         "secret-key",
			expectedStatus: http.StatusUnauthorized,
			description:    "Empty auth header should be rejected when API key is configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {Command: "docker", Args: []string{}},
				},
			}

			ctx := context.Background()
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err, "NewUnified() failed")
			defer us.Close()

			// Enable test mode to prevent os.Exit()
			us.SetTestMode(true)

			// Create routed mode server with or without API key
			httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, tt.apiKey)

			// Create test request
			req := httptest.NewRequest(tt.method, "/close", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			// Send request
			httpServer.Handler.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

// TestCreateFilteredServer_EdgeCases tests edge cases for filtered server creation
func TestCreateFilteredServer_EdgeCases(t *testing.T) {
	t.Run("empty backend has no tools", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{},
		}

		ctx := context.Background()
		us, err := NewUnified(ctx, cfg)
		require.NoError(t, err, "NewUnified() failed")
		defer us.Close()

		// Create filtered server for non-existent backend
		filteredServer := createFilteredServer(us, "nonexistent")
		assert.NotNil(t, filteredServer, "Should create server even for empty backend")

		// Verify no tools for non-existent backend
		tools := us.GetToolsForBackend("nonexistent")
		assert.Empty(t, tools, "Expected no tools for non-existent backend")
	})

	t.Run("multiple backends don't interfere", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{},
		}

		ctx := context.Background()
		us, err := NewUnified(ctx, cfg)
		require.NoError(t, err, "NewUnified() failed")
		defer us.Close()

		// Add tools for multiple backends
		us.toolsMu.Lock()
		us.tools["backend1___tool1"] = &ToolInfo{
			Name:        "backend1___tool1",
			Description: "Tool 1",
			BackendID:   "backend1",
			Handler:     nil,
		}
		us.tools["backend2___tool2"] = &ToolInfo{
			Name:        "backend2___tool2",
			Description: "Tool 2",
			BackendID:   "backend2",
			Handler:     nil,
		}
		us.tools["backend1___tool3"] = &ToolInfo{
			Name:        "backend1___tool3",
			Description: "Tool 3",
			BackendID:   "backend1",
			Handler:     nil,
		}
		us.toolsMu.Unlock()

		// Verify backend1 has only its tools
		backend1Tools := us.GetToolsForBackend("backend1")
		assert.Len(t, backend1Tools, 2, "Backend1 should have 2 tools")

		toolNames := make([]string, 0, len(backend1Tools))
		for _, tool := range backend1Tools {
			toolNames = append(toolNames, tool.Name)
		}
		assert.Contains(t, toolNames, "tool1")
		assert.Contains(t, toolNames, "tool3")
		assert.NotContains(t, toolNames, "tool2", "Backend1 should not have backend2's tool")

		// Verify backend2 has only its tools
		backend2Tools := us.GetToolsForBackend("backend2")
		assert.Len(t, backend2Tools, 1, "Backend2 should have 1 tool")
		assert.Equal(t, "tool2", backend2Tools[0].Name)
	})
}
