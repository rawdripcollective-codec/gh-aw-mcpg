package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitServerFileLogger(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "server-logs")

	// Initialize the server logger
	err := InitServerFileLogger(logDir)
	require.NoError(t, err, "InitServerFileLogger failed")
	defer CloseServerFileLogger()

	// Check that the log directory was created
	_, err = os.Stat(logDir)
	assert.NoError(t, err, "Log directory was not created: %s", logDir)
}

func TestServerFileLoggerCreatesLogFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "server-logs")

	// Initialize the server logger
	err := InitServerFileLogger(logDir)
	require.NoError(t, err)
	defer CloseServerFileLogger()

	// Log messages for different servers
	LogInfoWithServer("github", "test", "Test message 1")
	LogInfoWithServer("slack", "test", "Test message 2")
	LogWarnWithServer("github", "test", "Warning message")

	// Close to flush all files
	err = CloseServerFileLogger()
	require.NoError(t, err)

	// Check that log files were created for each server
	githubLog := filepath.Join(logDir, "github.log")
	slackLog := filepath.Join(logDir, "slack.log")

	_, err = os.Stat(githubLog)
	assert.NoError(t, err, "github.log was not created")

	_, err = os.Stat(slackLog)
	assert.NoError(t, err, "slack.log was not created")

	// Read and verify log contents
	githubContent, err := os.ReadFile(githubLog)
	require.NoError(t, err)
	assert.Contains(t, string(githubContent), "Test message 1")
	assert.Contains(t, string(githubContent), "Warning message")
	assert.Contains(t, string(githubContent), "[INFO]")
	assert.Contains(t, string(githubContent), "[WARN]")

	slackContent, err := os.ReadFile(slackLog)
	require.NoError(t, err)
	assert.Contains(t, string(slackContent), "Test message 2")
	assert.NotContains(t, string(slackContent), "Test message 1")
}

func TestServerFileLoggerConcurrentAccess(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "server-logs")

	// Initialize the server logger
	err := InitServerFileLogger(logDir)
	require.NoError(t, err)
	defer CloseServerFileLogger()

	// Concurrently log messages from multiple goroutines
	var wg sync.WaitGroup
	serverIDs := []string{"server1", "server2", "server3"}
	messagesPerServer := 50

	for _, serverID := range serverIDs {
		for i := 0; i < messagesPerServer; i++ {
			wg.Add(1)
			go func(sid string, index int) {
				defer wg.Done()
				LogInfoWithServer(sid, "test", "Message %d", index)
			}(serverID, i)
		}
	}

	wg.Wait()

	// Close to flush all files
	err = CloseServerFileLogger()
	require.NoError(t, err)

	// Verify that each server has the expected number of log entries
	for _, serverID := range serverIDs {
		logFile := filepath.Join(logDir, serverID+".log")
		content, err := os.ReadFile(logFile)
		require.NoError(t, err, "Failed to read log file for %s", serverID)

		// Count the number of lines
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		assert.Equal(t, messagesPerServer, len(lines),
			"Server %s should have %d log entries, got %d", serverID, messagesPerServer, len(lines))
	}
}

func TestServerFileLoggerFallback(t *testing.T) {
	// Use a non-writable directory to trigger fallback
	logDir := "/root/nonexistent/directory"

	// Initialize the logger - should not fail, but use fallback
	err := InitServerFileLogger(logDir)
	require.NoError(t, err, "InitServerFileLogger should not fail on fallback")
	defer CloseServerFileLogger()

	globalServerLoggerMu.RLock()
	useFallback := globalServerFileLogger.useFallback
	globalServerLoggerMu.RUnlock()

	assert.True(t, useFallback, "Logger should be in fallback mode")

	// Log should not panic in fallback mode
	assert.NotPanics(t, func() {
		LogInfoWithServer("github", "test", "Test message in fallback mode")
	})
}

func TestServerFileLoggerAllLevels(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "server-logs")

	// Initialize the server logger
	err := InitServerFileLogger(logDir)
	require.NoError(t, err)
	defer CloseServerFileLogger()

	serverID := "test-server"

	// Log messages at all levels
	LogInfoWithServer(serverID, "test", "Info message")
	LogWarnWithServer(serverID, "test", "Warning message")
	LogErrorWithServer(serverID, "test", "Error message")
	LogDebugWithServer(serverID, "test", "Debug message")

	// Close to flush
	err = CloseServerFileLogger()
	require.NoError(t, err)

	// Read log file
	logFile := filepath.Join(logDir, serverID+".log")
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify all log levels are present
	assert.Contains(t, contentStr, "[INFO]")
	assert.Contains(t, contentStr, "[WARN]")
	assert.Contains(t, contentStr, "[ERROR]")
	assert.Contains(t, contentStr, "[DEBUG]")

	// Verify messages are present
	assert.Contains(t, contentStr, "Info message")
	assert.Contains(t, contentStr, "Warning message")
	assert.Contains(t, contentStr, "Error message")
	assert.Contains(t, contentStr, "Debug message")
}

func TestServerFileLoggerMultipleInit(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "server-logs")

	// Initialize the server logger
	err := InitServerFileLogger(logDir)
	require.NoError(t, err)

	// Log a message
	LogInfoWithServer("server1", "test", "Message 1")

	// Re-initialize (should close old logger and create new one)
	err = InitServerFileLogger(logDir)
	require.NoError(t, err)

	// Log another message
	LogInfoWithServer("server1", "test", "Message 2")

	// Close
	err = CloseServerFileLogger()
	require.NoError(t, err)

	// Verify both messages are in the file
	logFile := filepath.Join(logDir, "server1.log")
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	assert.Contains(t, string(content), "Message 1")
	assert.Contains(t, string(content), "Message 2")
}

func TestServerFileLoggerPreservesUnifiedView(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Initialize both the unified file logger and the server file logger
	err := InitFileLogger(logDir, "mcp-gateway.log")
	require.NoError(t, err, "InitFileLogger failed")
	defer CloseGlobalLogger()

	err = InitServerFileLogger(logDir)
	require.NoError(t, err, "InitServerFileLogger failed")
	defer CloseServerFileLogger()

	// Log messages using per-serverID logging
	LogInfoWithServer("github", "backend", "GitHub server started")
	LogWarnWithServer("slack", "backend", "Slack connection timeout")
	LogErrorWithServer("github", "backend", "GitHub authentication failed")

	// Close loggers to flush
	err = CloseServerFileLogger()
	require.NoError(t, err)
	err = CloseGlobalLogger()
	require.NoError(t, err)

	// Verify per-serverID log files exist and contain correct messages
	githubLog := filepath.Join(logDir, "github.log")
	githubContent, err := os.ReadFile(githubLog)
	require.NoError(t, err, "github.log should exist")
	assert.Contains(t, string(githubContent), "GitHub server started")
	assert.Contains(t, string(githubContent), "GitHub authentication failed")
	assert.NotContains(t, string(githubContent), "Slack connection timeout", "github.log should not contain Slack messages")

	slackLog := filepath.Join(logDir, "slack.log")
	slackContent, err := os.ReadFile(slackLog)
	require.NoError(t, err, "slack.log should exist")
	assert.Contains(t, string(slackContent), "Slack connection timeout")
	assert.NotContains(t, string(slackContent), "GitHub", "slack.log should not contain GitHub messages")

	// CRITICAL: Verify unified log file contains ALL messages from all servers
	unifiedLog := filepath.Join(logDir, "mcp-gateway.log")
	unifiedContent, err := os.ReadFile(unifiedLog)
	require.NoError(t, err, "mcp-gateway.log should exist")

	// All messages should be in the unified log with serverID prefix
	assert.Contains(t, string(unifiedContent), "[github]", "unified log should have github prefix")
	assert.Contains(t, string(unifiedContent), "GitHub server started", "unified log should contain GitHub message")
	assert.Contains(t, string(unifiedContent), "[slack]", "unified log should have slack prefix")
	assert.Contains(t, string(unifiedContent), "Slack connection timeout", "unified log should contain Slack message")
	assert.Contains(t, string(unifiedContent), "GitHub authentication failed", "unified log should contain GitHub error")

	// Verify unified log has all three messages
	lines := strings.Split(strings.TrimSpace(string(unifiedContent)), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "unified log should have at least 3 messages")
}

