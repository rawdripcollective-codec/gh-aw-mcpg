package server

import (
	"encoding/json"
	"net/http"

	"github.com/github/gh-aw-mcpg/internal/logger"
)

var logHealth = logger.New("server:health")

// HealthResponse represents the JSON structure for the /health endpoint response
// as defined in MCP Gateway Specification section 8.1.1
type HealthResponse struct {
	Status         string                  `json:"status"`         // "healthy" or "unhealthy"
	SpecVersion    string                  `json:"specVersion"`    // MCP Gateway Specification version
	GatewayVersion string                  `json:"gatewayVersion"` // Gateway implementation version
	Servers        map[string]ServerStatus `json:"servers"`        // Map of server names to their health status
}

// BuildHealthResponse constructs a HealthResponse from the unified server's status
func BuildHealthResponse(unifiedServer *UnifiedServer) HealthResponse {
	logHealth.Print("Building health response")

	// Get server status
	serverStatus := unifiedServer.GetServerStatus()
	logHealth.Printf("Retrieved status for %d servers", len(serverStatus))

	// Determine overall health based on server status
	overallStatus := "healthy"
	for serverName, status := range serverStatus {
		if status.Status == "error" {
			logHealth.Printf("Server error detected: server=%s, marking overall status as unhealthy", serverName)
			overallStatus = "unhealthy"
			break
		}
	}
	logHealth.Printf("Overall health status determined: %s", overallStatus)

	return HealthResponse{
		Status:         overallStatus,
		SpecVersion:    MCPGatewaySpecVersion,
		GatewayVersion: gatewayVersion,
		Servers:        serverStatus,
	}
}

// HandleHealth returns an http.HandlerFunc that handles the /health endpoint
// This function is used by both routed and unified modes to ensure consistent behavior
func HandleHealth(unifiedServer *UnifiedServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logHealth.Printf("Health check request: method=%s, remote=%s", r.Method, r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := BuildHealthResponse(unifiedServer)
		logHealth.Printf("Health response: status=%s, servers=%d", response.Status, len(response.Servers))
		json.NewEncoder(w).Encode(response)
	}
}
