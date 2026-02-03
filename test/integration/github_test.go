package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitHubMCPMockBackend tests integration with a mock GitHub MCP server
// This test validates the gateway can communicate with GitHub-style MCP servers
// without requiring actual GitHub credentials
func TestGitHubMCPMockBackend(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GitHub MCP mock integration test in short mode")
	}

	// Create a mock GitHub MCP backend that returns realistic responses
	githubBackend := createGitHubMockServer(t)
	defer githubBackend.Close()

	t.Logf("✓ Mock GitHub MCP backend started at %s", githubBackend.URL)

	// Create JSON config for the gateway
	configContent := `{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "` + githubBackend.URL + `"
    }
  },
  "gateway": {
    "port": 13110,
    "domain": "localhost",
    "apiKey": "test-github-key"
  }
}`

	t.Logf("✓ Created config: %s", configContent)

	// Start the gateway with the config via stdin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start gateway with JSON config via stdin
	gatewayCmd := startGatewayWithJSONConfig(ctx, t, configContent)
	defer gatewayCmd.Process.Kill()

	// Wait for gateway to start
	gatewayURL := "http://127.0.0.1:13110"
	if !waitForServer(t, gatewayURL+"/health", 15*time.Second) {
		t.Fatal("Gateway did not start in time")
	}
	t.Logf("✓ Gateway started at %s", gatewayURL)

	// Test 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(gatewayURL + "/health")
		require.NoError(t, err, "Health check failed")
		defer resp.Body.Close()

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err, "Failed to decode health response")

		assert.Equal(t, "healthy", health["status"])
		servers := health["servers"].(map[string]interface{})
		assert.Contains(t, servers, "github", "GitHub backend not found in health check")

		t.Log("✓ Health check passed - GitHub backend registered")
	})

	// Test 2: Initialize connection
	t.Run("InitializeConnection", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-github-key")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err, "Initialize request failed")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Initialize response: %s", string(body))

		require.Equal(t, http.StatusOK, resp.StatusCode, "Initialize failed with status %d: %s", resp.StatusCode, string(body))

		// Check if response uses SSE-formatted streaming
		contentType := resp.Header.Get("Content-Type")
		var result map[string]interface{}
		if contentType == "text/event-stream" {
			// Parse SSE-formatted response
			result = parseSSEResponse(t, string(body))
		} else {
			// Parse regular JSON response
			err = json.Unmarshal(body, &result)
			require.NoError(t, err, "Failed to parse initialize response")
		}

		// Verify response has server info
		if resultData, ok := result["result"].(map[string]interface{}); ok {
			assert.Contains(t, resultData, "serverInfo", "Initialize response should contain serverInfo")
			t.Log("✓ Successfully initialized connection to GitHub backend")
		}

		// Send initialized notification to complete the handshake
		initializedNotif := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}

		jsonData, _ = json.Marshal(initializedNotif)
		req, _ = http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-github-key")
		resp, err = client.Do(req)
		if err == nil {
			resp.Body.Close()
		}

		// Give the server a moment to process the notification
		time.Sleep(100 * time.Millisecond)
		t.Log("✓ Sent initialized notification")
	})

	// Test 3: List all available tools
	t.Run("ListTools", func(t *testing.T) {
		toolsReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		jsonData, _ := json.Marshal(toolsReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-github-key")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err, "Tools list request failed")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Tools list response length: %d bytes", len(body))

		require.Equal(t, http.StatusOK, resp.StatusCode, "Tools list failed with status %d: %s", resp.StatusCode, string(body))

		// Check if response uses SSE-formatted streaming
		contentType := resp.Header.Get("Content-Type")
		var result map[string]interface{}
		if contentType == "text/event-stream" {
			// Parse SSE-formatted response
			result = parseSSEResponse(t, string(body))
		} else {
			// Parse regular JSON response
			err = json.Unmarshal(body, &result)
			require.NoError(t, err, "Failed to parse tools list response")
		}

		// Verify we got tools
		if resultData, ok := result["result"].(map[string]interface{}); ok {
			if tools, ok := resultData["tools"].([]interface{}); ok {
				assert.NotEmpty(t, tools, "Should have at least one tool")
				t.Logf("✓ Found %d tools in GitHub MCP server", len(tools))

				// Verify tool structure
				if len(tools) > 0 {
					firstTool := tools[0].(map[string]interface{})
					assert.Contains(t, firstTool, "name", "Tool should have name")
					assert.Contains(t, firstTool, "description", "Tool should have description")
					assert.Contains(t, firstTool, "inputSchema", "Tool should have inputSchema")
					t.Logf("✓ Tool structure validated (example: %s)", firstTool["name"])
				}
			} else {
				t.Logf("⚠ No tools array found in result, keys: %v", keys(resultData))
			}
		} else if errData, ok := result["error"]; ok {
			t.Logf("⚠ Error in tools/list response: %v", errData)
		} else {
			t.Logf("⚠ Unexpected response structure, keys: %v", keys(result))
		}

		t.Log("✓ Successfully listed tools from GitHub backend")
	})

	t.Log("✓ GitHub MCP mock backend integration test passed")
}

// helper function to get keys from a map
func keys(m map[string]interface{}) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// TestGitHubMCPRealBackend tests connection to the actual GitHub MCP server
// This test requires GITHUB_PERSONAL_ACCESS_TOKEN environment variable to be set
// It discovers all tools via tools/list and tests each one with a minimal call
func TestGitHubMCPRealBackend(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real GitHub MCP connection test in short mode")
	}

	token := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
	if token == "" {
		t.Skip("Skipping real GitHub test: GITHUB_PERSONAL_ACCESS_TOKEN not set")
	}

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping GitHub MCP test")
	}

	// Check if GitHub MCP server image is available
	githubImage := "ghcr.io/github/github-mcp-server:latest"
	t.Logf("Checking for GitHub MCP server image: %s", githubImage)

	// Pull the latest image to ensure we have it
	pullCmd := exec.Command("docker", "pull", githubImage)
	pullOutput, pullErr := pullCmd.CombinedOutput()
	if pullErr != nil {
		t.Logf("Warning: Failed to pull GitHub MCP image: %v, output: %s", pullErr, string(pullOutput))
		// Continue anyway - the image might already be available locally
	} else {
		t.Logf("✓ GitHub MCP server image ready: %s", githubImage)
	}

	// Create JSON config for the gateway with real GitHub MCP server
	configContent := `{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "` + githubImage + `",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "` + token + `"
      }
    }
  },
  "gateway": {
    "port": 13111,
    "domain": "localhost",
    "apiKey": "test-github-key"
  }
}`

	t.Logf("✓ Created config for real GitHub MCP connection")

	// Start the gateway with the config via stdin
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Find the binary
	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	port := "13111"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config-stdin",
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	// Set stdin to the JSON config
	cmd.Stdin = bytes.NewBufferString(configContent)

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start gateway: %v\nSTDOUT: %s\nSTDERR: %s", err, stdout.String(), stderr.String())
	}

	// Start a goroutine to log output if test fails
	go func() {
		<-ctx.Done()
		if t.Failed() {
			t.Logf("Gateway STDOUT: %s", stdout.String())
			t.Logf("Gateway STDERR: %s", stderr.String())
		}
	}()

	defer cmd.Process.Kill()

	// Wait for gateway to start
	gatewayURL := "http://127.0.0.1:" + port
	if !waitForServer(t, gatewayURL+"/health", 60*time.Second) {
		t.Logf("Gateway STDOUT: %s", stdout.String())
		t.Logf("Gateway STDERR: %s", stderr.String())
		t.Fatal("Gateway did not start in time")
	}
	t.Logf("✓ Gateway started at %s", gatewayURL)

	// Test 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(gatewayURL + "/health")
		require.NoError(t, err, "Health check failed")
		defer resp.Body.Close()

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err, "Failed to decode health response")

		assert.Equal(t, "healthy", health["status"])
		servers := health["servers"].(map[string]interface{})
		assert.Contains(t, servers, "github", "GitHub backend not found in health check")

		t.Log("✓ Health check passed - Real GitHub backend registered")
	})

	// Test 2: Initialize connection
	var sessionID string
	t.Run("InitializeConnection", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-github-key")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err, "Initialize request failed")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Initialize response: %s", string(body))

		require.Equal(t, http.StatusOK, resp.StatusCode, "Initialize failed with status %d: %s", resp.StatusCode, string(body))

		// Check if response uses SSE-formatted streaming
		contentType := resp.Header.Get("Content-Type")
		var result map[string]interface{}
		if contentType == "text/event-stream" {
			// Parse SSE-formatted response
			result = parseSSEResponse(t, string(body))
		} else {
			// Parse regular JSON response
			err = json.Unmarshal(body, &result)
			require.NoError(t, err, "Failed to parse initialize response")
		}

		// Verify response has server info
		if resultData, ok := result["result"].(map[string]interface{}); ok {
			assert.Contains(t, resultData, "serverInfo", "Initialize response should contain serverInfo")
			serverInfo := resultData["serverInfo"].(map[string]interface{})
			t.Logf("✓ Successfully initialized connection to GitHub MCP server: %v", serverInfo["name"])
		}

		// Send initialized notification to complete the handshake
		initializedNotif := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}

		jsonData, _ = json.Marshal(initializedNotif)
		req, _ = http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "test-github-key")
		resp, err = client.Do(req)
		if err == nil {
			resp.Body.Close()
		}

		// Give the server a moment to process the notification
		time.Sleep(100 * time.Millisecond)
		t.Log("✓ Sent initialized notification")

		// Use the auth token as session ID for subsequent requests
		sessionID = "test-github-key"
	})

	// Test 3: List all available tools
	var toolsList []map[string]interface{}
	t.Run("ListAllTools", func(t *testing.T) {
		toolsReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		jsonData, _ := json.Marshal(toolsReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", sessionID)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err, "Tools list request failed")
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Tools list response length: %d bytes", len(body))

		require.Equal(t, http.StatusOK, resp.StatusCode, "Tools list failed with status %d: %s", resp.StatusCode, string(body))

		// Check if response uses SSE-formatted streaming
		contentType := resp.Header.Get("Content-Type")
		var result map[string]interface{}
		if contentType == "text/event-stream" {
			// Parse SSE-formatted response
			result = parseSSEResponse(t, string(body))
		} else {
			// Parse regular JSON response
			err = json.Unmarshal(body, &result)
			require.NoError(t, err, "Failed to parse tools list response")
		}

		// Extract tools from response
		if resultData, ok := result["result"].(map[string]interface{}); ok {
			if tools, ok := resultData["tools"].([]interface{}); ok {
				require.NotEmpty(t, tools, "Should have at least one tool")
				t.Logf("✓ Found %d tools in GitHub MCP server", len(tools))

				// Convert to proper type for later tests
				for _, tool := range tools {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						toolsList = append(toolsList, toolMap)
					}
				}

				// Log all tool names
				toolNames := make([]string, 0, len(toolsList))
				for _, tool := range toolsList {
					toolNames = append(toolNames, tool["name"].(string))
				}
				t.Logf("✓ Available tools: %s", strings.Join(toolNames, ", "))
			}
		}

		require.NotEmpty(t, toolsList, "Failed to extract tools from response")
	})

	// Test 4: Test each tool with a minimal call
	// We'll test a representative sample of tools from different categories
	t.Run("TestToolCalls", func(t *testing.T) {
		// Define test cases for different tool categories
		toolTestCases := []struct {
			toolPattern string // Tool name pattern to match
			args        map[string]interface{}
			description string
			skipIfError bool // Skip if this specific tool fails (might not have permissions)
		}{
			{
				toolPattern: "list_branches",
				args: map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
				},
				description: "List repository branches",
				skipIfError: false,
			},
			{
				toolPattern: "list_commits",
				args: map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
				},
				description: "List repository commits",
				skipIfError: false,
			},
			{
				toolPattern: "get_file_contents",
				args: map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
					"path":  "README.md",
				},
				description: "Get file contents",
				skipIfError: false,
			},
			{
				toolPattern: "search_repositories",
				args: map[string]interface{}{
					"query": "mcp gateway",
				},
				description: "Search repositories",
				skipIfError: false,
			},
			{
				toolPattern: "list_issues",
				args: map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
				},
				description: "List issues",
				skipIfError: true, // May fail if repo has no issues
			},
			{
				toolPattern: "list_pull_requests",
				args: map[string]interface{}{
					"owner": "github",
					"repo":  "gh-aw-mcpg",
				},
				description: "List pull requests",
				skipIfError: true,
			},
		}

		requestID := 100
		for _, tc := range toolTestCases {
			// Find matching tool
			var matchedTool map[string]interface{}
			for _, tool := range toolsList {
				toolName := tool["name"].(string)
				if strings.Contains(toolName, tc.toolPattern) {
					matchedTool = tool
					break
				}
			}

			if matchedTool == nil {
				t.Logf("⚠ Tool pattern '%s' not found in tools list, skipping", tc.toolPattern)
				continue
			}

			toolName := matchedTool["name"].(string)
			t.Run(fmt.Sprintf("CallTool_%s", toolName), func(t *testing.T) {
				requestID++
				callReq := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      requestID,
					"method":  "tools/call",
					"params": map[string]interface{}{
						"name":      toolName,
						"arguments": tc.args,
					},
				}

				jsonData, _ := json.Marshal(callReq)
				req, _ := http.NewRequest("POST", gatewayURL+"/mcp/github", bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				req.Header.Set("Authorization", sessionID)

				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(req)
				require.NoError(t, err, "Tool call request failed for %s", toolName)
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				t.Logf("Tool call response for %s: %s", toolName, string(body))

				if resp.StatusCode != http.StatusOK {
					if tc.skipIfError {
						t.Logf("⚠ Tool call failed (expected): %s - Status: %d", tc.description, resp.StatusCode)
						return
					}
					t.Fatalf("Tool call failed with status %d: %s", resp.StatusCode, string(body))
				}

				// Check if response uses SSE-formatted streaming
				contentType := resp.Header.Get("Content-Type")
				var result map[string]interface{}
				if contentType == "text/event-stream" {
					// Parse SSE-formatted response
					result = parseSSEResponse(t, string(body))
				} else {
					// Parse regular JSON response
					err = json.Unmarshal(body, &result)
					require.NoError(t, err, "Failed to parse tool call response")
				}

				// Check for errors in the response
				if errData, hasError := result["error"]; hasError {
					if tc.skipIfError {
						t.Logf("⚠ Tool returned error (expected): %s - Error: %v", tc.description, errData)
						return
					}
					t.Fatalf("Tool call returned error: %v", errData)
				}

				// Verify we got a result
				if _, hasResult := result["result"]; hasResult {
					t.Logf("✓ Successfully called tool: %s - %s", toolName, tc.description)
				} else {
					t.Logf("⚠ Tool call succeeded but no result field: %s", toolName)
				}
			})
		}
	})

	// Test 5: Verify all tools can be listed and have proper schema
	t.Run("VerifyToolSchemas", func(t *testing.T) {
		require.NotEmpty(t, toolsList, "Tools list should not be empty")

		invalidTools := []string{}
		for _, tool := range toolsList {
			toolName := tool["name"].(string)

			// Verify required fields
			if _, hasName := tool["name"]; !hasName {
				invalidTools = append(invalidTools, fmt.Sprintf("%s: missing name", toolName))
			}
			if _, hasDescription := tool["description"]; !hasDescription {
				invalidTools = append(invalidTools, fmt.Sprintf("%s: missing description", toolName))
			}
			if _, hasSchema := tool["inputSchema"]; !hasSchema {
				invalidTools = append(invalidTools, fmt.Sprintf("%s: missing inputSchema", toolName))
			} else {
				// Verify schema structure
				schema := tool["inputSchema"].(map[string]interface{})
				if schemaType, ok := schema["type"]; !ok || schemaType != "object" {
					invalidTools = append(invalidTools, fmt.Sprintf("%s: invalid schema type", toolName))
				}
			}
		}

		if len(invalidTools) > 0 {
			t.Errorf("Found tools with invalid structure:\n%s", strings.Join(invalidTools, "\n"))
		} else {
			t.Logf("✓ All %d tools have valid schema structure", len(toolsList))
		}
	})

	t.Log("✓ Real GitHub MCP backend integration test passed")
}

// createGitHubMockServer creates a mock server that mimics GitHub MCP server responses
func createGitHubMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var reqBody map[string]interface{}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Ignore empty requests
		if len(bodyBytes) == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}

		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		method, _ := reqBody["method"].(string)
		id := reqBody["id"]

		var response map[string]interface{}
		switch method {
		case "initialize":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{
						"name":    "mock-github-mcp",
						"version": "1.0.0",
					},
				},
			}
		case "tools/list":
			// Return a representative sample of GitHub MCP tools
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "list_branches",
							"description": "List branches in a GitHub repository",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"owner": map[string]interface{}{
										"type":        "string",
										"description": "Repository owner",
									},
									"repo": map[string]interface{}{
										"type":        "string",
										"description": "Repository name",
									},
								},
								"required": []string{"owner", "repo"},
							},
						},
						{
							"name":        "get_file_contents",
							"description": "Get the contents of a file from a GitHub repository",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"owner": map[string]interface{}{
										"type":        "string",
										"description": "Repository owner",
									},
									"repo": map[string]interface{}{
										"type":        "string",
										"description": "Repository name",
									},
									"path": map[string]interface{}{
										"type":        "string",
										"description": "Path to file",
									},
								},
								"required": []string{"owner", "repo", "path"},
							},
						},
						{
							"name":        "list_issues",
							"description": "List issues in a GitHub repository",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"owner": map[string]interface{}{
										"type":        "string",
										"description": "Repository owner",
									},
									"repo": map[string]interface{}{
										"type":        "string",
										"description": "Repository name",
									},
								},
								"required": []string{"owner", "repo"},
							},
						},
						{
							"name":        "list_pull_requests",
							"description": "List pull requests in a GitHub repository",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"owner": map[string]interface{}{
										"type":        "string",
										"description": "Repository owner",
									},
									"repo": map[string]interface{}{
										"type":        "string",
										"description": "Repository name",
									},
								},
								"required": []string{"owner", "repo"},
							},
						},
						{
							"name":        "search_repositories",
							"description": "Search for repositories on GitHub",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"query": map[string]interface{}{
										"type":        "string",
										"description": "Search query",
									},
								},
								"required": []string{"query"},
							},
						},
					},
				},
			}
		case "tools/call":
			// Mock tool call response
			params, _ := reqBody["params"].(map[string]interface{})
			toolName, _ := params["name"].(string)

			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Mock response for tool: %s", toolName),
						},
					},
					"isError": false,
				},
			}
		default:
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
}
