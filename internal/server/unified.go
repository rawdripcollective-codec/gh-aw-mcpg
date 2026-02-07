package server

import (
"context"
"fmt"
"log"
"sync"
"time"

"github.com/github/gh-aw-mcpg/internal/config"
"github.com/github/gh-aw-mcpg/internal/difc"
"github.com/github/gh-aw-mcpg/internal/guard"
"github.com/github/gh-aw-mcpg/internal/launcher"
"github.com/github/gh-aw-mcpg/internal/logger"
"github.com/github/gh-aw-mcpg/internal/mcp"
"github.com/github/gh-aw-mcpg/internal/sys"
sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logUnified = logger.New("server:unified")

// MCPProtocolVersion is the MCP protocol version supported by this gateway
const MCPProtocolVersion = "2024-11-05"

// MCPGatewaySpecVersion is the MCP Gateway Specification version this implementation conforms to
const MCPGatewaySpecVersion = "1.5.0"

// Session represents a MCPG session
type Session struct {
Token     string
SessionID string
StartTime time.Time
}

// ServerStatus represents the health status of a backend server
type ServerStatus struct {
Status string `json:"status"` // "running" | "stopped" | "error"
Uptime int    `json:"uptime"` // seconds since server was launched
}

// NewSession creates a new Session with the given session ID and optional token
func NewSession(sessionID, token string) *Session {
return &Session{
Token:     token,
SessionID: sessionID,
StartTime: time.Now(),
}
}

// SessionIDContextKey is used to store MCP session ID in context
// This is re-exported from mcp package for backward compatibility
const SessionIDContextKey = mcp.SessionIDContextKey

// ToolInfo stores metadata about a registered tool
type ToolInfo struct {
Name        string
Description string
InputSchema map[string]interface{}
BackendID   string // Which backend this tool belongs to
Handler     func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error)
}

// UnifiedServer implements a unified MCP server that aggregates multiple backend servers
type UnifiedServer struct {
launcher             *launcher.Launcher
sysServer            *sys.SysServer
ctx                  context.Context
server               *sdk.Server
sessions             map[string]*Session // mcp-session-id -> Session
sessionMu            sync.RWMutex
tools                map[string]*ToolInfo // prefixed tool name -> tool info
toolsMu              sync.RWMutex
sequentialLaunch     bool   // When true, launches MCP servers sequentially during startup. Default is false (parallel launch).
payloadDir           string // Base directory for storing large payload files (segmented by session ID)
payloadSizeThreshold int    // Size threshold (in bytes) for storing payloads to disk. Payloads larger than this are stored to disk, smaller ones are returned inline.

// DIFC components
guardRegistry *guard.Registry
agentRegistry *difc.AgentRegistry
capabilities  *difc.Capabilities
evaluator     *difc.Evaluator
enableDIFC    bool // When true, DIFC enforcement and session requirement are enabled

// Shutdown state tracking
isShutdown   bool
shutdownMu   sync.RWMutex
shutdownOnce sync.Once

// Testing support - when true, skips os.Exit() call
testMode bool
}

// NewUnified creates a new unified MCP server
func NewUnified(ctx context.Context, cfg *config.Config) (*UnifiedServer, error) {
logUnified.Printf("Creating new unified server: enableDIFC=%v, sequentialLaunch=%v, servers=%d", cfg.EnableDIFC, cfg.SequentialLaunch, len(cfg.Servers))
l := launcher.New(ctx, cfg)

// Get payload directory from config, with fallback to default
payloadDir := config.DefaultPayloadDir
if cfg.Gateway != nil && cfg.Gateway.PayloadDir != "" {
payloadDir = cfg.Gateway.PayloadDir
}

// Get payload size threshold from config, with fallback to default
payloadSizeThreshold := config.DefaultPayloadSizeThreshold
if cfg.Gateway != nil && cfg.Gateway.PayloadSizeThreshold > 0 {
payloadSizeThreshold = cfg.Gateway.PayloadSizeThreshold
}
logUnified.Printf("Payload configuration: dir=%s, sizeThreshold=%d bytes (%.2f KB)",
payloadDir, payloadSizeThreshold, float64(payloadSizeThreshold)/1024)

us := &UnifiedServer{
launcher:             l,
sysServer:            sys.NewSysServer(l.ServerIDs()),
ctx:                  ctx,
sessions:             make(map[string]*Session),
tools:                make(map[string]*ToolInfo),
sequentialLaunch:     cfg.SequentialLaunch,
payloadDir:           payloadDir,
payloadSizeThreshold: payloadSizeThreshold,

// Initialize DIFC components
guardRegistry: guard.NewRegistry(),
agentRegistry: difc.NewAgentRegistry(),
capabilities:  difc.NewCapabilities(),
evaluator:     difc.NewEvaluator(),
enableDIFC:    cfg.EnableDIFC,
}

// Create MCP server
server := sdk.NewServer(&sdk.Implementation{
Name:    "awmg-unified",
Version: "1.0.0",
}, nil)

us.server = server

// Register guards for all backends
for _, serverID := range l.ServerIDs() {
us.registerGuard(serverID)
}

// Register aggregated tools from all backends
if err := us.registerAllTools(); err != nil {
return nil, fmt.Errorf("failed to register tools: %w", err)
}

logUnified.Printf("Unified server created successfully with %d tools", len(us.tools))
return us, nil
}

// Run starts the unified MCP server on the specified transport
func (us *UnifiedServer) Run(transport sdk.Transport) error {
log.Println("Starting unified MCP server...")
return us.server.Run(us.ctx, transport)
}

// GetPayloadSizeThreshold returns the configured payload size threshold (in bytes).
// Payloads larger than this threshold are stored to disk, smaller ones are returned inline.
// This getter allows other modules to access the threshold configuration.
func (us *UnifiedServer) GetPayloadSizeThreshold() int {
return us.payloadSizeThreshold
}

// GetServerIDs returns the list of backend server IDs
func (us *UnifiedServer) GetServerIDs() []string {
return us.launcher.ServerIDs()
}

// GetServerStatus returns the status of all configured backend servers
func (us *UnifiedServer) GetServerStatus() map[string]ServerStatus {
status := make(map[string]ServerStatus)

// Get all configured servers
serverIDs := us.launcher.ServerIDs()

for _, serverID := range serverIDs {
// Check if server has been launched by checking launcher connections
// For now, we'll return "running" for all configured servers
// and track uptime from when the gateway started
// This is a simple implementation - a more sophisticated version
// would track actual connection state per server
status[serverID] = ServerStatus{
Status: "running",
Uptime: 0, // Will be properly tracked when servers are actually launched
}
}

return status
}

// Close cleans up resources
func (us *UnifiedServer) Close() error {
us.launcher.Close()
return nil
}

// IsShutdown returns true if the gateway has been shut down
func (us *UnifiedServer) IsShutdown() bool {
us.shutdownMu.RLock()
defer us.shutdownMu.RUnlock()
return us.isShutdown
}

// InitiateShutdown initiates graceful shutdown and returns the number of servers terminated
// This method is idempotent - subsequent calls will return 0 servers terminated
func (us *UnifiedServer) InitiateShutdown() int {
serversTerminated := 0
us.shutdownOnce.Do(func() {
// Mark as shutdown
us.shutdownMu.Lock()
us.isShutdown = true
us.shutdownMu.Unlock()

log.Println("Initiating gateway shutdown...")
logger.LogInfo("shutdown", "Gateway shutdown initiated")

// Count servers before closing
serversTerminated = len(us.launcher.ServerIDs())

// Terminate all backend servers
log.Printf("Terminating %d backend server(s)...", serversTerminated)
logger.LogInfo("shutdown", "Terminating %d backend servers", serversTerminated)
us.launcher.Close()

log.Println("Backend servers terminated")
logger.LogInfo("shutdown", "Backend servers terminated successfully")
})
return serversTerminated
}

// RegisterTestTool registers a tool for testing purposes
// This method is used by integration tests to inject mock tools into the gateway
func (us *UnifiedServer) RegisterTestTool(name string, tool *ToolInfo) {
us.toolsMu.Lock()
defer us.toolsMu.Unlock()
us.tools[name] = tool
}

// SetTestMode enables test mode which prevents os.Exit() calls
// This should only be used in unit tests
func (us *UnifiedServer) SetTestMode(enabled bool) {
us.testMode = enabled
}

// ShouldExit returns whether the gateway should exit after shutdown
// Returns false in test mode to prevent actual process exit
func (us *UnifiedServer) ShouldExit() bool {
return !us.testMode
}

// IsDIFCEnabled returns whether DIFC is enabled
func (us *UnifiedServer) IsDIFCEnabled() bool {
return us.enableDIFC
}
