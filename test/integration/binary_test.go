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

	"github.com/stretchr/testify/require"
	"time"
)

// TestBinaryInvocation_RoutedMode tests the awmg binary in routed mode
func TestBinaryInvocation_RoutedMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	// Find the binary
	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create a temporary config file
	configFile := createTempConfig(t, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	// Start the server process
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13001" // Use a specific port for testing
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Ensure the process is killed at the end
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Wait for server to start
	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started successfully")

	// Test 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/health")
		require.NoError(t, err, "Health check failed")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		t.Log("✓ Health check passed")
	})

	// Test 2: Initialize request to routed endpoint
	t.Run("InitializeRouted", func(t *testing.T) {
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

		resp := sendMCPRequest(t, serverURL+"/mcp/testserver", "test-token", initReq)

		// Verify response structure
		if resp["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc 2.0, got %v", resp["jsonrpc"])
		}

		if errObj, hasError := resp["error"]; hasError {
			t.Logf("Response error: %v", errObj)
		}

		t.Log("✓ Initialize request completed")
	})

	// Capture final logs for debugging
	t.Logf("Server output:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
}

// TestBinaryInvocation_UnifiedMode tests the awmg binary in unified mode
func TestBinaryInvocation_UnifiedMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	configFile := createTempConfig(t, map[string]interface{}{
		"backend1": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
		"backend2": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13002"
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

	t.Log("✓ Server started successfully")

	// Test unified endpoint
	t.Run("InitializeUnified", func(t *testing.T) {
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

		resp := sendMCPRequest(t, serverURL+"/mcp", "test-token", initReq)

		if resp["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc 2.0, got %v", resp["jsonrpc"])
		}

		t.Log("✓ Unified mode initialize request completed")
	})

	t.Logf("Server output:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
}

// TestBinaryInvocation_ConfigStdin tests reading config from stdin
func TestBinaryInvocation_ConfigStdin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := "13003"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config-stdin",
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	// Prepare config JSON for stdin
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"testserver": map[string]interface{}{
				"type":      "local",
				"container": "echo",
			},
		},
		"gateway": map[string]interface{}{
			"port":   13003,
			"domain": "localhost",
			"apiKey": "test-key",
		},
	}
	configBytes, _ := json.Marshal(configJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(configBytes)
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

	t.Log("✓ Server started with stdin config")

	// Test health check
	resp, err := http.Get(serverURL + "/health")
	require.NoError(t, err, "Health check failed")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Log("✓ Config from stdin test passed")
	t.Logf("Server output:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
}

// TestBinaryInvocation_PipeOutput tests that stdout pipe output works correctly
// This validates the fix for "sync /dev/stdout: invalid argument" error
func TestBinaryInvocation_PipeOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create a simple config
	configFile := createTempConfig(t, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := "13004"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--unified",
	)

	// Capture stdout through a pipe (the scenario we're testing)
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

	// Wait for server to start
	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 5*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started successfully with piped stdout")

	// Small delay to ensure stdout is written
	time.Sleep(100 * time.Millisecond)

	// Parse the JSON gateway configuration from stdout
	stdoutStr := stdout.String()
	if stdoutStr == "" {
		t.Fatal("Expected JSON configuration on stdout, got empty output")
	}

	// Find the JSON object in stdout (it may be mixed with other output)
	var gatewayConfig map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&gatewayConfig); err != nil {
		t.Fatalf("Failed to parse JSON from stdout: %v\nOutput: %s", err, stdoutStr)
	}

	// Verify the gateway configuration structure
	mcpServers, ok := gatewayConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'mcpServers' field in output, got: %v", gatewayConfig)
	}

	testserver, ok := mcpServers["testserver"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'testserver' in mcpServers, got: %v", mcpServers)
	}

	// Verify the server config has expected fields
	if serverType, ok := testserver["type"].(string); !ok || serverType != "http" {
		t.Errorf("Expected type 'http', got: %v", testserver["type"])
	}

	if url, ok := testserver["url"].(string); !ok || url == "" {
		t.Errorf("Expected non-empty url, got: %v", testserver["url"])
	}

	t.Log("✓ Gateway configuration JSON successfully written to piped stdout")

	// Verify no sync errors in stderr
	stderrStr := stderr.String()
	if bytes.Contains([]byte(stderrStr), []byte("failed to flush stdout")) {
		t.Errorf("Found stdout flush error in stderr: %s", stderrStr)
	}
	if bytes.Contains([]byte(stderrStr), []byte("sync /dev/stdout: invalid argument")) {
		t.Errorf("Found sync error in stderr: %s", stderrStr)
	}

	t.Log("✓ No sync errors in output")
	t.Logf("Parsed gateway config: %+v", gatewayConfig)
}

// TestBinaryInvocation_PipeInputOutput tests both stdin config input and stdout JSON output through pipes
// This validates the complete pipe scenario: config via stdin, JSON output via stdout
func TestBinaryInvocation_PipeInputOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := "13005"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config-stdin",
		"--listen", "127.0.0.1:"+port,
		"--unified",
	)

	// Prepare config JSON for stdin pipe
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"pipetest": map[string]interface{}{
				"type":      "local",
				"container": "echo",
			},
		},
		"gateway": map[string]interface{}{
			"port":   13005,
			"domain": "localhost",
			"apiKey": "test-key",
		},
	}
	configBytes, err := json.Marshal(configJSON)
	require.NoError(t, err, "Failed to marshal config")

	// Setup pipes for both stdin and stdout
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(configBytes)
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

	// Wait for server to start
	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 5*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started with config from stdin pipe")

	// Small delay to ensure stdout is written
	time.Sleep(100 * time.Millisecond)

	// Parse the JSON gateway configuration from stdout pipe
	stdoutStr := stdout.String()
	if stdoutStr == "" {
		t.Fatal("Expected JSON configuration on stdout, got empty output")
	}

	var gatewayConfig map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&gatewayConfig); err != nil {
		t.Fatalf("Failed to parse JSON from stdout: %v\nOutput: %s", err, stdoutStr)
	}

	// Verify the gateway configuration structure
	mcpServers, ok := gatewayConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'mcpServers' field in output, got: %v", gatewayConfig)
	}

	pipetest, ok := mcpServers["pipetest"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'pipetest' server in mcpServers, got: %v", mcpServers)
	}

	// Verify server config
	if serverType, ok := pipetest["type"].(string); !ok || serverType != "http" {
		t.Errorf("Expected type 'http', got: %v", pipetest["type"])
	}

	t.Log("✓ Gateway configuration JSON successfully written to stdout pipe")
	t.Log("✓ Config input via stdin pipe and JSON output via stdout pipe both working")

	// Verify no sync errors
	stderrStr := stderr.String()
	if bytes.Contains([]byte(stderrStr), []byte("failed to flush stdout")) ||
		bytes.Contains([]byte(stderrStr), []byte("sync /dev/stdout: invalid argument")) {
		t.Errorf("Found sync error in stderr: %s", stderrStr)
	}

	t.Log("✓ No pipe-related sync errors")
	t.Logf("Parsed gateway config: %+v", gatewayConfig)
}

// TestBinaryInvocation_NoConfigRequired tests that the binary requires either --config or --config-stdin
func TestBinaryInvocation_NoConfigRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	// Test running without any config flag
	cmd := exec.Command(binaryPath)
	output, err := cmd.CombinedOutput()

	// Should exit with error
	require.Error(t, err)

	outputStr := string(output)
	// Should contain the error message about requiring config
	if !bytes.Contains(output, []byte("configuration source required")) {
		t.Errorf("Expected 'configuration source required' error message, got: %s", outputStr)
	}

	// Should mention both --config and --config-stdin
	if !bytes.Contains(output, []byte("--config")) || !bytes.Contains(output, []byte("--config-stdin")) {
		t.Errorf("Expected error message to mention both --config and --config-stdin, got: %s", outputStr)
	}

	t.Logf("✓ Binary correctly requires config source: %s", outputStr)
}

// TestBinaryInvocation_Version tests the version flag
func TestBinaryInvocation_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)

	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to get version")

	outputStr := string(output)
	if outputStr == "" {
		t.Error("Version output is empty")
	}

	t.Logf("✓ Version output: %s", outputStr)
}

// Helper functions

// findBinary locates the awmg binary
func findBinary(t *testing.T) string {
	t.Helper()

	// Look for binary in common locations
	locations := []string{
		"./awmg",        // Current directory
		"../../awmg",    // From test/integration
		"../../../awmg", // Alternative path
	}

	// Also check in PATH
	if path, err := exec.LookPath("awmg"); err == nil {
		locations = append([]string{path}, locations...)
	}

	for _, loc := range locations {
		absPath, err := filepath.Abs(loc)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	t.Skip("Could not find awmg binary. Run 'make build' first.")
	return ""
}

// createTempConfig creates a temporary TOML config file
func createTempConfig(t *testing.T, servers map[string]interface{}) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "awmg-test-config-*.toml")
	require.NoError(t, err, "Failed to create temp config")
	defer tmpFile.Close()

	// Write TOML format
	fmt.Fprintln(tmpFile, "[servers]")
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

// killProcessOnPort kills any process listening on the specified port
// This helps prevent test interference from stale server processes
func killProcessOnPort(t *testing.T, port string) {
	t.Helper()

	// Use lsof to find processes listening on the port (macOS/Linux)
	cmd := exec.Command("lsof", "-ti", "tcp:"+port)
	output, err := cmd.Output()
	if err != nil {
		// No process found on port, which is fine
		return
	}

	// Kill each PID found
	pids := strings.TrimSpace(string(output))
	if pids == "" {
		return
	}

	for _, pid := range strings.Split(pids, "\n") {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		killCmd := exec.Command("kill", "-9", pid)
		if err := killCmd.Run(); err != nil {
			t.Logf("Warning: failed to kill process %s on port %s: %v", pid, port, err)
		} else {
			t.Logf("Killed stale process %s on port %s", pid, port)
		}
	}

	// Give the OS a moment to release the port
	time.Sleep(100 * time.Millisecond)
}

// waitForServer waits for the server to become available
func waitForServer(t *testing.T, url string, timeout time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// sendMCPRequest sends an MCP request and returns the response
// The authToken parameter can be:
// - Plain API key when API key authentication is configured
// - Session ID for session tracking (can include "Bearer " prefix for backward compatibility)
func sendMCPRequest(t *testing.T, url string, authToken string, payload map[string]interface{}) map[string]interface{} {
	t.Helper()

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	// Per spec 7.1: Authorization header contains value directly (not Bearer scheme)
	// For session tracking without auth, Bearer prefix is optional
	req.Header.Set("Authorization", authToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response")

	if resp.StatusCode != http.StatusOK {
		t.Logf("Response status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Check if response uses SSE-formatted streaming (part of streamable HTTP)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "text/event-stream" {
		// Parse SSE-formatted response
		return parseSSEResponse(t, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Logf("Response body: %s", string(body))
		t.Fatalf("Failed to decode response: %v", err)
	}

	return result
}

// parseSSEResponse parses Server-Sent Events formatted responses and extracts the JSON data
// Note: SSE formatting is used by streamable HTTP transport for streaming responses
func parseSSEResponse(t *testing.T, body string) map[string]interface{} {
	t.Helper()

	lines := bytes.Split([]byte(body), []byte("\n"))
	var dataLines []string

	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			data := string(bytes.TrimPrefix(line, []byte("data: ")))
			dataLines = append(dataLines, data)
		}
	}

	if len(dataLines) == 0 {
		t.Fatalf("No data lines found in SSE-formatted response: %s", body)
	}

	// Join all data lines (in case the response is multi-line)
	jsonData := bytes.TrimSpace([]byte(dataLines[0]))

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to decode SSE-formatted data: %v, data: %s", err, string(jsonData))
	}

	return result
}

// TestBinaryInvocation_LogFileCreation tests that the mcp-gateway.log file is created when the binary runs
func TestBinaryInvocation_LogFileCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create a temporary directory for logs
	tmpLogDir, err := os.MkdirTemp("", "awmg-log-test-*")
	require.NoError(t, err, "Failed to create temp log directory")
	defer os.RemoveAll(tmpLogDir)

	// Create a temporary config file
	configFile := createTempConfig(t, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	// Start the server process with custom log directory
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := "13006"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--log-dir", tmpLogDir,
		"--routed",
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

	// Wait for server to start
	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 5*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started successfully")

	// Check that the log file was created
	logFilePath := filepath.Join(tmpLogDir, "mcp-gateway.log")
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Fatalf("Log file was not created at %s", logFilePath)
	}

	t.Logf("✓ Log file created at: %s", logFilePath)

	// Read the log file to verify it contains log entries
	logContent, err := os.ReadFile(logFilePath)
	require.NoError(t, err, "Failed to read log file")

	if len(logContent) == 0 {
		t.Error("Log file is empty")
	} else {
		t.Logf("✓ Log file contains %d bytes", len(logContent))
	}

	// Verify log file contains expected startup messages
	logStr := string(logContent)
	expectedMessages := []string{
		"startup",
		"Starting MCPG",
	}

	for _, msg := range expectedMessages {
		if !bytes.Contains(logContent, []byte(msg)) {
			t.Errorf("Log file does not contain expected message: %q", msg)
			t.Logf("Log content:\n%s", logStr)
		}
	}

	t.Log("✓ Log file contains expected startup messages")

	// Verify log file permissions (should be 0644)
	fileInfo, err := os.Stat(logFilePath)
	require.NoError(t, err, "Failed to stat log file")

	perms := fileInfo.Mode().Perm()
	expectedPerms := os.FileMode(0644)
	if perms != expectedPerms {
		t.Errorf("Log file has incorrect permissions: got %o, expected %o", perms, expectedPerms)
	} else {
		t.Logf("✓ Log file has correct permissions: %o", perms)
	}

	// Test that the log file is readable by other processes
	// Try to open and read the file while the server is still running
	readFile, err := os.Open(logFilePath)
	require.NoError(t, err, "Failed to open log file for reading (concurrent access)")
	defer readFile.Close()

	// Read content
	concurrentRead, err := io.ReadAll(readFile)
	require.NoError(t, err, "Failed to read log file concurrently")

	if len(concurrentRead) == 0 {
		t.Error("Concurrent read returned empty content")
	} else {
		t.Logf("✓ Log file is readable by other processes (%d bytes read concurrently)", len(concurrentRead))
	}

	// Make an API call to generate more log entries
	resp, err := http.Get(serverURL + "/health")
	if err == nil {
		resp.Body.Close()
	}

	// Wait a moment for logs to be written
	time.Sleep(200 * time.Millisecond)

	// Read the log file again to verify new entries were added (or at least it's still readable)
	newLogContent, err := os.ReadFile(logFilePath)
	require.NoError(t, err, "Failed to read log file after API call")

	// Log file should either grow or stay the same (health endpoint may not always log)
	if len(newLogContent) >= len(logContent) {
		if len(newLogContent) > len(logContent) {
			t.Logf("✓ Log file grew from %d to %d bytes after API call (immediate flush working)", len(logContent), len(newLogContent))
		} else {
			t.Logf("✓ Log file size unchanged (%d bytes) but still readable after API call", len(logContent))
		}
	} else {
		t.Errorf("Log file shrank from %d to %d bytes - unexpected behavior", len(logContent), len(newLogContent))
	}

	t.Log("✓ All log file checks passed")
}

// TestBinaryInvocation_LogDirEnvironmentVariable tests that MCP_GATEWAY_LOG_DIR environment variable works
func TestBinaryInvocation_LogDirEnvironmentVariable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary integration test in short mode")
	}

	// Find the binary
	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Create a temporary directory for logs
	tmpLogDir := t.TempDir()
	t.Logf("Using temporary log directory: %s", tmpLogDir)

	// Create a temporary config file
	configFile := createTempConfig(t, map[string]interface{}{
		"testserver": map[string]interface{}{
			"command": "echo",
			"args":    []string{},
		},
	})
	defer os.Remove(configFile)

	// Start the server process with MCP_GATEWAY_LOG_DIR environment variable (no --log-dir flag)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := "13007"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config", configFile,
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	// Set the MCP_GATEWAY_LOG_DIR environment variable
	cmd.Env = append(os.Environ(), "MCP_GATEWAY_LOG_DIR="+tmpLogDir)

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

	// Wait for server to start
	serverURL := "http://127.0.0.1:" + port
	if !waitForServer(t, serverURL+"/health", 5*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Server did not start in time")
	}

	t.Log("✓ Server started successfully with MCP_GATEWAY_LOG_DIR environment variable")

	// Check that the log file was created in the directory specified by the environment variable
	logFilePath := filepath.Join(tmpLogDir, "mcp-gateway.log")
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Fatalf("Log file was not created at %s (from MCP_GATEWAY_LOG_DIR)", logFilePath)
	}

	t.Logf("✓ Log file created at: %s (from MCP_GATEWAY_LOG_DIR)", logFilePath)

	// Read the log file to verify it contains log entries
	logContent, err := os.ReadFile(logFilePath)
	require.NoError(t, err, "Failed to read log file")

	if len(logContent) == 0 {
		t.Error("Log file is empty")
	} else {
		t.Logf("✓ Log file contains %d bytes", len(logContent))
	}

	// Verify log file contains expected startup messages
	expectedMessages := []string{
		"startup",
		"Starting MCPG",
	}

	for _, msg := range expectedMessages {
		if !bytes.Contains(logContent, []byte(msg)) {
			t.Errorf("Log file does not contain expected message: %q", msg)
			t.Logf("Log content:\n%s", string(logContent))
		}
	}

	t.Log("✓ MCP_GATEWAY_LOG_DIR environment variable test passed")
}
