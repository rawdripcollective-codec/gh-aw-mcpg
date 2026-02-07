package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultPayloadSizeThreshold(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{
			name:     "no env var - returns default",
			envValue: "",
			expected: defaultPayloadSizeThreshold,
		},
		{
			name:     "valid env var",
			envValue: "2048",
			expected: 2048,
		},
		{
			name:     "very large threshold",
			envValue: "10240",
			expected: 10240,
		},
		{
			name:     "small threshold",
			envValue: "512",
			expected: 512,
		},
		{
			name:     "invalid value - non-numeric",
			envValue: "invalid",
			expected: defaultPayloadSizeThreshold,
		},
		{
			name:     "invalid value - negative",
			envValue: "-100",
			expected: defaultPayloadSizeThreshold,
		},
		{
			name:     "invalid value - zero",
			envValue: "0",
			expected: defaultPayloadSizeThreshold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set or unset environment variable
			if tt.envValue != "" {
				os.Setenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD", tt.envValue)
				defer os.Unsetenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD")
			} else {
				os.Unsetenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD")
			}

			result := getDefaultPayloadSizeThreshold()
			assert.Equal(t, tt.expected, result, "Threshold should match expected value")
		})
	}
}

func TestPayloadSizeThresholdFlagDefault(t *testing.T) {
	// Ensure environment is clean
	os.Unsetenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD")

	result := getDefaultPayloadSizeThreshold()
	assert.Equal(t, 1024, result, "Default should be 1024 bytes")
}

func TestPayloadSizeThresholdEnvVar(t *testing.T) {
	// Test that environment variable overrides default
	os.Setenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD", "4096")
	defer os.Unsetenv("MCP_GATEWAY_PAYLOAD_SIZE_THRESHOLD")

	result := getDefaultPayloadSizeThreshold()
	assert.Equal(t, 4096, result, "Environment variable should override default")
}
