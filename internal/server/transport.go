package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logTransport = logger.New("server:transport")

// HTTPTransport wraps the SDK's HTTP transport
type HTTPTransport struct {
	Addr string
}

// Start implements sdk.Transport interface
func (t *HTTPTransport) Start(ctx context.Context) error {
	logTransport.Printf("Starting HTTP transport: addr=%s", t.Addr)
	// The SDK will handle the actual HTTP server setup
	// We just need to provide the address
	log.Printf("HTTP transport ready on %s", t.Addr)
	return nil
}

// Send implements sdk.Transport interface
func (t *HTTPTransport) Send(msg interface{}) error {
	// Messages are sent via HTTP responses, handled by SDK
	return nil
}

// Recv implements sdk.Transport interface
func (t *HTTPTransport) Recv() (interface{}, error) {
	// Messages are received via HTTP requests, handled by SDK
	return nil, nil
}

// Close implements sdk.Transport interface
func (t *HTTPTransport) Close() error {
	return nil
}

// withResponseLogging wraps an http.Handler to log response bodies
func withResponseLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lw := newResponseWriter(w)
		handler.ServeHTTP(lw, r)
		if len(lw.Body()) > 0 {
			log.Printf("[%s] %s %s - Status: %d, Response: %s", r.RemoteAddr, r.Method, r.URL.Path, lw.StatusCode(), string(lw.Body()))
		}
	})
}

// CreateHTTPServerForMCP creates an HTTP server that handles MCP over streamable HTTP transport
// If apiKey is provided, all requests except /health require authentication (spec 7.1)
func CreateHTTPServerForMCP(addr string, unifiedServer *UnifiedServer, apiKey string) *http.Server {
	logTransport.Printf("Creating HTTP server for MCP: addr=%s, auth_enabled=%v", addr, apiKey != "")
	mux := http.NewServeMux()

	// OAuth discovery endpoint - return 404 since we don't use OAuth
	mux.Handle("/mcp/.well-known/oauth-authorization-server", withResponseLogging(handleOAuthDiscovery()))

	logTransport.Print("Registering streamable HTTP handler for MCP protocol")
	// Create StreamableHTTP handler for MCP protocol (supports POST requests)
	// This is what Codex uses with transport = "streamablehttp"
	streamableHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		// With streamable HTTP, this callback fires for each new session establishment
		// Subsequent JSON-RPC messages in the same session are handled by the SDK
		// We use the Authorization header value as the session ID
		// This groups all requests from the same agent (same auth value) into one session

		// Extract and validate session ID from Authorization header
		sessionID := extractAndValidateSession(r)
		if sessionID == "" {
			// Return nil to reject the connection
			// The SDK will handle sending an appropriate error response
			return nil
		}

		logger.LogInfo("client", "MCP connection established, remote=%s, method=%s, path=%s, session=%s", r.RemoteAddr, r.Method, r.URL.Path, sessionID)
		log.Printf("=== NEW STREAMABLE HTTP CONNECTION ===")
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		log.Printf("Authorization (Session ID): %s", sessionID)

		log.Printf("DEBUG: About to check request body, Method=%s, Body!=nil: %v", r.Method, r.Body != nil)

		// Log request body for debugging (typically the 'initialize' request)
		logHTTPRequestBody(r, sessionID, "")

		// Store session ID in request context
		// This context will be passed to all tool handlers for this connection
		*r = *injectSessionContext(r, sessionID, "")
		log.Printf("✓ Injected session ID into context")
		log.Printf("==========================\n")

		return unifiedServer.server
	}, &sdk.StreamableHTTPOptions{
		Stateless:      false,                                      // Support stateful sessions
		Logger:         logger.NewSlogLoggerWithHandler(logTransport), // Integrate SDK logging with project logger
		SessionTimeout: 30 * time.Minute,                            // Prevent resource leaks from idle connections
	})

	// Wrap SDK handler with detailed logging for JSON-RPC translation debugging
	loggedHandler := WithSDKLogging(streamableHandler, "unified")

	// Apply shutdown check middleware (spec 5.1.3)
	// This must come before auth to ensure shutdown takes precedence
	shutdownHandler := rejectIfShutdown(unifiedServer, loggedHandler, "server:transport")

	// Apply auth middleware if API key is configured (spec 7.1)
	finalHandler := shutdownHandler
	if apiKey != "" {
		finalHandler = authMiddleware(apiKey, shutdownHandler.ServeHTTP)
	}

	// Mount handler at /mcp endpoint (logging is done in the callback above)
	mux.Handle("/mcp/", finalHandler)
	mux.Handle("/mcp", finalHandler)

	// Health check (spec 8.1.1)
	healthHandler := HandleHealth(unifiedServer)
	mux.Handle("/health", withResponseLogging(healthHandler))

	// Close endpoint for graceful shutdown (spec 5.1.3)
	closeHandler := handleClose(unifiedServer)

	// Apply auth middleware if API key is configured (spec 7.1)
	finalCloseHandler := closeHandler
	if apiKey != "" {
		finalCloseHandler = authMiddleware(apiKey, closeHandler.ServeHTTP)
	}
	mux.Handle("/close", withResponseLogging(finalCloseHandler))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
