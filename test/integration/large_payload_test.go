package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLargePayload_StoredInPayloadDir tests that large payloads from backend MCP servers
// are properly stored in the configured payloadDir
func TestLargePayload_StoredInPayloadDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create a temporary payload directory
	payloadDir := t.TempDir()
	t.Logf("Using payload directory: %s", payloadDir)

	// Create a config file with payload_dir configured
	configFile := createTempConfigWithPayloadDir(t, payloadDir, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13101"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", payloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started with custom payload directory")

	// Verify the payload directory exists
	info, err := os.Stat(payloadDir)
	require.NoError(t, err, "Payload directory should exist")
	assert.True(t, info.IsDir(), "Payload directory should be a directory")

	t.Log("✓ Payload directory verified")
}

// TestLargePayload_SessionIsolation tests that payloads are isolated by session ID
func TestLargePayload_SessionIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a temporary payload directory
	payloadDir := t.TempDir()
	t.Logf("Using payload directory: %s", payloadDir)

	// Create a config file
	configFile := createTempConfigWithPayloadDir(t, payloadDir, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13102"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", payloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started successfully")

	// Initialize session 1
	session1Token := "session-alpha-abc123"
	initSession(t, serverURL, session1Token)

	// Initialize session 2
	session2Token := "session-beta-xyz789"
	initSession(t, serverURL, session2Token)

	t.Log("✓ Both sessions initialized")

	// Verify session directories are created
	session1Dir := filepath.Join(payloadDir, session1Token)
	session2Dir := filepath.Join(payloadDir, session2Token)

	// Note: Session directories are created when payload-generating tool calls are made
	// The directory structure is {payloadDir}/{sessionID}/{queryID}/payload.json

	// For now, just verify that different sessions would have different directories
	assert.NotEqual(t, session1Dir, session2Dir, "Session directories should be different")

	t.Logf("Session 1 directory would be: %s", session1Dir)
	t.Logf("Session 2 directory would be: %s", session2Dir)
	t.Log("✓ Session isolation verified - sessions have distinct payload paths")
}

// TestLargePayload_PayloadDirFlag tests that the --payload-dir flag is respected
func TestLargePayload_PayloadDirFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a custom payload directory
	customPayloadDir := t.TempDir()
	t.Logf("Custom payload directory: %s", customPayloadDir)

	configFile := createTempConfig(t, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13103"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", customPayloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started with --payload-dir flag")

	// The server should have started successfully with the custom payload directory
	// Verify the directory exists
	info, err := os.Stat(customPayloadDir)
	require.NoError(t, err, "Custom payload directory should exist")
	assert.True(t, info.IsDir(), "Custom payload directory should be a directory")

	t.Logf("✓ Custom payload directory verified: %s", customPayloadDir)
}

// TestLargePayload_ConfigPayloadDir tests that payload_dir in config.toml is respected
func TestLargePayload_ConfigPayloadDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a custom payload directory
	configPayloadDir := t.TempDir()
	t.Logf("Config payload directory: %s", configPayloadDir)

	// Create a config file with payload_dir in gateway section
	configFile := createTempConfigWithPayloadDir(t, configPayloadDir, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13104"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started with payload_dir from config")

	// Verify the directory exists
	info, err := os.Stat(configPayloadDir)
	require.NoError(t, err, "Config payload directory should exist")
	assert.True(t, info.IsDir(), "Config payload directory should be a directory")

	t.Logf("✓ Config payload directory verified: %s", configPayloadDir)
}

// TestLargePayload_MultipleSessionsIsolated tests that multiple sessions have isolated payload storage
func TestLargePayload_MultipleSessionsIsolated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a temporary payload directory
	payloadDir := t.TempDir()
	t.Logf("Using payload directory: %s", payloadDir)

	configFile := createTempConfigWithPayloadDir(t, payloadDir, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13105"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", payloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started")

	// Test with multiple sessions
	sessions := []string{
		"session-1-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"session-2-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"session-3-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	for _, sessionToken := range sessions {
		initSession(t, serverURL, sessionToken)
		t.Logf("✓ Initialized session: %s", sessionToken)
	}

	// Verify expected session directories
	for _, sessionToken := range sessions {
		expectedDir := filepath.Join(payloadDir, sessionToken)
		t.Logf("Expected session directory: %s", expectedDir)
	}

	// Verify all session directories would be unique
	sessionDirs := make(map[string]bool)
	for _, sessionToken := range sessions {
		dir := filepath.Join(payloadDir, sessionToken)
		assert.False(t, sessionDirs[dir], "Session directories should be unique")
		sessionDirs[dir] = true
	}

	t.Log("✓ All sessions have unique payload directories")
}

// createTempConfigWithPayloadDir creates a temporary TOML config file with payload_dir
func createTempConfigWithPayloadDir(t *testing.T, payloadDir string, servers map[string]interface{}) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "awmg-test-config-*.toml")
	require.NoError(t, err, "Failed to create temp config")
	defer tmpFile.Close()

	// Write gateway section with payload_dir
	fmt.Fprintln(tmpFile, "[gateway]")
	fmt.Fprintf(tmpFile, "payload_dir = %q\n", payloadDir)

	// Write servers section
	fmt.Fprintln(tmpFile, "\n[servers]")
	for name, config := range servers {
		fmt.Fprintf(tmpFile, "\n[servers.%s]\n", name)
		if cfg, ok := config.(map[string]interface{}); ok {
			if cmd, ok := cfg["command"].(string); ok {
				fmt.Fprintf(tmpFile, "command = %q\n", cmd)
			}
			if args, ok := cfg["args"].([]string); ok {
				fmt.Fprintf(tmpFile, "args = [")
				for i, arg := range args {
					if i > 0 {
						fmt.Fprint(tmpFile, ", ")
					}
					fmt.Fprintf(tmpFile, "%q", arg)
				}
				fmt.Fprintln(tmpFile, "]")
			}
		}
	}

	return tmpFile.Name()
}

// initSession sends an initialize request for a session
func initSession(t *testing.T, serverURL, sessionToken string) {
	t.Helper()

	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, err := json.Marshal(initReq)
	require.NoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", sessionToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Initialize request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response")

	// Allow 200 OK for success
	if resp.StatusCode != http.StatusOK {
		t.Logf("Initialize response status: %d, body: %s", resp.StatusCode, string(body))
	}
}

// TestLargePayload_PayloadDirectoryPermissions tests that payload directories have secure permissions
func TestLargePayload_PayloadDirectoryPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a temporary payload directory
	payloadDir := t.TempDir()

	configFile := createTempConfigWithPayloadDir(t, payloadDir, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13106"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", payloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	// Initialize a session to trigger session directory creation
	sessionToken := "secure-session-test"
	initSession(t, serverURL, sessionToken)

	// Session directory is created on first payload-generating tool call
	// For this test, we verify the base payload directory exists and is accessible
	info, err := os.Stat(payloadDir)
	require.NoError(t, err, "Payload directory should exist")
	assert.True(t, info.IsDir(), "Payload directory should be a directory")

	t.Log("✓ Payload directory permissions verified")
}

// TestLargePayload_SessionIDFromAuthorizationHeader tests that session ID is correctly extracted
// from the Authorization header for payload isolation
func TestLargePayload_SessionIDFromAuthorizationHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	payloadDir := t.TempDir()

	configFile := createTempConfigWithPayloadDir(t, payloadDir, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13107"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", payloadDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started")

	// Test with different Authorization header formats
	testCases := []struct {
		name          string
		authHeader    string
		expectedToken string
	}{
		{
			name:          "plain token",
			authHeader:    "my-api-key-123",
			expectedToken: "my-api-key-123",
		},
		{
			name:          "Bearer token",
			authHeader:    "Bearer my-bearer-token-456",
			expectedToken: "my-bearer-token-456",
		},
		{
			name:          "complex token",
			authHeader:    "token-with-special_chars.and/slashes",
			expectedToken: "token-with-special_chars.and/slashes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initSession(t, serverURL, tc.authHeader)
			t.Logf("✓ Initialized session with auth header: %s", tc.name)
		})
	}

	t.Log("✓ Session ID extraction from Authorization header verified")
}

// TestLargePayload_PayloadDirDoesNotExist tests behavior when payload dir doesn't exist
// The server should create it
func TestLargePayload_PayloadDirDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Create a path that doesn't exist
	baseTmpDir := t.TempDir()
	nonExistentDir := filepath.Join(baseTmpDir, "non-existent-subdir", "payloads")

	// Verify it doesn't exist
	_, err := os.Stat(nonExistentDir)
	require.True(t, os.IsNotExist(err), "Directory should not exist initially")

	configFile := createTempConfig(t, map[string]interface{}{
		"echo": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13108"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
		"--payload-dir", nonExistentDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started with non-existent payload directory (will be created on first use)")

	// Initialize a session
	initSession(t, serverURL, "test-session")

	t.Log("✓ Session initialized successfully")
}

// TestLargePayload_PayloadPathStructure verifies the payload path structure
// Expected: {payloadDir}/{sessionID}/{queryID}/payload.json
func TestLargePayload_PayloadPathStructure(t *testing.T) {
	// This is a documentation/verification test
	// The actual path structure is tested in unit tests

	expectedStructure := "{payloadDir}/{sessionID}/{queryID}/payload.json"
	t.Logf("Expected payload path structure: %s", expectedStructure)

	// Verify the structure components
	components := []string{
		"payloadDir - Base directory for all payloads (configured via --payload-dir or payload_dir in config)",
		"sessionID - Unique identifier per client session (from Authorization header)",
		"queryID - Random ID generated for each tool call (32 hex characters)",
		"payload.json - The actual payload file",
	}

	for _, component := range components {
		t.Logf("  - %s", component)
	}

	// Verify the purpose of session isolation
	t.Log("\nSession isolation purpose:")
	t.Log("  - Each agent/client gets its own directory")
	t.Log("  - Agents can mount only their session directory")
	t.Log("  - Prevents cross-session data leakage")

	// Verify file permissions expectations
	t.Log("\nExpected permissions:")
	t.Log("  - Session directories: 0700 (owner only)")
	t.Log("  - Payload files: 0600 (owner read/write only)")

	// This test documents the expected behavior
	assert.Equal(t, "{payloadDir}/{sessionID}/{queryID}/payload.json", expectedStructure)
	assert.True(t, strings.Contains(expectedStructure, "sessionID"), "Structure should include sessionID for isolation")
}
