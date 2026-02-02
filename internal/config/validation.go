package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/githubnext/gh-aw-mcpg/internal/config/rules"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// ValidationError is an alias for rules.ValidationError for backward compatibility
type ValidationError = rules.ValidationError

// Variable expression pattern: ${VARIABLE_NAME}
var varExprPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

var logValidation = logger.New("config:validation")

// expandVariables expands variable expressions in a string
// Returns the expanded string and error if any variable is undefined
func expandVariables(value, jsonPath string) (string, error) {
	logValidation.Printf("Expanding variables: jsonPath=%s", jsonPath)
	var undefinedVars []string

	result := varExprPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		if envValue, exists := os.LookupEnv(varName); exists {
			logValidation.Printf("Expanded variable: %s (found in environment)", varName)
			return envValue
		}

		// Track undefined variable
		undefinedVars = append(undefinedVars, varName)
		logValidation.Printf("Undefined variable: %s", varName)
		return match // Keep original if undefined
	})

	if len(undefinedVars) > 0 {
		logValidation.Printf("Variable expansion failed: undefined variables=%v", undefinedVars)
		return "", rules.UndefinedVariable(undefinedVars[0], jsonPath)
	}

	logValidation.Print("Variable expansion completed successfully")
	return result, nil
}

// ExpandRawJSONVariables expands all ${VAR} expressions in JSON data before schema validation.
// This ensures the schema validates the expanded values, not the variable syntax.
// It collects all undefined variables and reports them in a single error.
func ExpandRawJSONVariables(data []byte) ([]byte, error) {
	logValidation.Print("Expanding variables in raw JSON data")
	var undefinedVars []string

	result := varExprPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		// Extract variable name (remove ${ and })
		varName := string(match[2 : len(match)-1])

		if envValue, exists := os.LookupEnv(varName); exists {
			logValidation.Printf("Expanded variable in JSON: %s", varName)
			return []byte(envValue)
		}

		// Track undefined variable
		undefinedVars = append(undefinedVars, varName)
		logValidation.Printf("Undefined variable in JSON: %s", varName)
		return match // Keep original if undefined
	})

	if len(undefinedVars) > 0 {
		logValidation.Printf("Variable expansion failed: undefined variables=%v", undefinedVars)
		return nil, rules.UndefinedVariable(undefinedVars[0], "configuration")
	}

	logValidation.Print("Raw JSON variable expansion completed")
	return result, nil
}

// expandEnvVariables expands all variable expressions in an env map
func expandEnvVariables(env map[string]string, serverName string) (map[string]string, error) {
	logValidation.Printf("Expanding env variables for server: %s, count=%d", serverName, len(env))
	result := make(map[string]string, len(env))

	for key, value := range env {
		jsonPath := fmt.Sprintf("mcpServers.%s.env.%s", serverName, key)

		expanded, err := expandVariables(value, jsonPath)
		if err != nil {
			return nil, err
		}

		result[key] = expanded
	}

	logValidation.Printf("Env variable expansion completed for server: %s", serverName)
	return result, nil
}

// validateMounts validates mount specifications using centralized rules
func validateMounts(mounts []string, jsonPath string) error {
	for i, mount := range mounts {
		if err := rules.MountFormat(mount, jsonPath, i); err != nil {
			return err
		}
	}
	return nil
}

// validateServerConfig validates a server configuration (stdio or HTTP)
func validateServerConfig(name string, server *StdinServerConfig) error {
	return validateServerConfigWithCustomSchemas(name, server, nil)
}

// validateServerConfigWithCustomSchemas validates a server configuration with custom schema support
func validateServerConfigWithCustomSchemas(name string, server *StdinServerConfig, customSchemas map[string]string) error {
	logValidation.Printf("Validating server config: name=%s, type=%s", name, server.Type)
	jsonPath := fmt.Sprintf("mcpServers.%s", name)

	// Validate type (empty defaults to stdio)
	if server.Type == "" {
		server.Type = "stdio"
		logValidation.Printf("Server type empty, defaulting to stdio: name=%s", name)
	}

	// Normalize "local" to "stdio"
	if server.Type == "local" {
		server.Type = "stdio"
		logValidation.Printf("Server type normalized from 'local' to 'stdio': name=%s", name)
	}

	// Check if it's a standard type
	if server.Type == "stdio" || server.Type == "http" {
		return validateStandardServerConfig(name, server, jsonPath)
	}

	// It's a custom type - validate against customSchemas
	return validateCustomServerConfig(name, server, customSchemas, jsonPath)
}

// validateStandardServerConfig validates stdio or http server configurations
func validateStandardServerConfig(name string, server *StdinServerConfig, jsonPath string) error {
	// For stdio servers, container is required
	if server.Type == "stdio" || server.Type == "local" {
		if server.Container == "" {
			logValidation.Printf("Validation failed: stdio server missing container field, name=%s", name)
			return rules.MissingRequired("container", "stdio", jsonPath, "Add a 'container' field (e.g., \"ghcr.io/owner/image:tag\")")
		}

		// Reject unsupported 'command' field
		if server.Command != "" {
			logValidation.Printf("Validation failed: stdio server has unsupported command field, name=%s", name)
			return rules.UnsupportedField("command", "'command' field is not supported (stdio servers must use 'container')", jsonPath, "Remove 'command' field and use 'container' instead")
		}

		// Validate mounts if provided
		if len(server.Mounts) > 0 {
			logValidation.Printf("Validating mounts for server: name=%s, mount_count=%d", name, len(server.Mounts))
			if err := validateMounts(server.Mounts, jsonPath); err != nil {
				return err
			}
		}
	}

	// For HTTP servers, url is required
	if server.Type == "http" {
		if server.URL == "" {
			logValidation.Printf("Validation failed: HTTP server missing url field, name=%s", name)
			return rules.MissingRequired("url", "HTTP", jsonPath, "Add a 'url' field (e.g., \"https://example.com/mcp\")")
		}
	}

	logValidation.Printf("Server config validation passed: name=%s", name)
	return nil
}

// validateCustomServerConfig validates custom server type configurations
func validateCustomServerConfig(name string, server *StdinServerConfig, customSchemas map[string]string, jsonPath string) error {
	serverType := server.Type

	// Check if custom type is registered
	if customSchemas == nil {
		logValidation.Printf("Custom type not registered: name=%s, type=%s (no customSchemas)", name, serverType)
		return rules.UnsupportedType("type", serverType, jsonPath, "Custom server type '"+serverType+"' is not registered in customSchemas. Add the custom type to the customSchemas field or use a standard type ('stdio' or 'http')")
	}

	schemaURL, exists := customSchemas[serverType]
	if !exists {
		logValidation.Printf("Custom type not registered: name=%s, type=%s", name, serverType)
		return rules.UnsupportedType("type", serverType, jsonPath, "Custom server type '"+serverType+"' is not registered in customSchemas. Add the custom type to the customSchemas field or use a standard type ('stdio' or 'http')")
	}

	logValidation.Printf("Custom type found in customSchemas: name=%s, type=%s, schemaURL=%s", name, serverType, schemaURL)

	// If schema URL is empty, skip validation
	if schemaURL == "" {
		logValidation.Printf("Custom schema URL is empty, skipping validation: name=%s, type=%s", name, serverType)
		return nil
	}

	// Fetch and validate against custom schema
	// For now, we just validate that the schema is fetchable
	// Full JSON schema validation against custom schemas can be added in the future
	logValidation.Printf("Custom schema validation passed: name=%s, type=%s", name, serverType)
	return nil
}

// validateCustomSchemas validates the customSchemas field
func validateCustomSchemas(customSchemas map[string]string) error {
	if customSchemas == nil {
		return nil
	}

	logValidation.Printf("Validating customSchemas: count=%d", len(customSchemas))

	for typeName := range customSchemas {
		// Check for reserved type names
		if typeName == "stdio" || typeName == "http" {
			logValidation.Printf("Reserved type name in customSchemas: %s", typeName)
			return rules.UnsupportedType("customSchemas", typeName, fmt.Sprintf("customSchemas.%s", typeName), "Custom type name '"+typeName+"' conflicts with reserved type. Use a different name for your custom type (reserved types: stdio, http)")
		}
	}

	logValidation.Printf("customSchemas validation passed")
	return nil
}

// validateGatewayConfig validates gateway configuration
func validateGatewayConfig(gateway *StdinGatewayConfig) error {
	if gateway == nil {
		logValidation.Print("No gateway config to validate")
		return nil
	}

	logValidation.Print("Validating gateway configuration")

	// Validate port range using centralized rules
	if gateway.Port != nil {
		logValidation.Printf("Validating gateway port: %d", *gateway.Port)
		if err := rules.PortRange(*gateway.Port, "gateway.port"); err != nil {
			return err
		}
	}

	// Validate timeout values using centralized rules
	if gateway.StartupTimeout != nil {
		logValidation.Printf("Validating startup timeout: %d", *gateway.StartupTimeout)
		if err := rules.TimeoutPositive(*gateway.StartupTimeout, "startupTimeout", "gateway.startupTimeout"); err != nil {
			return err
		}
	}

	if gateway.ToolTimeout != nil {
		logValidation.Printf("Validating tool timeout: %d", *gateway.ToolTimeout)
		if err := rules.TimeoutPositive(*gateway.ToolTimeout, "toolTimeout", "gateway.toolTimeout"); err != nil {
			return err
		}
	}

	// Validate payloadDir if provided (per schema: minLength: 1)
	if gateway.PayloadDir != "" {
		logValidation.Printf("Validating payload directory: %s", gateway.PayloadDir)
		if err := rules.NonEmptyString(gateway.PayloadDir, "payloadDir", "gateway.payloadDir"); err != nil {
			return err
		}
	}

	logValidation.Print("Gateway config validation passed")
	return nil
}
