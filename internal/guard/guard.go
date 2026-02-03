package guard

import (
	"context"

	"github.com/github/gh-aw-mcpg/internal/difc"
)

// BackendCaller provides a way for guards to make read-only calls to the backend
// to gather information needed for labeling (e.g., fetching issue author)
type BackendCaller interface {
	// CallTool makes a read-only call to the backend MCP server
	// This is used by guards to gather metadata for labeling
	CallTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)
}

// Guard handles DIFC labeling for a specific MCP server
// Guards ONLY label resources - they do NOT make access control decisions
// The Reference Monitor (in the server) uses guard-provided labels to enforce DIFC policies
type Guard interface {
	// Name returns the identifier for this guard (e.g., "github", "noop")
	Name() string

	// LabelResource determines the resource being accessed and its labels
	// This may call the backend (via BackendCaller) to gather metadata needed for labeling
	// Returns:
	//   - resource: The labeled resource (simple or nested structure for fine-grained filtering)
	//   - operation: The type of operation (Read, Write, or ReadWrite)
	//   - error: Any error that occurred during labeling
	LabelResource(ctx context.Context, toolName string, args interface{}, backend BackendCaller, caps *difc.Capabilities) (*difc.LabeledResource, difc.OperationType, error)

	// LabelResponse labels the response data after a successful backend call
	// This is used for fine-grained filtering of collections
	// Returns:
	//   - labeledData: The response data with per-item labels (if applicable)
	//   - error: Any error that occurred during labeling
	// If the guard returns nil for labeledData, the reference monitor will use the
	// resource labels from LabelResource for the entire response
	LabelResponse(ctx context.Context, toolName string, result interface{}, backend BackendCaller, caps *difc.Capabilities) (difc.LabeledData, error)
}

// RequestState represents any state that the guard needs to pass from request to response
// This is useful when the guard needs to carry information from LabelResource to LabelResponse
type RequestState interface{}
