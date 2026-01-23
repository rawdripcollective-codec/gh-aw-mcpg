package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/config/rules"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	// Compile regex patterns from schema for additional validation
	containerPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9./_-]*(:([a-zA-Z0-9._-]+|latest))?$`)
	urlPattern       = regexp.MustCompile(`^https?://.+`)
	mountPattern     = regexp.MustCompile(`^[^:]+:[^:]+(:(ro|rw))?$`)
	domainVarPattern = regexp.MustCompile(`^\$\{[A-Z_][A-Z0-9_]*\}$`)

	// gatewayVersion stores the version string to include in error messages
	gatewayVersion = "dev"
	
	// logSchema is the debug logger for schema validation
	logSchema = logger.New("config:validation_schema")
)

// SetGatewayVersion sets the gateway version for error reporting
func SetGatewayVersion(version string) {
	if version != "" {
		gatewayVersion = version
	}
}

// fetchAndFixSchema fetches the JSON schema from the remote URL and fixes
// regex patterns that use negative lookahead (not supported in JSON Schema Draft 7)
func fetchAndFixSchema(url string) ([]byte, error) {
	logSchema.Printf("Fetching schema from URL: %s", url)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		logSchema.Printf("Schema fetch failed: %v", err)
		return nil, fmt.Errorf("failed to fetch schema from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logSchema.Printf("Schema fetch returned non-OK status: %d", resp.StatusCode)
		return nil, fmt.Errorf("failed to fetch schema: HTTP %d", resp.StatusCode)
	}
	
	logSchema.Print("Schema fetched successfully, applying fixes")

	schemaBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema response: %w", err)
	}

	// Fix regex patterns that use negative lookahead
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Fix the customServerConfig pattern that uses negative lookahead
	// The oneOf constraint in mcpServerConfig will still ensure that stdio/http
	// types are validated correctly. We replace the pattern with an enum that excludes
	// stdio and http, which achieves the same validation goal without negative lookahead.
	if definitions, ok := schema["definitions"].(map[string]interface{}); ok {
		if customServerConfig, ok := definitions["customServerConfig"].(map[string]interface{}); ok {
			if properties, ok := customServerConfig["properties"].(map[string]interface{}); ok {
				if typeField, ok := properties["type"].(map[string]interface{}); ok {
					// Remove the pattern entirely - the oneOf logic combined with the fact
					// that stdioServerConfig has enum: ["stdio"] and httpServerConfig has
					// enum: ["http"] will ensure proper validation
					delete(typeField, "pattern")
					// Also remove the type constraint since we want it to only match in the oneOf context
					delete(typeField, "type")
					// Add a not constraint to exclude stdio and http
					typeField["not"] = map[string]interface{}{
						"enum": []string{"stdio", "http"},
					}
				}
			}
		}
	}

	// Fix the customSchemas patternProperties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		if customSchemas, ok := properties["customSchemas"].(map[string]interface{}); ok {
			if patternProps, ok := customSchemas["patternProperties"].(map[string]interface{}); ok {
				// Find and replace the pattern property key with negative lookahead
				for key, value := range patternProps {
					if strings.Contains(key, "(?!") {
						// Replace with a simple pattern that matches any lowercase word
						// The validation logic will handle ensuring it's not stdio/http
						delete(patternProps, key)
						patternProps["^[a-z][a-z0-9-]*$"] = value
						break
					}
				}
			}
		}
	}

	fixedBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fixed schema: %w", err)
	}

	return fixedBytes, nil
}

// validateJSONSchema validates the raw JSON configuration against the JSON schema
func validateJSONSchema(data []byte) error {
	logSchema.Printf("Starting JSON schema validation: data_size=%d bytes", len(data))
	
	// Fetch the schema from the remote URL (source of truth)
	schemaURL := "https://raw.githubusercontent.com/githubnext/gh-aw/main/docs/public/schemas/mcp-gateway-config.schema.json"
	schemaJSON, err := fetchAndFixSchema(schemaURL)
	if err != nil {
		return fmt.Errorf("failed to fetch schema: %w", err)
	}

	// Parse the schema
	var schemaData interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	// Add the schema with its $id from the remote schema
	// Note: The remote schema uses https://docs.github.com/gh-aw/schemas/mcp-gateway-config.schema.json
	// as its $id, so we need to register it there as well
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaObj); err != nil {
		return fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	schemaID, ok := schemaObj["$id"].(string)
	if !ok || schemaID == "" {
		schemaID = schemaURL
	}

	// Add the schema with both URLs
	if err := compiler.AddResource(schemaURL, strings.NewReader(string(schemaJSON))); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}
	if schemaID != schemaURL {
		if err := compiler.AddResource(schemaID, strings.NewReader(string(schemaJSON))); err != nil {
			return fmt.Errorf("failed to add schema resource with $id: %w", err)
		}
	}

	schema, err := compiler.Compile(schemaID)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	// Parse the configuration
	var configObj interface{}
	if err := json.Unmarshal(data, &configObj); err != nil {
		return fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	// Validate the configuration
	if err := schema.Validate(configObj); err != nil {
		logSchema.Printf("Schema validation failed: %v", err)
		return formatSchemaError(err)
	}

	logSchema.Print("Schema validation completed successfully")
	return nil
}

// formatSchemaError formats JSON schema validation errors to be user-friendly
func formatSchemaError(err error) error {
	if err == nil {
		return nil
	}

	// The jsonschema library returns a ValidationError type with detailed info
	if ve, ok := err.(*jsonschema.ValidationError); ok {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Configuration validation error (MCP Gateway version: %s):\n\n", gatewayVersion))

		// Recursively format all errors
		formatValidationErrorRecursive(ve, &sb, 0)

		rules.AppendConfigDocsFooter(&sb)

		return fmt.Errorf("%s", sb.String())
	}

	return fmt.Errorf("configuration validation error (version: %s): %s", gatewayVersion, err.Error())
}

// formatValidationErrorRecursive recursively formats validation errors with proper indentation
func formatValidationErrorRecursive(ve *jsonschema.ValidationError, sb *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)

	// Format location and message
	location := ve.InstanceLocation
	if location == "" {
		location = "<root>"
	}
	fmt.Fprintf(sb, "%sLocation: %s\n", indent, location)
	fmt.Fprintf(sb, "%sError: %s\n", indent, ve.Message)

	// Add detailed context based on the error message
	context := formatErrorContext(ve, indent)
	if context != "" {
		sb.WriteString(context)
	}

	// Recursively process nested causes
	if len(ve.Causes) > 0 {
		for _, cause := range ve.Causes {
			formatValidationErrorRecursive(cause, sb, depth+1)
		}
	}

	// Add spacing between sibling errors at the same level
	if depth == 0 {
		sb.WriteString("\n")
	}
}

// formatErrorContext provides additional context about what caused the validation error
func formatErrorContext(ve *jsonschema.ValidationError, prefix string) string {
	var sb strings.Builder
	msg := ve.Message

	// For additional properties errors, explain what's wrong
	if strings.Contains(msg, "additionalProperties") || strings.Contains(msg, "additional property") {
		sb.WriteString(fmt.Sprintf("%sDetails: Configuration contains field(s) that are not defined in the schema\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Check for typos in field names or remove unsupported fields\n", prefix))
	}

	// For type errors, show the mismatch
	if strings.Contains(msg, "expected") && (strings.Contains(msg, "but got") || strings.Contains(msg, "type")) {
		sb.WriteString(fmt.Sprintf("%sDetails: Type mismatch - the value type doesn't match what's expected\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Verify the value is the correct type (string, number, boolean, object, array)\n", prefix))
	}

	// For enum errors (invalid values from a set of allowed values)
	if strings.Contains(msg, "value must be one of") || strings.Contains(msg, "must be") {
		sb.WriteString(fmt.Sprintf("%sDetails: Invalid value - the field has a restricted set of allowed values\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Check the documentation for the list of valid values\n", prefix))
	}

	// For missing required properties
	if strings.Contains(msg, "missing properties") || strings.Contains(msg, "required") {
		sb.WriteString(fmt.Sprintf("%sDetails: Required field(s) are missing\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Add the required field(s) to your configuration\n", prefix))
	}

	// For pattern validation failures (regex patterns)
	if strings.Contains(msg, "does not match pattern") || strings.Contains(msg, "pattern") {
		sb.WriteString(fmt.Sprintf("%sDetails: Value format is incorrect\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → The value must match a specific format or pattern\n", prefix))
	}

	// For minimum/maximum constraint violations
	if strings.Contains(msg, "must be >=") || strings.Contains(msg, "must be <=") || strings.Contains(msg, "minimum") || strings.Contains(msg, "maximum") {
		sb.WriteString(fmt.Sprintf("%sDetails: Value is outside the allowed range\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Adjust the value to be within the valid range\n", prefix))
	}

	// For oneOf errors (typically type selection issues)
	if strings.Contains(msg, "doesn't validate with any of") || strings.Contains(msg, "oneOf") {
		sb.WriteString(fmt.Sprintf("%sDetails: Configuration doesn't match any of the expected formats\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Review the structure and ensure it matches one of the valid configuration types\n", prefix))
	}

	// Add keyword location if it provides useful context
	if ve.KeywordLocation != "" && ve.KeywordLocation != ve.InstanceLocation {
		sb.WriteString(fmt.Sprintf("%sSchema location: %s\n", prefix, ve.KeywordLocation))
	}

	return sb.String()
}

// validateStringPatterns validates string fields against regex patterns from the schema
// This provides additional validation beyond the JSON schema validation
func validateStringPatterns(stdinCfg *StdinConfig) error {
	logSchema.Printf("Validating string patterns: server_count=%d", len(stdinCfg.MCPServers))
	
	// Validate server configurations
	for name, server := range stdinCfg.MCPServers {
		jsonPath := fmt.Sprintf("mcpServers.%s", name)
		logSchema.Printf("Validating server: name=%s, type=%s", name, server.Type)

		// Validate container pattern for stdio servers
		if server.Type == "" || server.Type == "stdio" || server.Type == "local" {
			if server.Container != "" && !containerPattern.MatchString(server.Container) {
				return &rules.ValidationError{
					Field:      "container",
					Message:    fmt.Sprintf("container image '%s' does not match required pattern", server.Container),
					JSONPath:   fmt.Sprintf("%s.container", jsonPath),
					Suggestion: "Use a valid container image format (e.g., 'ghcr.io/owner/image:tag' or 'owner/image:latest')",
				}
			}

			// Validate mount patterns
			for i, mount := range server.Mounts {
				if !mountPattern.MatchString(mount) {
					return &rules.ValidationError{
						Field:      "mounts",
						Message:    fmt.Sprintf("mount '%s' does not match required pattern", mount),
						JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
						Suggestion: "Use format 'source:dest:mode' where mode is 'ro' or 'rw'",
					}
				}
			}

			// Validate entrypoint is not empty if provided
			if server.Entrypoint != "" && len(strings.TrimSpace(server.Entrypoint)) == 0 {
				return &rules.ValidationError{
					Field:      "entrypoint",
					Message:    "entrypoint cannot be empty or whitespace only",
					JSONPath:   fmt.Sprintf("%s.entrypoint", jsonPath),
					Suggestion: "Provide a valid entrypoint path or remove the field",
				}
			}
		}

		// Validate URL pattern for HTTP servers
		if server.Type == "http" {
			if server.URL != "" && !urlPattern.MatchString(server.URL) {
				return &rules.ValidationError{
					Field:      "url",
					Message:    fmt.Sprintf("url '%s' does not match required pattern", server.URL),
					JSONPath:   fmt.Sprintf("%s.url", jsonPath),
					Suggestion: "Use a valid HTTP or HTTPS URL (e.g., 'https://api.example.com/mcp')",
				}
			}
		}
	}

	// Validate gateway configuration patterns
	if stdinCfg.Gateway != nil {
		// Validate port: must be integer 1-65535 or variable expression
		if stdinCfg.Gateway.Port != nil {
			if err := rules.PortRange(*stdinCfg.Gateway.Port, "gateway.port"); err != nil {
				return err
			}
		}

		// Validate domain: must be "localhost", "host.docker.internal", or variable expression
		if stdinCfg.Gateway.Domain != "" {
			domain := stdinCfg.Gateway.Domain
			if domain != "localhost" && domain != "host.docker.internal" && !domainVarPattern.MatchString(domain) {
				return &rules.ValidationError{
					Field:      "domain",
					Message:    fmt.Sprintf("domain '%s' must be 'localhost', 'host.docker.internal', or a variable expression", domain),
					JSONPath:   "gateway.domain",
					Suggestion: "Use 'localhost', 'host.docker.internal', or a variable like '${MCP_GATEWAY_DOMAIN}'",
				}
			}
		}

		// Validate timeouts are positive
		if stdinCfg.Gateway.StartupTimeout != nil {
			if err := rules.TimeoutPositive(*stdinCfg.Gateway.StartupTimeout, "startupTimeout", "gateway.startupTimeout"); err != nil {
				return err
			}
		}

		if stdinCfg.Gateway.ToolTimeout != nil {
			if err := rules.TimeoutPositive(*stdinCfg.Gateway.ToolTimeout, "toolTimeout", "gateway.toolTimeout"); err != nil {
				return err
			}
		}
	}

	return nil
}
