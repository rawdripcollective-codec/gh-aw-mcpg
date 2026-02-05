package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	tests := []struct {
		name           string
		inputVersion   string
		expectedResult string
	}{
		{
			name:           "set valid version",
			inputVersion:   "v1.2.3",
			expectedResult: "v1.2.3",
		},
		{
			name:           "set version with build metadata",
			inputVersion:   "v1.2.3, commit: abc1234, built: 2024-01-01",
			expectedResult: "v1.2.3, commit: abc1234, built: 2024-01-01",
		},
		{
			name:           "empty string does not change version",
			inputVersion:   "",
			expectedResult: "dev", // should remain default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset to default before each test
			gatewayVersion = "dev"

			Set(tt.inputVersion)
			result := Get()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGet(t *testing.T) {
	// Reset to default
	gatewayVersion = "dev"

	// Test default value
	result := Get()
	require.Equal(t, "dev", result, "Default version should be 'dev'")

	// Test after setting a value
	Set("v2.0.0")
	result = Get()
	assert.Equal(t, "v2.0.0", result, "Version should be updated to 'v2.0.0'")
}

func TestSetPreservesVersionOnEmpty(t *testing.T) {
	// Set an initial version
	gatewayVersion = "v1.0.0"

	// Try to set empty string
	Set("")

	// Version should remain unchanged
	result := Get()
	assert.Equal(t, "v1.0.0", result, "Version should not change when empty string is provided")
}
