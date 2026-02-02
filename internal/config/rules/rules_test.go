package rules

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortRange(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid port 8080",
			port:      8080,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "valid port 1",
			port:      1,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "valid port 65535",
			port:      65535,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "invalid port 0",
			port:      0,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid port 65536",
			port:      65536,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid negative port",
			port:      -1,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid port 100000",
			port:      100000,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PortRange(tt.port, tt.jsonPath)

			if tt.shouldErr {
				require.NotNil(t, err, "Expected error but got none")
				assert.Contains(t, err.Message, tt.errMsg, "Error message should contain expected text")
				assert.Equal(t, tt.jsonPath, err.JSONPath, "JSONPath should match")
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestTimeoutPositive(t *testing.T) {
	tests := []struct {
		name      string
		timeout   int
		fieldName string
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid timeout 30",
			timeout:   30,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: false,
		},
		{
			name:      "valid timeout 1",
			timeout:   1,
			fieldName: "toolTimeout",
			jsonPath:  "gateway.toolTimeout",
			shouldErr: false,
		},
		{
			name:      "valid large timeout",
			timeout:   3600,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: false,
		},
		{
			name:      "invalid timeout 0",
			timeout:   0,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: true,
			errMsg:    "startupTimeout must be at least 1",
		},
		{
			name:      "invalid negative timeout",
			timeout:   -10,
			fieldName: "toolTimeout",
			jsonPath:  "gateway.toolTimeout",
			shouldErr: true,
			errMsg:    "toolTimeout must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TimeoutPositive(tt.timeout, tt.fieldName, tt.jsonPath)

			if tt.shouldErr {
				require.NotNil(t, err, "Expected error but got none")
				assert.Contains(t, err.Message, tt.errMsg, "Error message should contain expected text")
				assert.Equal(t, tt.jsonPath, err.JSONPath, "JSONPath should match")
				assert.Equal(t, tt.fieldName, err.Field, "Field name should match")
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestMountFormat(t *testing.T) {
	tests := []struct {
		name      string
		mount     string
		jsonPath  string
		index     int
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid ro mount",
			mount:     "/host/path:/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid rw mount",
			mount:     "/var/data:/app/data:rw",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid mount with special chars",
			mount:     "/home/user/my-app:/app/data:ro",
			jsonPath:  "mcpServers.github",
			index:     1,
			shouldErr: false,
		},
		{
			name:      "valid mount without mode",
			mount:     "/host/path:/container/path",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "invalid format - too many colons",
			mount:     "/host/path:/container/path:ro:extra",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount format",
		},
		{
			name:      "invalid format - empty source",
			mount:     ":/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source cannot be empty",
		},
		{
			name:      "invalid format - empty dest",
			mount:     "/host/path::ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination cannot be empty",
		},
		{
			name:      "invalid mode",
			mount:     "/host/path:/container/path:invalid",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount mode",
		},
		{
			name:      "invalid mode - uppercase",
			mount:     "/host/path:/container/path:RO",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount mode",
		},
		{
			name:      "invalid source - relative path",
			mount:     "relative/path:/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - relative path",
			mount:     "/host/path:relative/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "invalid source - dot relative",
			mount:     "./config:/app/config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - dot relative",
			mount:     "/host/config:./config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "invalid source - parent relative",
			mount:     "../config:/app/config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - parent relative",
			mount:     "/host/config:../config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "valid mount - root paths",
			mount:     "/:/root:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid mount - deep nested paths",
			mount:     "/var/lib/docker/volumes/data:/app/data/volumes:rw",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MountFormat(tt.mount, tt.jsonPath, tt.index)

			if tt.shouldErr {
				require.NotNil(t, err, "Expected error but got none")
				assert.Contains(t, err.Message, tt.errMsg, "Error message should contain expected text")
				expectedPath := "mcpServers.github.mounts[0]"
				if tt.index != 0 {
					expectedPath = "mcpServers.github.mounts[1]"
				}
				assert.Equal(t, expectedPath, err.JSONPath, "JSONPath should match expected pattern")
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name       string
		valErr     *ValidationError
		wantSubstr []string
	}{
		{
			name: "error with suggestion",
			valErr: &ValidationError{
				Field:      "port",
				Message:    "port must be between 1 and 65535",
				JSONPath:   "gateway.port",
				Suggestion: "Use a valid port number",
			},
			wantSubstr: []string{
				"Configuration error at gateway.port",
				"port must be between 1 and 65535",
				"Suggestion: Use a valid port number",
			},
		},
		{
			name: "error without suggestion",
			valErr: &ValidationError{
				Field:    "timeout",
				Message:  "timeout must be positive",
				JSONPath: "gateway.startupTimeout",
			},
			wantSubstr: []string{
				"Configuration error at gateway.startupTimeout",
				"timeout must be positive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.valErr.Error()

			for _, substr := range tt.wantSubstr {
				assert.Contains(t, errStr, substr, "Error string should contain expected substring")
			}
		})
	}
}

func TestUnsupportedType(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		actualType string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "unsupported server type",
			fieldName:  "type",
			actualType: "grpc",
			jsonPath:   "mcpServers.github",
			suggestion: "Use 'stdio' for standard input/output transport or 'http' for HTTP transport",
			wantSubstr: []string{
				"type",
				"unsupported server type 'grpc'",
				"mcpServers.github.type",
				"Use 'stdio'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnsupportedType(tt.fieldName, tt.actualType, tt.jsonPath, tt.suggestion)

			assert.Equal(t, tt.fieldName, err.Field, "Field should match")
			assert.Contains(t, err.Message, tt.actualType, "Message should contain actual type")
			assert.Contains(t, err.JSONPath, tt.jsonPath, "JSONPath should contain json path")
			assert.Equal(t, tt.suggestion, err.Suggestion, "Suggestion should match")

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, errStr, substr, "Error string should contain expected substring")
			}
		})
	}
}

func TestUndefinedVariable(t *testing.T) {
	tests := []struct {
		name       string
		varName    string
		jsonPath   string
		wantSubstr []string
	}{
		{
			name:     "undefined env variable",
			varName:  "MY_VAR",
			jsonPath: "mcpServers.github.env.TOKEN",
			wantSubstr: []string{
				"undefined environment variable referenced: MY_VAR",
				"mcpServers.github.env.TOKEN",
				"Set the environment variable MY_VAR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UndefinedVariable(tt.varName, tt.jsonPath)

			assert.Equal(t, "env variable", err.Field, "Field should be 'env variable'")
			assert.Contains(t, err.Message, tt.varName, "Message should contain variable name")
			assert.Equal(t, tt.jsonPath, err.JSONPath, "JSONPath should match")
			assert.Contains(t, err.Suggestion, tt.varName, "Suggestion should contain variable name")

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, errStr, substr, "Error string should contain expected substring")
			}
		})
	}
}

func TestMissingRequired(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		serverType string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "missing container field",
			fieldName:  "container",
			serverType: "stdio",
			jsonPath:   "mcpServers.github",
			suggestion: "Add a 'container' field (e.g., \"ghcr.io/owner/image:tag\")",
			wantSubstr: []string{
				"container",
				"'container' is required",
				"stdio servers",
				"mcpServers.github",
			},
		},
		{
			name:       "missing url field",
			fieldName:  "url",
			serverType: "HTTP",
			jsonPath:   "mcpServers.httpServer",
			suggestion: "Add a 'url' field (e.g., \"https://example.com/mcp\")",
			wantSubstr: []string{
				"url",
				"'url' is required",
				"HTTP servers",
				"mcpServers.httpServer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MissingRequired(tt.fieldName, tt.serverType, tt.jsonPath, tt.suggestion)

			assert.Equal(t, tt.fieldName, err.Field, "Field should match")
			assert.Contains(t, err.Message, tt.fieldName, "Message should contain field name")
			assert.Contains(t, err.Message, tt.serverType, "Message should contain server type")
			assert.Equal(t, tt.jsonPath, err.JSONPath, "JSONPath should match")
			assert.Equal(t, tt.suggestion, err.Suggestion, "Suggestion should match")

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, errStr, substr, "Error string should contain expected substring")
			}
		})
	}
}

func TestUnsupportedField(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		message    string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "unsupported command field",
			fieldName:  "command",
			message:    "'command' field is not supported (stdio servers must use 'container')",
			jsonPath:   "mcpServers.github",
			suggestion: "Remove 'command' field and use 'container' instead",
			wantSubstr: []string{
				"command",
				"not supported",
				"mcpServers.github",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnsupportedField(tt.fieldName, tt.message, tt.jsonPath, tt.suggestion)

			assert.Equal(t, tt.fieldName, err.Field, "Field should match")
			assert.Equal(t, tt.message, err.Message, "Message should match")
			assert.Equal(t, tt.jsonPath, err.JSONPath, "JSONPath should match")
			assert.Equal(t, tt.suggestion, err.Suggestion, "Suggestion should match")

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, errStr, substr, "Error string should contain expected substring")
			}
		})
	}
}

func TestAppendConfigDocsFooter(t *testing.T) {
	var sb strings.Builder
	AppendConfigDocsFooter(&sb)

	result := sb.String()

	wantSubstr := []string{
		"Please check your configuration",
		ConfigSpecURL,
		"JSON Schema reference",
		SchemaURL,
	}

	for _, substr := range wantSubstr {
		assert.Contains(t, result, substr, "Footer should contain expected substring")
	}
}

func TestDocumentationURLConstants(t *testing.T) {
	assert.NotEmpty(t, ConfigSpecURL, "ConfigSpecURL should not be empty")
	assert.NotEmpty(t, SchemaURL, "SchemaURL should not be empty")
	assert.True(t, strings.HasPrefix(ConfigSpecURL, "https://"), "ConfigSpecURL should start with https://")
	assert.True(t, strings.HasPrefix(SchemaURL, "https://"), "SchemaURL should start with https://")
}

func TestNonEmptyString(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid non-empty string",
			value:     "/tmp/payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid single character",
			value:     "x",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "empty string",
			value:     "",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "payloadDir cannot be empty",
		},
		{
			name:      "empty string with different field",
			value:     "",
			fieldName: "apiKey",
			jsonPath:  "gateway.apiKey",
			shouldErr: true,
			errMsg:    "apiKey cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NonEmptyString(tt.value, tt.fieldName, tt.jsonPath)

			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Contains(t, err.Error(), tt.jsonPath)
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestAbsolutePath(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid Unix absolute path",
			value:     "/tmp/payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid Unix root path",
			value:     "/",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid Unix nested path",
			value:     "/var/lib/payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid Windows absolute path - C drive",
			value:     "C:\\payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid Windows absolute path - D drive",
			value:     "D:\\temp\\payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "valid Windows absolute path - lowercase drive",
			value:     "c:\\payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: false,
		},
		{
			name:      "invalid relative Unix path",
			value:     "tmp/payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "invalid relative path with dot",
			value:     "./payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "invalid relative path with double dot",
			value:     "../payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "invalid Windows relative path",
			value:     "payloads\\data",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "invalid Windows path without backslash",
			value:     "C:payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "invalid Windows path with forward slash",
			value:     "C:/payloads",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "must be an absolute path",
		},
		{
			name:      "empty string",
			value:     "",
			fieldName: "payloadDir",
			jsonPath:  "gateway.payloadDir",
			shouldErr: true,
			errMsg:    "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AbsolutePath(tt.value, tt.fieldName, tt.jsonPath)

			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Contains(t, err.Error(), tt.jsonPath)
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}
