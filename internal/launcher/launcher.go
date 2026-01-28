package launcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
	"github.com/githubnext/gh-aw-mcpg/internal/tty"
)

var logLauncher = logger.New("launcher:launcher")

// connectionResult is used to return the result of a connection attempt from a goroutine
type connectionResult struct {
	conn *mcp.Connection
	err  error
}

// Launcher manages backend MCP server connections
type Launcher struct {
	ctx                context.Context
	config             *config.Config
	connections        map[string]*mcp.Connection // Single connections per backend (stateless/HTTP)
	sessionPool        *SessionConnectionPool     // Session-aware connections (stateful/stdio)
	mu                 sync.RWMutex
	runningInContainer bool
	startupTimeout     time.Duration // Timeout for backend server startup
}

// New creates a new Launcher
func New(ctx context.Context, cfg *config.Config) *Launcher {
	logLauncher.Printf("Creating new launcher with %d configured servers", len(cfg.Servers))

	inContainer := tty.IsRunningInContainer()
	if inContainer {
		log.Println("[LAUNCHER] Detected running inside a container")
	}

	// Get startup timeout from config, default to config.DefaultStartupTimeout seconds
	startupTimeout := time.Duration(config.DefaultStartupTimeout) * time.Second
	if cfg.Gateway != nil && cfg.Gateway.StartupTimeout > 0 {
		startupTimeout = time.Duration(cfg.Gateway.StartupTimeout) * time.Second
		logLauncher.Printf("Using configured startup timeout: %v", startupTimeout)
	} else {
		logLauncher.Printf("Using default startup timeout: %v", startupTimeout)
	}

	return &Launcher{
		ctx:                ctx,
		config:             cfg,
		connections:        make(map[string]*mcp.Connection),
		sessionPool:        NewSessionConnectionPool(ctx),
		runningInContainer: inContainer,
		startupTimeout:     startupTimeout,
	}
}

// GetOrLaunch returns an existing connection or launches a new one
func GetOrLaunch(l *Launcher, serverID string) (*mcp.Connection, error) {
	logger.LogDebug("backend", "GetOrLaunch called for server: %s", serverID)
	logLauncher.Printf("GetOrLaunch called: serverID=%s", serverID)

	// Check if already exists
	l.mu.RLock()
	if conn, ok := l.connections[serverID]; ok {
		l.mu.RUnlock()
		logger.LogDebug("backend", "Reusing existing backend connection: %s", serverID)
		logLauncher.Printf("Reusing existing connection: serverID=%s", serverID)
		return conn, nil
	}
	l.mu.RUnlock()

	logLauncher.Printf("Connection not found in cache, launching new: serverID=%s", serverID)

	// Launch new connection
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, ok := l.connections[serverID]; ok {
		logger.LogDebug("backend", "Backend connection created by another goroutine: %s", serverID)
		logLauncher.Printf("Connection created by another goroutine: serverID=%s", serverID)
		return conn, nil
	}

	// Get server config
	serverCfg, ok := l.config.Servers[serverID]
	if !ok {
		logger.LogError("backend", "Backend server not found in config: %s", serverID)
		logLauncher.Printf("Server not found in config: serverID=%s", serverID)
		return nil, fmt.Errorf("server '%s' not found in config", serverID)
	}
	logLauncher.Printf("Retrieved server config: serverID=%s, type=%s", serverID, serverCfg.Type)

	// Handle HTTP backends differently
	if serverCfg.Type == "http" {
		logger.LogInfo("backend", "Configuring HTTP MCP backend: %s, url=%s", serverID, serverCfg.URL)
		log.Printf("[LAUNCHER] Configuring HTTP MCP backend: %s", serverID)
		log.Printf("[LAUNCHER] URL: %s", serverCfg.URL)
		logLauncher.Printf("HTTP backend: serverID=%s, url=%s", serverID, serverCfg.URL)

		// Create an HTTP connection
		conn, err := mcp.NewHTTPConnection(l.ctx, serverCfg.URL, serverCfg.Headers)
		if err != nil {
			logger.LogError("backend", "Failed to create HTTP connection: %s, error=%v", serverID, err)
			log.Printf("[LAUNCHER] ❌ FAILED to create HTTP connection for '%s'", serverID)
			log.Printf("[LAUNCHER] Error: %v", err)
			return nil, fmt.Errorf("failed to create HTTP connection: %w", err)
		}

		logger.LogInfo("backend", "Successfully configured HTTP MCP backend: %s", serverID)
		log.Printf("[LAUNCHER] Successfully configured HTTP backend: %s", serverID)
		logLauncher.Printf("HTTP connection configured: serverID=%s", serverID)

		l.connections[serverID] = conn
		return conn, nil
	}

	// stdio backends from this point
	// Warn if using direct command in a container
	isDirectCommand := serverCfg.Command != "docker"
	if l.runningInContainer && isDirectCommand {
		logger.LogWarn("backend", "Server '%s' uses direct command execution inside a container (command: %s)", serverID, serverCfg.Command)
		log.Printf("[LAUNCHER] ⚠️  WARNING: Server '%s' uses direct command execution inside a container", serverID)
		log.Printf("[LAUNCHER] ⚠️  Security Notice: Command '%s' will execute with the same privileges as the gateway", serverCfg.Command)
		log.Printf("[LAUNCHER] ⚠️  Consider using 'container' field instead for better isolation")
	}

	// Log the command being executed
	logger.LogInfo("backend", "Launching MCP backend server: %s, command=%s, args=%v", serverID, serverCfg.Command, sanitize.SanitizeArgs(serverCfg.Args))
	log.Printf("[LAUNCHER] Starting MCP server: %s", serverID)
	log.Printf("[LAUNCHER] Command: %s", serverCfg.Command)
	log.Printf("[LAUNCHER] Args: %v", sanitize.SanitizeArgs(serverCfg.Args))
	logLauncher.Printf("Launching new server: serverID=%s, command=%s, inContainer=%v, isDirectCommand=%v",
		serverID, serverCfg.Command, l.runningInContainer, isDirectCommand)

	// Check for environment variable passthrough (only check args after -e flags)
	for i := 0; i < len(serverCfg.Args); i++ {
		arg := serverCfg.Args[i]
		// If this arg is "-e", check the next argument
		if arg == "-e" && i+1 < len(serverCfg.Args) {
			nextArg := serverCfg.Args[i+1]
			// Check if it's a passthrough (no = sign) vs explicit value (has = sign)
			if !strings.Contains(nextArg, "=") {
				// This is a passthrough variable, check if it exists in our environment
				if val := os.Getenv(nextArg); val != "" {
					displayVal := val
					if len(val) > 10 {
						displayVal = val[:10] + "..."
					}
					log.Printf("[LAUNCHER] ✓ Env passthrough: %s=%s (from MCPG process)", nextArg, displayVal)
				} else {
					log.Printf("[LAUNCHER] ✗ WARNING: Env passthrough for %s requested but NOT FOUND in MCPG process", nextArg)
				}
			}
			i++ // Skip the next arg since we just processed it
		}
	}

	if len(serverCfg.Env) > 0 {
		log.Printf("[LAUNCHER] Additional env vars: %v", sanitize.TruncateSecretMap(serverCfg.Env))
	}

	log.Printf("[LAUNCHER] Starting server with %v timeout", l.startupTimeout)
	logLauncher.Printf("Starting server with timeout: serverID=%s, timeout=%v", serverID, l.startupTimeout)

	// Create a buffered channel to receive connection result
	// Buffer size of 1 prevents goroutine leak if timeout occurs before connection completes
	resultChan := make(chan connectionResult, 1)

	logLauncher.Printf("Starting connection goroutine: serverID=%s", serverID)
	// Launch connection in a goroutine
	go func() {
		conn, err := mcp.NewConnection(l.ctx, serverCfg.Command, serverCfg.Args, serverCfg.Env)
		resultChan <- connectionResult{conn, err}
	}()

	// Wait for connection with timeout
	select {
	case result := <-resultChan:
		conn, err := result.conn, result.err
		if err != nil {
			// Enhanced error logging for command-based servers
			logger.LogError("backend", "Failed to launch MCP backend server: %s, error=%v", serverID, err)
			log.Printf("[LAUNCHER] ❌ FAILED to launch server '%s'", serverID)
			log.Printf("[LAUNCHER] Error: %v", err)
			log.Printf("[LAUNCHER] Debug Information:")
			log.Printf("[LAUNCHER]   - Command: %s", serverCfg.Command)
			log.Printf("[LAUNCHER]   - Args: %v", serverCfg.Args)
			log.Printf("[LAUNCHER]   - Env vars: %v", sanitize.TruncateSecretMap(serverCfg.Env))
			log.Printf("[LAUNCHER]   - Running in container: %v", l.runningInContainer)
			log.Printf("[LAUNCHER]   - Is direct command: %v", isDirectCommand)
			log.Printf("[LAUNCHER]   - Startup timeout: %v", l.startupTimeout)

			if isDirectCommand && l.runningInContainer {
				log.Printf("[LAUNCHER] ⚠️  Possible causes:")
				log.Printf("[LAUNCHER]   - Command '%s' may not be installed in the gateway container", serverCfg.Command)
				log.Printf("[LAUNCHER]   - Consider using 'container' config instead of 'command'")
				log.Printf("[LAUNCHER]   - Or add '%s' to the gateway's Dockerfile", serverCfg.Command)
			} else if isDirectCommand {
				log.Printf("[LAUNCHER] ⚠️  Possible causes:")
				log.Printf("[LAUNCHER]   - Command '%s' may not be in PATH", serverCfg.Command)
				log.Printf("[LAUNCHER]   - Check if '%s' is installed: which %s", serverCfg.Command, serverCfg.Command)
				log.Printf("[LAUNCHER]   - Verify file permissions and execute bit")
			}

			return nil, fmt.Errorf("failed to create connection: %w", err)
		}

		logger.LogInfo("backend", "Successfully launched MCP backend server: %s", serverID)
		log.Printf("[LAUNCHER] Successfully launched: %s", serverID)
		logLauncher.Printf("Connection established: serverID=%s", serverID)

		l.connections[serverID] = conn
		return conn, nil

	case <-time.After(l.startupTimeout):
		// Timeout occurred
		logger.LogError("backend", "MCP backend server startup timeout: %s, timeout=%v", serverID, l.startupTimeout)
		log.Printf("[LAUNCHER] ❌ Server startup timed out after %v", l.startupTimeout)
		log.Printf("[LAUNCHER] ⚠️  The server may be hanging or taking too long to initialize")
		log.Printf("[LAUNCHER] ⚠️  Consider increasing 'startupTimeout' in gateway config if server needs more time")
		logLauncher.Printf("Startup timeout occurred: serverID=%s, timeout=%v", serverID, l.startupTimeout)
		return nil, fmt.Errorf("server startup timeout after %v", l.startupTimeout)
	}
}

// GetOrLaunchForSession returns a session-aware connection or launches a new one
// This is used for stateful stdio backends that require persistent connections
func GetOrLaunchForSession(l *Launcher, serverID, sessionID string) (*mcp.Connection, error) {
	logger.LogDebug("backend", "GetOrLaunchForSession called: server=%s, session=%s", serverID, sessionID)
	logLauncher.Printf("GetOrLaunchForSession called: serverID=%s, sessionID=%s", serverID, sessionID)

	// Get server config first to determine backend type
	l.mu.RLock()
	serverCfg, ok := l.config.Servers[serverID]
	l.mu.RUnlock()

	if !ok {
		logger.LogError("backend", "Backend server not found in config: %s", serverID)
		return nil, fmt.Errorf("server '%s' not found in config", serverID)
	}

	// For HTTP backends, use the regular GetOrLaunch (stateless)
	if serverCfg.Type == "http" {
		logLauncher.Printf("HTTP backend detected, using stateless connection: serverID=%s", serverID)
		return GetOrLaunch(l, serverID)
	}

	logLauncher.Printf("Checking session pool: serverID=%s, sessionID=%s", serverID, sessionID)
	// For stdio backends, check the session pool first
	if conn, exists := l.sessionPool.Get(serverID, sessionID); exists {
		logger.LogDebug("backend", "Reusing session connection: server=%s, session=%s", serverID, sessionID)
		logLauncher.Printf("Reusing session connection: serverID=%s, sessionID=%s", serverID, sessionID)
		return conn, nil
	}

	// Need to launch new connection for this session
	logLauncher.Printf("Session connection not found, creating new: serverID=%s, sessionID=%s", serverID, sessionID)

	// Lock for launching
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring lock
	if conn, exists := l.sessionPool.Get(serverID, sessionID); exists {
		logger.LogDebug("backend", "Session connection created by another goroutine: server=%s, session=%s", serverID, sessionID)
		logLauncher.Printf("Session connection created by another goroutine: serverID=%s, sessionID=%s", serverID, sessionID)
		return conn, nil
	}

	// Warn if using direct command in a container
	isDirectCommand := serverCfg.Command != "docker"
	if l.runningInContainer && isDirectCommand {
		logger.LogWarn("backend", "Server '%s' uses direct command execution inside a container (command: %s)", serverID, serverCfg.Command)
		log.Printf("[LAUNCHER] ⚠️  WARNING: Server '%s' uses direct command execution inside a container", serverID)
		log.Printf("[LAUNCHER] ⚠️  Security Notice: Command '%s' will execute with the same privileges as the gateway", serverCfg.Command)
		log.Printf("[LAUNCHER] ⚠️  Consider using 'container' field instead for better isolation")
	}

	// Log the command being executed
	logger.LogInfo("backend", "Launching MCP backend server for session: server=%s, session=%s, command=%s, args=%v", serverID, sessionID, serverCfg.Command, sanitize.SanitizeArgs(serverCfg.Args))
	log.Printf("[LAUNCHER] Starting MCP server for session: %s (session: %s)", serverID, sessionID)
	log.Printf("[LAUNCHER] Command: %s", serverCfg.Command)
	log.Printf("[LAUNCHER] Args: %v", sanitize.SanitizeArgs(serverCfg.Args))
	logLauncher.Printf("Launching new session server: serverID=%s, sessionID=%s, command=%s", serverID, sessionID, serverCfg.Command)

	// Check for environment variable passthrough
	for i := 0; i < len(serverCfg.Args); i++ {
		arg := serverCfg.Args[i]
		if arg == "-e" && i+1 < len(serverCfg.Args) {
			nextArg := serverCfg.Args[i+1]
			if !strings.Contains(nextArg, "=") {
				if val := os.Getenv(nextArg); val != "" {
					displayVal := val
					if len(val) > 10 {
						displayVal = val[:10] + "..."
					}
					log.Printf("[LAUNCHER] ✓ Env passthrough: %s=%s (from MCPG process)", nextArg, displayVal)
				} else {
					log.Printf("[LAUNCHER] ✗ WARNING: Env passthrough for %s requested but NOT FOUND in MCPG process", nextArg)
				}
			}
			i++
		}
	}

	if len(serverCfg.Env) > 0 {
		log.Printf("[LAUNCHER] Additional env vars: %v", sanitize.TruncateSecretMap(serverCfg.Env))
	}

	log.Printf("[LAUNCHER] Starting server for session with %v timeout", l.startupTimeout)
	logLauncher.Printf("Starting session server with timeout: serverID=%s, sessionID=%s, timeout=%v", serverID, sessionID, l.startupTimeout)

	// Create a buffered channel to receive connection result
	// Buffer size of 1 prevents goroutine leak if timeout occurs before connection completes
	resultChan := make(chan connectionResult, 1)

	// Launch connection in a goroutine
	go func() {
		conn, err := mcp.NewConnection(l.ctx, serverCfg.Command, serverCfg.Args, serverCfg.Env)
		resultChan <- connectionResult{conn, err}
	}()

	// Wait for connection with timeout
	select {
	case result := <-resultChan:
		conn, err := result.conn, result.err
		if err != nil {
			logger.LogError("backend", "Failed to launch MCP backend server for session: server=%s, session=%s, error=%v", serverID, sessionID, err)
			log.Printf("[LAUNCHER] ❌ FAILED to launch server '%s' for session '%s'", serverID, sessionID)
			log.Printf("[LAUNCHER] Error: %v", err)
			log.Printf("[LAUNCHER] Debug Information:")
			log.Printf("[LAUNCHER]   - Command: %s", serverCfg.Command)
			log.Printf("[LAUNCHER]   - Args: %v", serverCfg.Args)
			log.Printf("[LAUNCHER]   - Env vars: %v", sanitize.TruncateSecretMap(serverCfg.Env))
			log.Printf("[LAUNCHER]   - Running in container: %v", l.runningInContainer)
			log.Printf("[LAUNCHER]   - Is direct command: %v", isDirectCommand)
			log.Printf("[LAUNCHER]   - Startup timeout: %v", l.startupTimeout)

			// Record error in session pool
			l.sessionPool.RecordError(serverID, sessionID)

			return nil, fmt.Errorf("failed to create connection: %w", err)
		}

		logger.LogInfo("backend", "Successfully launched MCP backend server for session: server=%s, session=%s", serverID, sessionID)
		log.Printf("[LAUNCHER] Successfully launched: %s (session: %s)", serverID, sessionID)
		logLauncher.Printf("Session connection established: serverID=%s, sessionID=%s", serverID, sessionID)

		// Add to session pool
		l.sessionPool.Set(serverID, sessionID, conn)
		return conn, nil

	case <-time.After(l.startupTimeout):
		// Timeout occurred
		logger.LogError("backend", "MCP backend server startup timeout for session: server=%s, session=%s, timeout=%v", serverID, sessionID, l.startupTimeout)
		log.Printf("[LAUNCHER] ❌ Server startup timed out after %v", l.startupTimeout)
		log.Printf("[LAUNCHER] ⚠️  The server may be hanging or taking too long to initialize")
		log.Printf("[LAUNCHER] ⚠️  Consider increasing 'startupTimeout' in gateway config if server needs more time")
		// Record error in session pool before returning
		l.sessionPool.RecordError(serverID, sessionID)
		return nil, fmt.Errorf("server startup timeout after %v", l.startupTimeout)
	}
}

// ServerIDs returns all configured server IDs
func (l *Launcher) ServerIDs() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	ids := make([]string, 0, len(l.config.Servers))
	for id := range l.config.Servers {
		ids = append(ids, id)
	}
	logLauncher.Printf("Retrieved server IDs: count=%d, ids=%v", len(ids), ids)
	return ids
}

// Close closes all connections
func (l *Launcher) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	logLauncher.Printf("Closing launcher: connections=%d, hasSessionPool=%v", len(l.connections), l.sessionPool != nil)
	logLauncher.Printf("Closing %d connections", len(l.connections))
	for _, conn := range l.connections {
		conn.Close()
	}
	l.connections = make(map[string]*mcp.Connection)

	// Stop session pool and close all session connections
	if l.sessionPool != nil {
		logLauncher.Printf("Stopping session connection pool")
		l.sessionPool.Stop()
	}
	logLauncher.Print("Launcher closed successfully")
}
