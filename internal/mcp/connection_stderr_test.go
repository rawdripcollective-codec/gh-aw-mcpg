package mcp

import (
	"testing"
)

// TestConnection_MultipleServersStderrLogging documents the expected behavior for serverID in stderr logs
func TestConnection_MultipleServersStderrLogging(t *testing.T) {
	// This test documents the expected behavior of stderr logging with serverID.
	//
	// Before this change, stderr from parallel backend servers was interleaved without attribution:
	//
	// mcp:connection [stderr] ✗ failed to run status command: exit status 1 +357ms
	// mcp:connection [stderr] Output: unknown command "aw" for "gh" +779µs
	// mcp:connection [stderr] Did you mean this? +697µs
	// mcp:connection [stderr] co +982µs
	// mcp:connection [stderr] pr +1ms
	//
	// After this change, stderr logs include the serverID for clear attribution:
	//
	// mcp:connection [server1 stderr] ✗ failed to run status command: exit status 1 +357ms
	// mcp:connection [server2 stderr] Output: unknown command "aw" for "gh" +779µs
	// mcp:connection [server1 stderr] Did you mean this? +697µs
	// mcp:connection [server2 stderr] co +982µs
	// mcp:connection [server1 stderr] pr +1ms
	//
	// This makes it clear which server each log line is from when multiple backend servers
	// run in parallel.
	//
	// The serverID is passed through:
	// 1. Launcher has serverID when calling NewConnection or NewHTTPConnection
	// 2. Connection struct stores serverID field
	// 3. Stderr streaming goroutine includes serverID: logConn.Printf("[%s stderr] %s", serverID, line)

	t.Log("This test documents the expected behavior for multiple servers")
	t.Log("With the serverID in stderr logs, parallel server logs are now distinguishable")
	t.Log("See internal/mcp/connection.go:202 for the implementation")
}
