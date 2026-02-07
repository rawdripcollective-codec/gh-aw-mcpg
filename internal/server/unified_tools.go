package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/github/gh-aw-mcpg/internal/difc"
	"github.com/github/gh-aw-mcpg/internal/guard"
	"github.com/github/gh-aw-mcpg/internal/launcher"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// guardBackendCaller implements guard.BackendCaller for guards to query backend metadata
type guardBackendCaller struct {
	server   *UnifiedServer
	serverID string
	ctx      context.Context
}

func (g *guardBackendCaller) CallTool(ctx context.Context, toolName string, args interface{}) (interface{}, error) {
	// Make a read-only call to the backend for metadata
	// This bypasses DIFC checks since it's internal to the guard
	log.Printf("[DIFC] Guard calling backend %s tool %s for metadata", g.serverID, toolName)

	// Get or launch backend connection (use session-aware connection for stateful backends)
	sessionID := g.ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		sessionID = "default"
	}
	conn, err := launcher.GetOrLaunchForSession(g.server.launcher, g.serverID, sessionID.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	response, err := conn.SendRequestWithServerID(g.ctx, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}, g.serverID)
	if err != nil {
		return nil, err
	}

	// Check if the backend returned an error
	if response.Error != nil {
		return nil, fmt.Errorf("backend error: code=%d, message=%s", response.Error.Code, response.Error.Message)
	}

	// Parse the result
	var result interface{}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	return result, nil
}

// convertToCallToolResult converts backend result data to SDK CallToolResult format
// The backend returns a JSON object with a "content" field containing an array of content items
func convertToCallToolResult(data interface{}) (*sdk.CallToolResult, error) {
	// Try to marshal and unmarshal to get the structure
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal backend result: %w", err)
	}

	// Parse the backend result structure
	var backendResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}

	if err := json.Unmarshal(dataBytes, &backendResult); err != nil {
		return nil, fmt.Errorf("failed to parse backend result structure: %w", err)
	}

	// Convert content items to SDK Content format
	content := make([]sdk.Content, 0, len(backendResult.Content))
	for _, item := range backendResult.Content {
		switch item.Type {
		case "text":
			content = append(content, &sdk.TextContent{
				Text: item.Text,
			})
		default:
			// For unknown types, try to preserve as text
			log.Printf("Warning: Unknown content type '%s', treating as text", item.Type)
			content = append(content, &sdk.TextContent{
				Text: item.Text,
			})
		}
	}

	return &sdk.CallToolResult{
		Content: content,
		IsError: backendResult.IsError,
	}, nil
}

// callBackendTool calls a tool on a backend server with DIFC enforcement
func (us *UnifiedServer) callBackendTool(ctx context.Context, serverID, toolName string, args interface{}) (*sdk.CallToolResult, interface{}, error) {
	// Note: Session validation happens at the tool registration level via closures
	// The closure captures the request and validates before calling this method
	log.Printf("Calling tool on %s: %s with DIFC enforcement", serverID, toolName)

	// **Phase 0: Extract agent ID and get/create agent labels**
	agentID := guard.GetAgentIDFromContext(ctx)
	agentLabels := us.agentRegistry.GetOrCreate(agentID)
	log.Printf("[DIFC] Agent %s | Secrecy: %v | Integrity: %v",
		agentID, agentLabels.GetSecrecyTags(), agentLabels.GetIntegrityTags())

	// Get guard for this backend
	g := us.guardRegistry.Get(serverID)

	// Create backend caller for the guard
	backendCaller := &guardBackendCaller{
		server:   us,
		serverID: serverID,
		ctx:      ctx,
	}

	// **Phase 1: Guard labels the resource**
	resource, operation, err := g.LabelResource(ctx, toolName, args, backendCaller, us.capabilities)
	if err != nil {
		log.Printf("[DIFC] Guard labeling failed: %v", err)
		return newErrorCallToolResult(fmt.Errorf("guard labeling failed: %w", err))
	}

	log.Printf("[DIFC] Resource: %s | Operation: %s | Secrecy: %v | Integrity: %v",
		resource.Description, operation, resource.Secrecy.Label.GetTags(), resource.Integrity.Label.GetTags())

	// **Phase 2: Reference Monitor performs coarse-grained access check**
	isWrite := (operation == difc.OperationWrite || operation == difc.OperationReadWrite)
	result := us.evaluator.Evaluate(agentLabels.Secrecy, agentLabels.Integrity, resource, operation)

	if !result.IsAllowed() {
		// Access denied - log and return detailed error
		log.Printf("[DIFC] Access DENIED for agent %s to %s: %s", agentID, resource.Description, result.Reason)
		detailedErr := difc.FormatViolationError(result, agentLabels.Secrecy, agentLabels.Integrity, resource)
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: detailedErr.Error(),
				},
			},
			IsError: true,
		}, nil, detailedErr
	}

	log.Printf("[DIFC] Access ALLOWED for agent %s to %s", agentID, resource.Description)

	// **Phase 3: Execute the backend call**
	// Get or launch backend connection (use session-aware connection for stateful backends)
	sessionID := us.getSessionID(ctx)
	conn, err := launcher.GetOrLaunchForSession(us.launcher, serverID, sessionID)
	if err != nil {
		return newErrorCallToolResult(fmt.Errorf("failed to connect: %w", err))
	}

	response, err := conn.SendRequestWithServerID(ctx, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}, serverID)
	if err != nil {
		return newErrorCallToolResult(err)
	}

	// Check if the backend returned an error
	if response.Error != nil {
		return newErrorCallToolResult(fmt.Errorf("backend error: code=%d, message=%s", response.Error.Code, response.Error.Message))
	}

	// Parse the backend result
	var backendResult interface{}
	if err := json.Unmarshal(response.Result, &backendResult); err != nil {
		return newErrorCallToolResult(fmt.Errorf("failed to parse result: %w", err))
	}

	// **Phase 4: Guard labels the response data (for fine-grained filtering)**
	labeledData, err := g.LabelResponse(ctx, toolName, backendResult, backendCaller, us.capabilities)
	if err != nil {
		log.Printf("[DIFC] Response labeling failed: %v", err)
		return newErrorCallToolResult(fmt.Errorf("response labeling failed: %w", err))
	}

	// **Phase 5: Reference Monitor performs fine-grained filtering (if applicable)**
	var finalResult interface{}
	if labeledData != nil {
		// Guard provided fine-grained labels - check if it's a collection
		if collection, ok := labeledData.(*difc.CollectionLabeledData); ok {
			// Filter collection based on agent labels
			filtered := us.evaluator.FilterCollection(agentLabels.Secrecy, agentLabels.Integrity, collection, operation)

			log.Printf("[DIFC] Filtered collection: %d/%d items accessible",
				filtered.GetAccessibleCount(), filtered.TotalCount)

			if filtered.GetFilteredCount() > 0 {
				log.Printf("[DIFC] Filtered out %d items due to DIFC policy", filtered.GetFilteredCount())
			}

			// Convert filtered data to result
			finalResult, err = filtered.ToResult()
			if err != nil {
				return newErrorCallToolResult(fmt.Errorf("failed to convert filtered data: %w", err))
			}
		} else {
			// Simple labeled data - already passed coarse-grained check
			finalResult, err = labeledData.ToResult()
			if err != nil {
				return newErrorCallToolResult(fmt.Errorf("failed to convert labeled data: %w", err))
			}
		}

		// **Phase 6: Accumulate labels from this operation (for reads)**
		if !isWrite {
			overall := labeledData.Overall()
			agentLabels.AccumulateFromRead(overall)
			log.Printf("[DIFC] Agent %s accumulated labels | Secrecy: %v | Integrity: %v",
				agentID, agentLabels.GetSecrecyTags(), agentLabels.GetIntegrityTags())
		}
	} else {
		// No fine-grained labeling - use original backend result
		finalResult = backendResult

		// **Phase 6: Accumulate labels from resource (for reads)**
		if !isWrite {
			agentLabels.AccumulateFromRead(resource)
			log.Printf("[DIFC] Agent %s accumulated labels | Secrecy: %v | Integrity: %v",
				agentID, agentLabels.GetSecrecyTags(), agentLabels.GetIntegrityTags())
		}
	}

	// Convert finalResult to SDK CallToolResult format
	callResult, err := convertToCallToolResult(finalResult)
	if err != nil {
		return newErrorCallToolResult(fmt.Errorf("failed to convert result: %w", err))
	}

	return callResult, finalResult, nil
}

// parseToolArguments extracts arguments from CallToolRequest
func parseToolArguments(req *sdk.CallToolRequest) (map[string]interface{}, error) {
	// The CallToolRequest.Params.Arguments is already a map[string]interface{}
	// We just need to validate it's present
	toolArgs, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
		// If it's not already a map, try to unmarshal it
		argsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
		}
		if err := json.Unmarshal(argsBytes, &toolArgs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}
	}
	return toolArgs, nil
}

// newErrorCallToolResult creates a standard error CallToolResult
// This helper reduces code duplication for error returns following the pattern:
// return &sdk.CallToolResult{IsError: true}, nil, err
func newErrorCallToolResult(err error) (*sdk.CallToolResult, interface{}, error) {
	return &sdk.CallToolResult{IsError: true}, nil, err
}

// GetToolsForBackend returns tools for a specific backend with prefix stripped
func (us *UnifiedServer) GetToolsForBackend(backendID string) []ToolInfo {
	us.toolsMu.RLock()
	defer us.toolsMu.RUnlock()

	prefix := backendID + "___"
	filtered := make([]ToolInfo, 0)

	for _, tool := range us.tools {
		if tool.BackendID == backendID {
			// Create a copy with the prefix stripped from the name
			filteredTool := *tool
			filteredTool.Name = tool.Name[len(prefix):] // Strip prefix
			filtered = append(filtered, filteredTool)
		}
	}

	return filtered
}

// GetToolHandler returns the handler for a specific backend tool
// This allows routed mode to reuse the unified server's tool handlers
func (us *UnifiedServer) GetToolHandler(backendID string, toolName string) func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error) {
	us.toolsMu.RLock()
	defer us.toolsMu.RUnlock()

	prefixedName := backendID + "___" + toolName
	if toolInfo, ok := us.tools[prefixedName]; ok {
		return toolInfo.Handler
	}
	return nil
}
