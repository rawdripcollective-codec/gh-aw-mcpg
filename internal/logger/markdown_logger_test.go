package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitMarkdownLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")
	defer CloseMarkdownLogger()

	// Check that the log directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created: %s", logDir)
	}

	// Check that the log file was created
	logPath := filepath.Join(logDir, fileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logPath)
	}
}

func TestMarkdownLoggerFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "format-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")

	// Write messages at different levels
	LogInfoMd("test", "This is an info message")
	LogWarnMd("test", "This is a warning message")
	LogErrorMd("test", "This is an error message")
	LogDebugMd("test", "This is a debug message")

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Check for HTML details wrapper
	assert.True(t, strings.Contains(logContent, "<details>"), "Log file does not contain opening <details> tag")
	assert.True(t, strings.Contains(logContent, "<summary>MCP Gateway</summary>"), "Log file does not contain summary tag")
	assert.True(t, strings.Contains(logContent, "</details>"), "Log file does not contain closing </details> tag")

	// Check for emoji bullet points
	expectedEmojis := []struct {
		emoji   string
		message string
	}{
		{"✓", "This is an info message"},
		{"⚠️", "This is a warning message"},
		{"✗", "This is an error message"},
		{"🔍", "This is a debug message"},
	}

	for _, expected := range expectedEmojis {
		if !strings.Contains(logContent, expected.emoji) {
			t.Errorf("Log file does not contain emoji: %s", expected.emoji)
		}
		if !strings.Contains(logContent, expected.message) {
			t.Errorf("Log file does not contain message: %s", expected.message)
		}
	}

	// Check for markdown bullet points
	assert.True(t, strings.Contains(logContent, "- ✓"), "Log file does not contain markdown bullet points")
}

func TestMarkdownLoggerSecretSanitization(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "secret-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")

	// Test various secret patterns
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"token=ghp_1234567890123456789012345678901234567890",
			"[REDACTED]",
		},
		{
			"API_KEY=sk_test_abcdefghijklmnopqrstuvwxyz123456",
			"[REDACTED]",
		},
		{
			"password: supersecretpassword123",
			"[REDACTED]",
		},
		{
			"Normal log message without secrets",
			"Normal log message without secrets",
		},
		{
			"Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			"[REDACTED]",
		},
	}

	for _, tc := range testCases {
		LogInfoMd("test", "%s", tc.input)
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Verify secrets are redacted
	secretStrings := []string{
		"ghp_1234567890123456789012345678901234567890",
		"sk_test_abcdefghijklmnopqrstuvwxyz123456",
		"supersecretpassword123",
		"Bearer abcdefghijklmnopqrstuvwxyz",
	}

	for _, secret := range secretStrings {
		if strings.Contains(logContent, secret) {
			t.Errorf("Log file contains secret that should be redacted: %s", secret)
		}
	}

	// Verify redaction marker is present
	assert.True(t, strings.Contains(logContent, "[REDACTED]"), "Log file does not contain [REDACTED] marker")

	// Verify normal message is not redacted
	assert.True(t, strings.Contains(logContent, "Normal log message without secrets"), "Log file does not contain non-secret message")
}

func TestMarkdownLoggerCategories(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "category-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")

	// Log messages with different categories
	categories := []string{"startup", "client", "backend", "shutdown"}
	for _, category := range categories {
		LogInfoMd(category, "Message for category %s", category)
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Verify all categories are present
	for _, category := range categories {
		if !strings.Contains(logContent, category) {
			t.Errorf("Log file does not contain category: %s", category)
		}
	}
}

func TestMarkdownLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "concurrent-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")
	defer CloseMarkdownLogger()

	// Write from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				LogInfoMd("concurrent", "Message from goroutine %d, iteration %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Count the number of log lines (each starts with "- ✓")
	lines := strings.Count(logContent, "- ✓")
	// Should have 100 lines (10 goroutines * 10 messages each)
	expectedLines := 100
	assert.Equal(t, expectedLines, lines, "%d log lines, got %d")
}

func TestMarkdownLoggerCodeBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "codeblock-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")

	// Log messages with technical content that should use code blocks
	LogInfoMd("test", "command=/usr/bin/docker args=[run --rm -i container]")
	LogInfoMd("test", "Multi-line\ncontent\nhere")
	LogInfoMd("test", "Simple message")

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Check for code blocks for technical content
	codeBlockCount := strings.Count(logContent, "```")
	assert.False(t, codeBlockCount < 2, "Expected at least 2 code block markers (opening and closing), got %d")
}

func TestMarkdownLoggerFallback(t *testing.T) {
	// Use a non-writable directory
	logDir := "/root/nonexistent/directory"
	fileName := "test.md"

	// Initialize the logger - should not fail, but use fallback
	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger should not fail on fallback")
	defer CloseMarkdownLogger()

	globalMarkdownMu.RLock()
	useFallback := globalMarkdownLogger.useFallback
	globalMarkdownMu.RUnlock()

	if !useFallback {
		t.Logf("Logger initialized without fallback (may have permissions)")
	}

	// Should not panic when logging
	LogInfoMd("test", "This should not crash")
}

func TestMarkdownLoggerRPCFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "rpc-format-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	require.NoError(t, err, "InitMarkdownLogger failed")

	// Test RPC-style pre-formatted messages (should NOT be wrapped in code blocks)
	LogDebugMd("rpc", "**github**→`tools/list`")
	LogDebugMd("rpc", "**safeoutputs**→`tools/call`\n\n```json\n{\n  \"id\": 1\n}\n```")
	LogDebugMd("rpc", "**backend**←`resp`")

	// Test regular messages (should use code blocks for multi-line)
	LogInfoMd("backend", "command=/usr/bin/docker args=[run]")
	LogInfoMd("backend", "Multi\nline\ncontent")

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// RPC messages should be on single lines (not wrapped in code blocks)
	// Check that "- 🔍 rpc **github**→`tools/list`" is on one line
	assert.True(t, strings.Contains(logContent, "- 🔍 rpc **github**→`tools/list`"), "RPC message should be on single line without extra code block wrapping")

	// Check that RPC messages with JSON blocks are properly formatted
	// The title should be on one line, followed by the JSON block INDENTED with 2 spaces
	assert.True(t, strings.Contains(logContent, "- 🔍 rpc **safeoutputs**→`tools/call`"), "RPC message with JSON should have title on single line")

	// Verify that JSON code block lines are properly indented under the bullet point
	// The empty line after the first line should be indented
	assert.True(t, strings.Contains(logContent, "- 🔍 rpc **safeoutputs**→`tools/call`\n  \n  ```json"), "JSON code block should be indented with 2 spaces")

	// Verify the closing code fence is also indented
	assert.True(t, strings.Contains(logContent, "  ```"), "Closing code fence should be indented")

	// Regular multi-line messages should still use code blocks
	assert.True(t, strings.Contains(logContent, "- ✓ **backend**\n  ```\n  command="), "Regular multi-line messages should still use code blocks")

	// Count occurrences of nested code blocks (should not happen)
	// A nested code block would look like: ``` \n  **server** \n ```
	nestedPattern := "```\n  **"
	if strings.Contains(logContent, nestedPattern) {
		t.Errorf("Found nested code blocks - RPC messages should not be double-wrapped")
	}
}

func TestMarkdownLoggerMultiLineErrors(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "error-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}

	// Test multi-line error message (like JSON schema validation errors)
	errorMsg := `Configuration validation error (MCP Gateway version: dev):

Location: <root>
Error: doesn't validate with schema
  Location: <root>
  Error: missing properties: 'gateway'
  Details: Required field(s) are missing
    → Add the required field(s) to your configuration
  Schema location: /required

Please check your configuration`

	LogErrorMd("startup", "Configuration validation failed:\n%s", errorMsg)

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check for error emoji
	if !strings.Contains(logContent, "✗") {
		t.Errorf("Log file does not contain error emoji")
	}

	// Check for startup category
	if !strings.Contains(logContent, "startup") {
		t.Errorf("Log file does not contain startup category")
	}

	// Check for code block formatting (multi-line errors should use code blocks)
	if !strings.Contains(logContent, "```") {
		t.Errorf("Log file does not contain code block markers for multi-line error")
	}

	// Check that the error message content is present
	if !strings.Contains(logContent, "Configuration validation error") {
		t.Errorf("Log file does not contain error message content")
	}

	if !strings.Contains(logContent, "missing properties") {
		t.Errorf("Log file does not contain error details")
	}
}
