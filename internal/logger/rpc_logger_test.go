package logger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatRPCMessage(t *testing.T) {
	tests := []struct {
		name string
		info *RPCMessageInfo
		want []string // Strings that should be present in output
	}{
		{
			name: "outbound request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/list"}`,
			},
			want: []string{"github→tools/list", "50b", `{"jsonrpc":"2.0","method":"tools/list"}`},
		},
		{
			name: "inbound response with error",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Payload:     `{"jsonrpc":"2.0","error":{"code":-32600}}`,
				Error:       "Invalid request",
			},
			want: []string{"github←resp", "100b", "err:Invalid request"},
		},
		{
			name: "client request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageRequest,
				ServerID:    "client",
				Method:      "tools/call",
				PayloadSize: 200,
				Payload:     `{"method":"tools/call","params":{}}`,
			},
			want: []string{"client←tools/call", "200b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRPCMessage(tt.info)

			for _, expected := range tt.want {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestFormatRPCMessageMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		info    *RPCMessageInfo
		want    []string // Strings that should be present in output
		notWant []string // Strings that should NOT be present in output
	}{
		{
			name: "outbound request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/list","params":{}}`,
			},
			want:    []string{"**github**→`tools/list`", "```json", `"params"`, "{}"},
			notWant: []string{`"jsonrpc"`, `"method"`},
		},
		{
			name: "inbound response",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Payload:     `{"result":{}}`,
			},
			want:    []string{"**github**←`resp`", "```json", `"result"`},
			notWant: []string{`"jsonrpc"`, `"method"`},
		},
		{
			name: "response with error",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Error:       "Connection timeout",
			},
			want:    []string{"**github**←`resp`", "⚠️`Connection timeout`"},
			notWant: []string{},
		},
		{
			name: "invalid JSON payload uses inline backticks",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/call",
				PayloadSize: 30,
				Payload:     `{invalid json syntax}`,
			},
			want:    []string{"**github**→`tools/call`", "`{invalid json syntax}`"},
			notWant: []string{"```json"}, // Should NOT use code blocks for invalid JSON
		},
		{
			name: "request with only params null after field removal",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/list","params":null}`,
			},
			want:    []string{"**github**→`tools/list`"},
			notWant: []string{"```json", `"params"`}, // Should NOT show JSON block when only params: null
		},
		{
			name: "request with empty object after field removal",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/list"}`,
			},
			want:    []string{"**github**→`tools/list`"},
			notWant: []string{"```json"}, // Should NOT show JSON block when empty
		},
		{
			name: "tools/call with tool name",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/call",
				PayloadSize: 100,
				Payload:     `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"search_code","arguments":{"query":"test"}}}`,
			},
			want:    []string{"**github**→`tools/call` `search_code`", "```json", `"arguments"`},
			notWant: []string{`"jsonrpc"`, `"method"`},
		},
		{
			name: "tools/call without tool name in params",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/call",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/call","params":{}}`,
			},
			want:    []string{"**github**→`tools/call`", "```json", `"params"`},
			notWant: []string{`"jsonrpc"`, `"method"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRPCMessageMarkdown(tt.info)

			for _, expected := range tt.want {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notWant {
				assert.False(t, strings.Contains(result, notExpected), "Expected result NOT to contain %q, got:\n%s")
			}
		})
	}
}

func TestFormatJSONWithoutFields(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		fieldsToRemove []string
		wantContains   []string
		wantNotContain []string
		wantValid      bool
		wantEmpty      bool
	}{
		{
			name:           "remove jsonrpc and method",
			input:          `{"jsonrpc":"2.0","method":"tools/call","params":{"arg":"value"},"id":1}`,
			fieldsToRemove: []string{"jsonrpc", "method"},
			wantContains:   []string{`"params"`, `"arg"`, `"value"`, `"id"`},
			wantNotContain: []string{`"jsonrpc"`, `"method"`},
			wantValid:      true,
			wantEmpty:      false,
		},
		{
			name:           "compact single line format",
			input:          `{"a":"b","c":{"d":"e"}}`,
			fieldsToRemove: []string{},
			wantContains:   []string{`"a":"b"`, `"c":`, `"d":"e"`},
			wantNotContain: []string{"\n", "  "},
			wantValid:      true,
			wantEmpty:      false,
		},
		{
			name:           "invalid JSON returns as-is with false",
			input:          `{invalid json}`,
			fieldsToRemove: []string{"jsonrpc"},
			wantContains:   []string{`{invalid json}`},
			wantNotContain: []string{},
			wantValid:      false,
			wantEmpty:      false,
		},
		{
			name:           "empty object",
			input:          `{}`,
			fieldsToRemove: []string{"jsonrpc"},
			wantContains:   []string{`{}`},
			wantNotContain: []string{},
			wantValid:      true,
			wantEmpty:      true,
		},
		{
			name:           "only params null after removal",
			input:          `{"jsonrpc":"2.0","method":"tools/list","params":null}`,
			fieldsToRemove: []string{"jsonrpc", "method"},
			wantContains:   []string{`"params"`, `null`},
			wantNotContain: []string{},
			wantValid:      true,
			wantEmpty:      true,
		},
		{
			name:           "params with value is not empty",
			input:          `{"jsonrpc":"2.0","method":"tools/list","params":{"key":"value"}}`,
			fieldsToRemove: []string{"jsonrpc", "method"},
			wantContains:   []string{`"params"`},
			wantNotContain: []string{},
			wantValid:      true,
			wantEmpty:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, isValid, isEmpty := formatJSONWithoutFields(tt.input, tt.fieldsToRemove)

			assert.Equal(t, tt.wantValid, isValid, "isValid=%v, got %v")

			assert.Equal(t, tt.wantEmpty, isEmpty, "isEmpty=%v, got %v")

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Expected result to contain %q, got:\n%s", want, result)
				}
			}

			for _, notWant := range tt.wantNotContain {
				assert.False(t, strings.Contains(result, notWant), "Expected result NOT to contain %q, got:\n%s")
			}
		})
	}
}

func TestLogRPCRequest(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Log an RPC request
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	LogRPCRequest(RPCDirectionOutbound, "github", "tools/list", payload)

	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()

	// Check text log
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	require.NoError(t, err, "Failed to read text log")

	textStr := string(textContent)
	expectedInText := []string{"github→tools/list", "58b"}
	for _, expected := range expectedInText {
		if !strings.Contains(textStr, expected) {
			t.Errorf("Text log does not contain %q", expected)
		}
	}

	// Check markdown log
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	require.NoError(t, err, "Failed to read markdown log")

	mdStr := string(mdContent)
	expectedInMd := []string{"**github**→`tools/list`"}
	for _, expected := range expectedInMd {
		if !strings.Contains(mdStr, expected) {
			t.Errorf("Markdown log does not contain %q", expected)
		}
	}
}

func TestLogRPCResponse(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Log an RPC response with error
	payload := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`)
	err := errors.New("backend connection failed")
	LogRPCResponse(RPCDirectionInbound, "github", payload, err)

	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()

	// Check text log
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	require.NoError(t, err, "Failed to read text log")

	textStr := string(textContent)
	expectedInText := []string{"github←resp", "err:backend connection failed"}
	for _, expected := range expectedInText {
		if !strings.Contains(textStr, expected) {
			t.Errorf("Text log does not contain %q", expected)
		}
	}

	// Check markdown log
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	require.NoError(t, err, "Failed to read markdown log")

	mdStr := string(mdContent)
	expectedInMd := []string{"**github**←`resp`", "⚠️`backend connection failed`"}
	for _, expected := range expectedInMd {
		if !strings.Contains(mdStr, expected) {
			t.Errorf("Markdown log does not contain %q", expected)
		}
	}
}

func TestLogRPCRequestWithSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Log an RPC request with a secret
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"authenticate","params":{"token":"ghp_1234567890123456789012345678901234567890"}}`)
	LogRPCRequest(RPCDirectionInbound, "client", "authenticate", payload)

	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()

	// Check text log - should NOT contain the actual token
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	require.NoError(t, err, "Failed to read text log")

	textStr := string(textContent)
	if strings.Contains(textStr, "ghp_1234567890123456789012345678901234567890") {
		t.Errorf("Text log contains secret that should be redacted")
	}
	assert.True(t, strings.Contains(textStr, "[REDACTED]"), "Text log does not contain [REDACTED] marker")

	// Check markdown log - should NOT contain the actual token
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	require.NoError(t, err, "Failed to read markdown log")

	mdStr := string(mdContent)
	if strings.Contains(mdStr, "ghp_1234567890123456789012345678901234567890") {
		t.Errorf("Markdown log contains secret that should be redacted")
	}
	assert.True(t, strings.Contains(mdStr, "[REDACTED]"), "Markdown log does not contain [REDACTED] marker")
}

func TestLogRPCRequestPayloadTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Create a large payload (> 10KB for text, > 512 chars for markdown)
	largeData := strings.Repeat("x", 12*1024) // 12KB of x's
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{"data":"` + largeData + `"}}`)
	LogRPCRequest(RPCDirectionOutbound, "backend", "test", payload)

	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()

	// Check text log - payload should be truncated at 10KB
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	require.NoError(t, err, "Failed to read text log")

	textStr := string(textContent)
	assert.True(t, strings.Contains(textStr, "..."), "Text log does not show truncation marker")

	// The logged payload should not contain the full 12KB of x's
	// (it should be truncated to 10KB + "...")
	xCount := strings.Count(textStr, strings.Repeat("x", 11*1024))
	if xCount > 0 {
		t.Errorf("Text log contains more data than expected after truncation (should be ~10KB)")
	}

	// Check markdown log - should be truncated at 512 chars
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	require.NoError(t, err, "Failed to read markdown log")

	mdStr := string(mdContent)
	assert.True(t, strings.Contains(mdStr, "..."), "Markdown log does not show truncation marker")

	// Markdown should have much less data (truncated at 512 chars)
	xCountMd := strings.Count(mdStr, strings.Repeat("x", 600))
	if xCountMd > 0 {
		t.Errorf("Markdown log contains more data than expected after truncation (should be ~512 chars)")
	}
}
