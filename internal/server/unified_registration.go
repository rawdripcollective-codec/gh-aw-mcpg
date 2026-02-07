package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/github/gh-aw-mcpg/internal/launcher"
	"github.com/github/gh-aw-mcpg/internal/logger"
	"github.com/github/gh-aw-mcpg/internal/mcp"
	"github.com/github/gh-aw-mcpg/internal/middleware"
	"github.com/github/gh-aw-mcpg/internal/sys"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// launchResult tracks the result of launching a backend server
type launchResult struct {
	serverID string
	err      error
	duration time.Duration
}

// registerAllTools registers tools from all backend servers and sys tools (if DIFC is enabled)
func (us *UnifiedServer) registerAllTools() error {
	log.Println("Registering tools from all backends...")
	logUnified.Printf("Starting tool registration for %d backends", len(us.launcher.ServerIDs()))

	// Only register sys tools if DIFC is enabled
	// When DIFC is disabled (default), sys tools are not needed
	if us.enableDIFC {
		log.Println("DIFC enabled: registering sys tools...")
		if err := us.registerSysTools(); err != nil {
			log.Printf("Warning: failed to register sys tools: %v", err)
		}
	} else {
		log.Println("DIFC disabled: skipping sys tools registration")
	}

	serverIDs := us.launcher.ServerIDs()

	if us.sequentialLaunch {
		// Launch servers sequentially
		return us.registerAllToolsSequential(serverIDs)
	} else {
		// Launch servers in parallel (default behavior)
		return us.registerAllToolsParallel(serverIDs)
	}
}

// registerAllToolsSequential registers tools from backend servers sequentially
func (us *UnifiedServer) registerAllToolsSequential(serverIDs []string) error {
	logUnified.Printf("Registering tools sequentially from %d backends", len(serverIDs))

	for _, serverID := range serverIDs {
		logUnified.Printf("Registering tools from backend: %s", serverID)
		if err := us.registerToolsFromBackend(serverID); err != nil {
			log.Printf("Warning: failed to register tools from %s: %v", serverID, err)
			// Continue with other backends
		}
	}

	logUnified.Printf("Tool registration complete: total tools=%d", len(us.tools))
	return nil
}

// registerAllToolsParallel registers tools from backend servers in parallel
func (us *UnifiedServer) registerAllToolsParallel(serverIDs []string) error {
	logUnified.Printf("Registering tools in parallel from %d backends", len(serverIDs))

	var wg sync.WaitGroup
	results := make(chan launchResult, len(serverIDs))

	// Launch each server in its own goroutine
	for _, serverID := range serverIDs {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()

			startTime := time.Now()
			err := us.registerToolsFromBackend(sid)
			duration := time.Since(startTime)

			results <- launchResult{
				serverID: sid,
				err:      err,
				duration: duration,
			}
		}(serverID)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect and log results
	successCount := 0
	failureCount := 0
	for result := range results {
		if result.err != nil {
			log.Printf("Warning: failed to register tools from %s (took %v): %v", result.serverID, result.duration, result.err)
			logger.LogWarnWithServer(result.serverID, "backend", "Failed to register tools from %s (took %v): %v", result.serverID, result.duration, result.err)
			failureCount++
		} else {
			logUnified.Printf("Successfully registered tools from %s (took %v)", result.serverID, result.duration)
			logger.LogInfoWithServer(result.serverID, "backend", "Successfully registered tools from %s (took %v)", result.serverID, result.duration)
			successCount++
		}
	}

	log.Printf("Parallel tool registration complete: %d succeeded, %d failed, total tools=%d", successCount, failureCount, len(us.tools))
	logUnified.Printf("Tool registration complete: %d succeeded, %d failed, total tools=%d", successCount, failureCount, len(us.tools))
	return nil
}

// registerToolsFromBackend registers tools from a specific backend with <server>___<tool> naming
func (us *UnifiedServer) registerToolsFromBackend(serverID string) error {
	log.Printf("Registering tools from backend: %s", serverID)

	// Get connection to backend
	conn, err := launcher.GetOrLaunch(us.launcher, serverID)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Create a context with session ID for HTTP backends
	// HTTP backends may require Mcp-Session-Id header even during initialization
	ctx := context.WithValue(context.Background(), SessionIDContextKey, fmt.Sprintf("gateway-init-%s", serverID))

	// List tools from backend
	result, err := conn.SendRequestWithServerID(ctx, "tools/list", nil, serverID)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Check if the backend returned an error
	if result.Error != nil {
		return fmt.Errorf("backend error listing tools: code=%d, message=%s", result.Error.Code, result.Error.Message)
	}

	// Parse the result
	var listResult struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(result.Result, &listResult); err != nil {
		return fmt.Errorf("failed to parse tools: %w", err)
	}

	// Register each tool with prefixed name
	toolNames := []string{}
	for _, tool := range listResult.Tools {
		prefixedName := fmt.Sprintf("%s___%s", serverID, tool.Name)
		toolDesc := fmt.Sprintf("[%s] %s", serverID, tool.Description)
		toolNames = append(toolNames, prefixedName)

		// Normalize the input schema to fix common validation issues
		normalizedSchema := mcp.NormalizeInputSchema(tool.InputSchema, prefixedName)

		// Store tool info for routed mode
		us.toolsMu.Lock()
		us.tools[prefixedName] = &ToolInfo{
			Name:        prefixedName,
			Description: toolDesc,
			InputSchema: normalizedSchema,
			BackendID:   serverID,
		}
		us.toolsMu.Unlock()

		// Create a closure to capture serverID and toolName
		serverIDCopy := serverID
		toolNameCopy := tool.Name

		// Create the handler function
		handler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
			// Extract arguments from the request params (not the args parameter which is SDK internal state)
			toolArgs, err := parseToolArguments(req)
			if err != nil {
				logger.LogError("client", "Failed to unmarshal tool arguments, tool=%s, error=%v", toolNameCopy, err)
				return newErrorCallToolResult(err)
			}

			// Log the MCP tool call request
			sessionID := us.getSessionID(ctx)
			argsJSON, _ := json.Marshal(toolArgs)
			logger.LogInfo("client", "MCP tool call request, session=%s, tool=%s, args=%s", sessionID, toolNameCopy, string(argsJSON))

			// Check session is initialized
			if err := us.requireSession(ctx); err != nil {
				logger.LogError("client", "MCP tool call failed: session not initialized, session=%s, tool=%s", sessionID, toolNameCopy)
				return newErrorCallToolResult(err)
			}

			result, data, err := us.callBackendTool(ctx, serverIDCopy, toolNameCopy, toolArgs)

			// Log the MCP tool call response
			if err != nil {
				logger.LogError("client", "MCP tool call error, session=%s, tool=%s, error=%v", sessionID, toolNameCopy, err)
			} else {
				resultJSON, _ := json.Marshal(data)
				logger.LogInfo("client", "MCP tool call response, session=%s, tool=%s, result=%s", sessionID, toolNameCopy, string(resultJSON))
			}

			return result, data, err
		}

		// Wrap handler with jqschema middleware if applicable
		finalHandler := handler
		if middleware.ShouldApplyMiddleware(prefixedName) {
			finalHandler = middleware.WrapToolHandler(handler, prefixedName, us.payloadDir, us.payloadSizeThreshold, us.getSessionID)
		}

		// Store handler for routed mode to reuse
		us.toolsMu.Lock()
		us.tools[prefixedName].Handler = finalHandler
		us.toolsMu.Unlock()

		// Register the tool with the SDK using the Server.AddTool method (not sdk.AddTool function)
		// The method version does NOT perform schema validation, allowing us to include
		// InputSchema from backends that use different JSON Schema versions (e.g., draft-07)
		// without validation errors. This is critical for clients to understand tool parameters.
		//
		// We need to wrap our typed handler to match the simpler ToolHandler signature.
		// The typed handler signature: func(context.Context, *CallToolRequest, interface{}) (*CallToolResult, interface{}, error)
		// The simple handler signature: func(context.Context, *CallToolRequest) (*CallToolResult, error)
		wrappedHandler := func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			// Call the final handler (which may include middleware wrapping)
			// The third parameter would be the pre-unmarshaled/validated input if using sdk.AddTool,
			// but we handle unmarshaling ourselves in the handler, so we pass nil
			result, _, err := finalHandler(ctx, req, nil)
			return result, err
		}

		us.server.AddTool(&sdk.Tool{
			Name:        prefixedName,
			Description: toolDesc,
			InputSchema: normalizedSchema, // Include the schema for clients to understand parameters
		}, wrappedHandler)

		log.Printf("Registered tool: %s", prefixedName)
	}

	log.Printf("Registered %d tools from %s: %v", len(listResult.Tools), serverID, toolNames)
	return nil
}

// registerSysTools registers built-in sys tools
func (us *UnifiedServer) registerSysTools() error {
	// Create sys_init handler
	sysInitHandler := func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		// Extract arguments from the request params
		toolArgs, err := parseToolArguments(req)
		if err != nil {
			logger.LogError("client", "Failed to unmarshal sys_init arguments, error=%v", err)
			return newErrorCallToolResult(err)
		}

		// Get agent ID from the incoming token (via context)
		// The token should be passed to initAgentLabels to initialize state
		sessionID := us.getSessionID(ctx)

		// Call sys.Init to initialize sys_init tooling
		// This validates the sys___init input and extracts agentId and labels
		result, err := us.sysServer.Init(ctx, toolArgs)
		if err != nil {
			logger.LogError("client", "sys_init call failed, session=%s, error=%v", sessionID, err)
			return newErrorCallToolResult(err)
		}

		// Store session
		us.sessionMu.Lock()
		us.sessions[sessionID] = NewSession(sessionID, "")
		us.sessionMu.Unlock()

		// Ensure session directory exists in payload mount point
		if err := us.ensureSessionDirectory(sessionID); err != nil {
			logger.LogWarn("client", "Failed to create session directory for session=%s: %v", sessionID, err)
			// Don't fail - payloads will attempt to create the directory when needed
		}

		logger.LogInfo("client", "sys_init successful, session=%s", sessionID)

		// Convert result to SDK format
		callResult, err := convertToCallToolResult(result)
		if err != nil {
			return newErrorCallToolResult(fmt.Errorf("failed to convert sys_init result: %w", err))
		}

		return callResult, result, nil
	}

	// Wrap sys_init handler with error handling
	sysInitWrapped := func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		result, _, err := sysInitHandler(ctx, req, nil)
		return result, err
	}

	// Register sys_init tool
	sysInitTool := sys.GetSysInitTool()
	us.server.AddTool(sysInitTool, sysInitWrapped)

	// Store tool info for routed mode
	us.toolsMu.Lock()
	us.tools[sysInitTool.Name] = &ToolInfo{
		Name:        sysInitTool.Name,
		Description: sysInitTool.Description,
		InputSchema: sysInitTool.InputSchema,
		BackendID:   "sys",
		Handler:     sysInitHandler,
	}
	us.toolsMu.Unlock()

	log.Printf("Registered sys tool: %s", sysInitTool.Name)
	return nil
}

// registerGuard registers the appropriate guard for a backend server
func (us *UnifiedServer) registerGuard(serverID string) {
	// For now, register NoopGuard for all backends
	// In the future, this could be configured per-backend
	us.guardRegistry.RegisterForBackend(serverID, "noop")
	log.Printf("Registered noop guard for backend: %s", serverID)
}
