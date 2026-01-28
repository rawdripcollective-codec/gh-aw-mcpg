package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateContainerID(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		shouldError bool
	}{
		{
			name:        "valid 64-char hex",
			containerID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			shouldError: false,
		},
		{
			name:        "valid 12-char hex (short form)",
			containerID: "abcdef123456",
			shouldError: false,
		},
		{
			name:        "valid with all hex digits",
			containerID: "0123456789abcdef",
			shouldError: false,
		},
		{
			name:        "empty string",
			containerID: "",
			shouldError: true,
		},
		{
			name:        "too short (11 chars)",
			containerID: "abcdef12345",
			shouldError: true,
		},
		{
			name:        "too long (65 chars)",
			containerID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678901",
			shouldError: true,
		},
		{
			name:        "invalid chars - uppercase",
			containerID: "ABCDEF123456",
			shouldError: true,
		},
		{
			name:        "invalid chars - special",
			containerID: "abc;def123456",
			shouldError: true,
		},
		{
			name:        "command injection attempt",
			containerID: "abcdef123456; rm -rf /",
			shouldError: true,
		},
		{
			name:        "path injection attempt",
			containerID: "../../../etc/passwd",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContainerID(tt.containerID)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestDetectContainerized(t *testing.T) {
	// This test verifies the function doesn't panic and returns consistent results
	isContainerized, containerID := detectContainerized()

	// In a test environment, we're typically not containerized
	// but we just verify the function works
	t.Logf("detectContainerized: isContainerized=%v, containerID=%s", isContainerized, containerID)

	// If we detect a container, the ID should have some content
	if isContainerized && containerID != "" {
		if len(containerID) < 12 {
			t.Errorf("Container ID should be at least 12 characters, got %d", len(containerID))
		}
	}
}

func TestCheckRequiredEnvVars(t *testing.T) {
	// Clear any existing env vars for the test
	for _, v := range RequiredEnvVars {
		os.Unsetenv(v)
	}
	defer func() {
		for _, v := range RequiredEnvVars {
			os.Unsetenv(v)
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected []string
	}{
		{
			name:     "all missing",
			envVars:  map[string]string{},
			expected: RequiredEnvVars,
		},
		{
			name: "all set",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "8080",
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: nil,
		},
		{
			name: "partial set - missing port",
			envVars: map[string]string{
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_PORT"},
		},
		{
			name: "partial set - missing domain",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "8080",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_DOMAIN"},
		},
		{
			name: "partial set - missing api key",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":   "8080",
				"MCP_GATEWAY_DOMAIN": "localhost",
			},
			expected: []string{"MCP_GATEWAY_API_KEY"},
		},
		{
			name: "empty string values are missing",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "",
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_PORT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			for _, v := range RequiredEnvVars {
				os.Unsetenv(v)
			}

			// Set up test environment
			for k, v := range tt.envVars {
				if v != "" {
					os.Setenv(k, v)
				}
			}

			missing := checkRequiredEnvVars()

			if len(missing) != len(tt.expected) {
				t.Errorf("Expected %d missing vars, got %d. Missing: %v", len(tt.expected), len(missing), missing)
				return
			}

			// Check each expected var is in the missing list
			for _, expectedVar := range tt.expected {
				found := false
				for _, missingVar := range missing {
					if missingVar == expectedVar {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected %s to be in missing list, but it wasn't. Missing: %v", expectedVar, missing)
				}
			}
		})
	}
}

func TestGetGatewayPortFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		setEnv      bool
		expected    int
		shouldError bool
	}{
		{
			name:        "valid port",
			envValue:    "8080",
			setEnv:      true,
			expected:    8080,
			shouldError: false,
		},
		{
			name:        "min port",
			envValue:    "1",
			setEnv:      true,
			expected:    1,
			shouldError: false,
		},
		{
			name:        "max port",
			envValue:    "65535",
			setEnv:      true,
			expected:    65535,
			shouldError: false,
		},
		{
			name:        "port zero - invalid",
			envValue:    "0",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "port too high",
			envValue:    "65536",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "negative port",
			envValue:    "-1",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "non-numeric port",
			envValue:    "abc",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "not set",
			setEnv:      false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_PORT")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_PORT", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_PORT")

			port, err := GetGatewayPortFromEnv()

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, port, "port %d, got %d")
			}
		})
	}
}

func TestGetGatewayDomainFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
	}{
		{
			name:     "valid domain",
			envValue: "localhost",
			setEnv:   true,
		},
		{
			name:     "domain with subdomain",
			envValue: "mcp.example.com",
			setEnv:   true,
		},
		{
			name:   "not set",
			setEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_DOMAIN")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_DOMAIN", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_DOMAIN")

			domain := GetGatewayDomainFromEnv()

			if tt.setEnv && domain != tt.envValue {
				t.Errorf("Expected domain %s, got %s", tt.envValue, domain)
			}
			if !tt.setEnv && domain != "" {
				t.Errorf("Expected empty domain when not set, got %s", domain)
			}
		})
	}
}

func TestGetGatewayAPIKeyFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
	}{
		{
			name:     "valid key",
			envValue: "my-secret-key",
			setEnv:   true,
		},
		{
			name:     "complex key",
			envValue: "abc123!@#$%^&*()",
			setEnv:   true,
		},
		{
			name:   "not set",
			setEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_API_KEY")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_API_KEY", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_API_KEY")

			key := GetGatewayAPIKeyFromEnv()

			if tt.setEnv && key != tt.envValue {
				t.Errorf("Expected key %s, got %s", tt.envValue, key)
			}
			if !tt.setEnv && key != "" {
				t.Errorf("Expected empty key when not set, got %s", key)
			}
		})
	}
}

func TestEnvValidationResultIsValid(t *testing.T) {
	tests := []struct {
		name   string
		result *EnvValidationResult
		valid  bool
	}{
		{
			name:   "valid - no errors",
			result: &EnvValidationResult{},
			valid:  true,
		},
		{
			name: "valid - with warnings",
			result: &EnvValidationResult{
				ValidationWarnings: []string{"some warning"},
			},
			valid: true,
		},
		{
			name: "invalid - with errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"some error"},
			},
			valid: false,
		},
		{
			name: "invalid - multiple errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"error 1", "error 2"},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestEnvValidationResultError(t *testing.T) {
	tests := []struct {
		name     string
		result   *EnvValidationResult
		expected string
	}{
		{
			name:     "no errors",
			result:   &EnvValidationResult{},
			expected: "",
		},
		{
			name: "single error",
			result: &EnvValidationResult{
				ValidationErrors: []string{"Docker not accessible"},
			},
			expected: "Environment validation failed:\n  - Docker not accessible",
		},
		{
			name: "multiple errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"Error 1", "Error 2"},
			},
			expected: "Environment validation failed:\n  - Error 1\n  - Error 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateExecutionEnvironment(t *testing.T) {
	// This test verifies the function runs without panicking
	// The actual Docker check will fail in most test environments

	// Save original env vars
	origPort := os.Getenv("MCP_GATEWAY_PORT")
	origDomain := os.Getenv("MCP_GATEWAY_DOMAIN")
	origAPIKey := os.Getenv("MCP_GATEWAY_API_KEY")
	defer func() {
		if origPort != "" {
			os.Setenv("MCP_GATEWAY_PORT", origPort)
		}
		if origDomain != "" {
			os.Setenv("MCP_GATEWAY_DOMAIN", origDomain)
		}
		if origAPIKey != "" {
			os.Setenv("MCP_GATEWAY_API_KEY", origAPIKey)
		}
	}()

	t.Run("with all env vars set", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateExecutionEnvironment()

		// Should not have missing env vars
		assert.False(t, len(result.MissingEnvVars) > 0, "Expected no missing env vars, got %v")
	})

	t.Run("with missing env vars", func(t *testing.T) {
		os.Unsetenv("MCP_GATEWAY_PORT")
		os.Unsetenv("MCP_GATEWAY_DOMAIN")
		os.Unsetenv("MCP_GATEWAY_API_KEY")

		result := ValidateExecutionEnvironment()

		// Should have missing env vars
		if len(result.MissingEnvVars) != 3 {
			t.Errorf("Expected 3 missing env vars, got %d: %v", len(result.MissingEnvVars), result.MissingEnvVars)
		}

		// Should have validation errors
		if len(result.ValidationErrors) == 0 {
			t.Error("Expected validation errors for missing env vars")
		}
	})
}

func TestRunDockerInspect(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		formatTemplate string
		shouldError    bool
	}{
		{
			name:           "empty container ID",
			containerID:    "",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "invalid container ID - too short",
			containerID:    "abc123",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "invalid container ID - special chars",
			containerID:    "abc;def123456",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "valid container ID format - command will fail without docker",
			containerID:    "abcdef123456",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true, // Will fail because container doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runDockerInspect(tt.containerID, tt.formatTemplate)

			if tt.shouldError {
				assert.Error(t, err, "Expected error but got none")
				assert.Empty(t, output, "Expected empty output on error")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestCheckDockerAccessible(t *testing.T) {
	t.Run("check docker accessibility", func(t *testing.T) {
		// This test verifies the function runs without panicking
		// In CI environments, Docker may or may not be available
		result := checkDockerAccessible()
		t.Logf("Docker accessible: %v", result)
		// We don't assert the result since Docker availability varies by environment
	})

	t.Run("with custom DOCKER_HOST", func(t *testing.T) {
		// Test with a custom DOCKER_HOST that doesn't exist
		originalHost := os.Getenv("DOCKER_HOST")
		defer func() {
			if originalHost != "" {
				os.Setenv("DOCKER_HOST", originalHost)
			} else {
				os.Unsetenv("DOCKER_HOST")
			}
		}()

		os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
		result := checkDockerAccessible()
		assert.False(t, result, "Should return false for nonexistent socket")
	})

	t.Run("with unix:// prefix in DOCKER_HOST", func(t *testing.T) {
		originalHost := os.Getenv("DOCKER_HOST")
		defer func() {
			if originalHost != "" {
				os.Setenv("DOCKER_HOST", originalHost)
			} else {
				os.Unsetenv("DOCKER_HOST")
			}
		}()

		// Set DOCKER_HOST with unix:// prefix
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		// Function should strip the unix:// prefix and check the path
		checkDockerAccessible()
		// If it doesn't panic, the prefix stripping works
	})
}

func TestCheckPortMapping(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		port        string
		shouldError bool
	}{
		{
			name:        "empty container ID",
			containerID: "",
			port:        "8080",
			shouldError: true,
		},
		{
			name:        "invalid container ID",
			containerID: "invalid;id",
			port:        "8080",
			shouldError: true,
		},
		{
			name:        "valid container ID format - nonexistent container",
			containerID: "abcdef123456",
			port:        "8080",
			shouldError: true, // Will fail because container doesn't exist
		},
		{
			name:        "empty port",
			containerID: "abcdef123456",
			port:        "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped, err := checkPortMapping(tt.containerID, tt.port)

			if tt.shouldError {
				assert.Error(t, err, "Expected error for %s", tt.name)
				assert.False(t, mapped, "Port should not be mapped on error")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestCheckStdinInteractive(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		expected    bool
	}{
		{
			name:        "empty container ID",
			containerID: "",
			expected:    false,
		},
		{
			name:        "invalid container ID",
			containerID: "invalid;id",
			expected:    false,
		},
		{
			name:        "valid container ID format - nonexistent container",
			containerID: "abcdef123456",
			expected:    false, // Will fail because container doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkStdinInteractive(tt.containerID)
			assert.Equal(t, tt.expected, result, "Unexpected result for %s", tt.name)
		})
	}
}

func TestCheckLogDirMounted(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		logDir      string
		expected    bool
	}{
		{
			name:        "empty container ID",
			containerID: "",
			logDir:      "/tmp/gh-aw/mcp-logs",
			expected:    false,
		},
		{
			name:        "invalid container ID",
			containerID: "invalid;id",
			logDir:      "/tmp/gh-aw/mcp-logs",
			expected:    false,
		},
		{
			name:        "valid container ID format - nonexistent container",
			containerID: "abcdef123456",
			logDir:      "/tmp/gh-aw/mcp-logs",
			expected:    false, // Will fail because container doesn't exist
		},
		{
			name:        "empty log directory",
			containerID: "abcdef123456",
			logDir:      "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkLogDirMounted(tt.containerID, tt.logDir)
			assert.Equal(t, tt.expected, result, "Unexpected result for %s", tt.name)
		})
	}
}

func TestValidateContainerizedEnvironment(t *testing.T) {
	// Save original env vars
	origPort := os.Getenv("MCP_GATEWAY_PORT")
	origDomain := os.Getenv("MCP_GATEWAY_DOMAIN")
	origAPIKey := os.Getenv("MCP_GATEWAY_API_KEY")
	origLogDir := os.Getenv("MCP_GATEWAY_LOG_DIR")
	defer func() {
		if origPort != "" {
			os.Setenv("MCP_GATEWAY_PORT", origPort)
		} else {
			os.Unsetenv("MCP_GATEWAY_PORT")
		}
		if origDomain != "" {
			os.Setenv("MCP_GATEWAY_DOMAIN", origDomain)
		} else {
			os.Unsetenv("MCP_GATEWAY_DOMAIN")
		}
		if origAPIKey != "" {
			os.Setenv("MCP_GATEWAY_API_KEY", origAPIKey)
		} else {
			os.Unsetenv("MCP_GATEWAY_API_KEY")
		}
		if origLogDir != "" {
			os.Setenv("MCP_GATEWAY_LOG_DIR", origLogDir)
		} else {
			os.Unsetenv("MCP_GATEWAY_LOG_DIR")
		}
	}()

	t.Run("empty container ID", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateContainerizedEnvironment("")

		assert.True(t, result.IsContainerized, "Should be marked as containerized")
		assert.Equal(t, "", result.ContainerID, "Container ID should be empty")
		assert.False(t, result.IsValid(), "Should be invalid with empty container ID")
		assert.Contains(t, result.Error(), "Container ID could not be determined")
	})

	t.Run("valid container ID with all env vars", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized, "Should be marked as containerized")
		assert.Equal(t, "abcdef123456", result.ContainerID)
		// Will fail validation because Docker checks will fail in test environment
		// but we verify the container ID was set correctly
	})

	t.Run("missing required env vars", func(t *testing.T) {
		os.Unsetenv("MCP_GATEWAY_PORT")
		os.Unsetenv("MCP_GATEWAY_DOMAIN")
		os.Unsetenv("MCP_GATEWAY_API_KEY")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized)
		assert.Equal(t, "abcdef123456", result.ContainerID)
		assert.False(t, result.IsValid(), "Should be invalid with missing env vars")
		assert.Len(t, result.MissingEnvVars, 3, "Should have 3 missing env vars")
	})

	t.Run("port validation failure", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized)
		// Port mapping check will fail (container doesn't exist)
		assert.False(t, result.PortMapped, "Port should not be mapped for nonexistent container")
	})

	t.Run("stdin interactive check", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized)
		// Stdin check will fail (container doesn't exist)
		assert.False(t, result.StdinInteractive, "Stdin should not be interactive for nonexistent container")
	})

	t.Run("log directory mount check with default", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")
		os.Unsetenv("MCP_GATEWAY_LOG_DIR")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized)
		// Log dir check will fail (container doesn't exist)
		assert.False(t, result.LogDirMounted, "Log dir should not be mounted for nonexistent container")
		// Should have a warning about log dir not being mounted
		assert.Greater(t, len(result.ValidationWarnings), 0, "Should have warnings")
	})

	t.Run("log directory mount check with custom dir", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")
		os.Setenv("MCP_GATEWAY_LOG_DIR", "/custom/log/path")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.True(t, result.IsContainerized)
		assert.False(t, result.LogDirMounted)
		// Verify the warning mentions the custom path
		hasCustomPathWarning := false
		for _, warning := range result.ValidationWarnings {
			if assert.Contains(t, warning, "/custom/log/path") {
				hasCustomPathWarning = true
				break
			}
		}
		if len(result.ValidationWarnings) > 0 {
			assert.True(t, hasCustomPathWarning, "Should have warning with custom log path")
		}
	})

	t.Run("docker not accessible", func(t *testing.T) {
		// Set a DOCKER_HOST that doesn't exist
		originalHost := os.Getenv("DOCKER_HOST")
		defer func() {
			if originalHost != "" {
				os.Setenv("DOCKER_HOST", originalHost)
			} else {
				os.Unsetenv("DOCKER_HOST")
			}
		}()

		os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateContainerizedEnvironment("abcdef123456")

		assert.False(t, result.DockerAccessible, "Docker should not be accessible")
		assert.False(t, result.IsValid(), "Should be invalid when Docker is not accessible")
		// Should have error about Docker not being accessible
		hasDockerError := false
		for _, err := range result.ValidationErrors {
			if assert.Contains(t, err, "Docker daemon") {
				hasDockerError = true
				break
			}
		}
		assert.True(t, hasDockerError, "Should have Docker accessibility error")
	})

	t.Run("validation result error message format", func(t *testing.T) {
		os.Unsetenv("MCP_GATEWAY_PORT")
		os.Unsetenv("MCP_GATEWAY_DOMAIN")
		os.Unsetenv("MCP_GATEWAY_API_KEY")

		result := ValidateContainerizedEnvironment("abcdef123456")

		errorMsg := result.Error()
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")
		assert.Contains(t, errorMsg, "Environment validation failed", "Error should have header")
		// Each error should be on its own line with bullet point
		assert.Contains(t, errorMsg, "\n  - ", "Errors should be formatted with bullets")
	})
}
