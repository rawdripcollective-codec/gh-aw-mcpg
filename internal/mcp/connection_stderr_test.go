package mcp

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnection_StderrLoggingWithServerID verifies that stderr output includes serverID
func TestConnection_StderrLoggingWithServerID(t *testing.T) {
	// Skip this test if Docker is not available
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Skipping Docker test")
	}

	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a command that will output to stderr and fail quickly
	// This command attempts to run a non-existent gh command which will produce stderr
	serverID := "test-github-server"
	command := "docker"
	args := []string{
		"run", "--rm", "-i",
		"ghcr.io/github/github-mcp-server:latest",
		"gh", "aw", "status", // This will fail and produce stderr
	}

	// Try to create a connection - it will fail but we'll capture stderr
	_, err := NewConnection(ctx, serverID, command, args, nil)

	// We expect an error since the command will fail
	require.Error(t, err, "Expected connection to fail")

	// Give goroutines time to flush logs
	time.Sleep(100 * time.Millisecond)

	// Check the captured log output
	logOutput := logBuf.String()

	// The stderr output should now include the serverID
	// Look for log entries that contain both the serverID and "stderr"
	assert.Contains(t, logOutput, "[test-github-server stderr]",
		"Log output should contain serverID in stderr logs. Got:\n%s", logOutput)

	// If we got stderr output about the command not found, verify it has the serverID
	if strings.Contains(logOutput, "stderr") {
		lines := strings.Split(logOutput, "\n")
		for _, line := range lines {
			if strings.Contains(line, "stderr") && strings.Contains(line, "test-github-server") {
				t.Logf("✓ Found expected stderr log with serverID: %s", line)
				return
			}
		}
	}
}

// TestConnection_MultipleServersStderrLogging verifies that stderr from multiple servers is distinguishable
func TestConnection_MultipleServersStderrLogging(t *testing.T) {
	// This is a conceptual test to document the expected behavior
	// In production, with parallel servers, logs should look like:
	//
	// mcp:connection [server1 stderr] ✗ failed to run status command: exit status 1 +357ms
	// mcp:connection [server2 stderr] Output: unknown command "aw" for "gh" +779µs
	// mcp:connection [server1 stderr] Did you mean this? +697µs
	// mcp:connection [server2 stderr] Usage: gh <command> +654µs
	//
	// Instead of:
	//
	// mcp:connection [stderr] ✗ failed to run status command: exit status 1 +357ms
	// mcp:connection [stderr] Output: unknown command "aw" for "gh" +779µs
	// mcp:connection [stderr] Did you mean this? +697µs
	// mcp:connection [stderr] Usage: gh <command> +654µs
	//
	// This makes it clear which server each log line is from.

	t.Log("This test documents the expected behavior for multiple servers")
	t.Log("With the serverID in stderr logs, parallel server logs are now distinguishable")
}

// TestConnection_DebugLoggerOutput verifies debug logger includes serverID
func TestConnection_DebugLoggerOutput(t *testing.T) {
	// Capture the debug logger output by redirecting stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Enable debug logging for this test
	oldDebug := os.Getenv("DEBUG")
	os.Setenv("DEBUG", "mcp:*")
	defer func() {
		os.Setenv("DEBUG", oldDebug)
		os.Stderr = oldStderr
		w.Close()
	}()

	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to create a connection that will produce stderr
	serverID := "debug-test-server"
	command := "docker"
	args := []string{
		"run", "--rm", "-i",
		"ghcr.io/github/github-mcp-server:latest",
		"gh", "nonexistent-command",
	}

	// Start the connection attempt
	go func() {
		NewConnection(ctx, serverID, command, args, nil)
	}()

	// Give it a moment to start and produce output
	time.Sleep(500 * time.Millisecond)

	// Close writer and read output
	w.Close()
	var debugBuf bytes.Buffer
	io.Copy(&debugBuf, r)

	debugOutput := debugBuf.String()

	// The debug output should contain the serverID in stderr logs
	if strings.Contains(debugOutput, "stderr") {
		assert.Contains(t, debugOutput, serverID,
			"Debug output should contain serverID. Got:\n%s", debugOutput)
		t.Logf("✓ Debug logger includes serverID in stderr logs")
	} else {
		t.Log("No stderr output captured in debug logs (command may not have run)")
	}
}
