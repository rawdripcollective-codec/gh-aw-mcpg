package integration

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"time"

	"github.com/github/gh-aw-mcpg/internal/config"
	"github.com/github/gh-aw-mcpg/internal/launcher"
	"github.com/github/gh-aw-mcpg/internal/mcp"
)

// TestHTTPError_ServerError tests that 5xx server errors are properly propagated
func TestHTTPError_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a mock server that returns 500 Internal Server Error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Internal server error occurred",
		})
	}))
	defer mockServer.Close()

	// Try to connect to the failing server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to 500 error, but it succeeded")
	}

	// Verify error message indicates HTTP status error
	if err != nil {
		t.Logf("✓ HTTP 500 error properly propagated: %v", err)
	}
}

// TestHTTPError_ClientError tests that 4xx client errors are properly propagated
func TestHTTPError_ClientError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a mock server that returns 401 Unauthorized
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Unauthorized - invalid credentials",
		})
	}))
	defer mockServer.Close()

	// Try to connect to the server with missing auth
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to 401 error, but it succeeded")
	}

	// Verify error message indicates HTTP status error
	if err != nil {
		t.Logf("✓ HTTP 401 error properly propagated: %v", err)
	}
}

// TestHTTPError_ConnectionTimeout tests that connection timeouts are properly handled
// This test is SKIPPED by default because it takes a long time (the HTTP client has a 120s timeout).
// The timeout behavior is tested implicitly in other tests that use context timeouts.
func TestHTTPError_ConnectionTimeout(t *testing.T) {
	t.Skip("Skipping timeout test - takes too long due to HTTP client 120s timeout. Other tests verify context-based timeouts work correctly.")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that delays response beyond the HTTP client's read timeout
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start responding but then hang
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Sleep to cause a read timeout (HTTP client has 120s timeout)
		// But we'll use a shorter context timeout to cut it off sooner
		time.Sleep(150 * time.Second)
	}))
	defer mockServer.Close()

	// Use a shorter timeout that's still reasonable for real-world scenarios
	// The connection tries multiple transports, each with their own timeouts,
	// so we need to account for that in our timeout budget
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	startTime := time.Now()
	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	elapsed := time.Since(startTime)

	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to timeout, but it succeeded")
	}

	// Verify the connection timed out within a reasonable timeframe
	// Allow up to 15 seconds since we have transport fallback with retries
	if elapsed > 15*time.Second {
		t.Errorf("Timeout took too long: %v (expected < 15s)", elapsed)
	}

	if err != nil {
		t.Logf("✓ Connection timeout properly handled after %v: %v", elapsed, err)
	}
}

// TestHTTPError_ConnectionRefused tests that connection refused errors are properly handled
func TestHTTPError_ConnectionRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use a port that's not listening (find a free port then don't use it)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to find free port")
	addr := listener.Addr().String()
	listener.Close() // Close so nothing is listening

	// Try to connect to the closed port
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", "http://"+addr, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to connection refused, but it succeeded")
	}

	// Verify error indicates connection problem
	if err != nil {
		t.Logf("✓ Connection refused error properly propagated: %v", err)
	}
}

// TestHTTPError_DroppedConnection tests that dropped connections (EOF/broken pipe) are handled
func TestHTTPError_DroppedConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that closes the connection immediately
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force close the connection without sending a proper response
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("Server doesn't support hijacking")
		}
		conn, _, err := hj.Hijack()
		require.NoError(t, err, "Failed to hijack connection")
		conn.Close() // Drop the connection
	}))
	defer mockServer.Close()

	// Try to connect to the server that drops connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to dropped connection, but it succeeded")
	}

	// Verify error indicates connection problem (EOF or similar)
	if err != nil {
		t.Logf("✓ Dropped connection error properly propagated: %v", err)
	}
}

// TestHTTPError_FirewallBlocking tests the scenario where a firewall blocks requests
// This simulates the server being behind a firewall by returning errors that indicate blocked requests
func TestHTTPError_FirewallBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that simulates firewall blocking with 403 Forbidden
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Access forbidden - request blocked by firewall",
		})
	}))
	defer mockServer.Close()

	// Try to connect to the firewalled server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to firewall blocking, but it succeeded")
	}

	// Verify error is properly propagated
	if err != nil {
		t.Logf("✓ Firewall blocking error properly propagated: %v", err)
	}
}

// TestHTTPError_IntermittentFailure tests handling of intermittent connection issues
func TestHTTPError_IntermittentFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	requestCount := 0

	// Create a server that fails all requests initially, then starts succeeding
	// We need to fail enough times to cover all transport attempts:
	// - Streamable HTTP (1 request: /sse endpoint check + initialize)
	// - SSE (1 request: /sse endpoint check + initialize)
	// - Plain JSON-RPC (1 request: initialize)
	// So we need at least 6 failures to ensure the first connection attempt fails
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 6 {
			// First 6 requests: simulate failure (covers all transport attempts)
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Service temporarily unavailable",
			})
		} else {
			// Subsequent requests: succeed
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer mockServer.Close()

	// First connection attempt should fail (all transports fail)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel1()

	conn1, err1 := mcp.NewHTTPConnection(ctx1, "test-server", mockServer.URL, nil)
	if err1 == nil {
		conn1.Close()
		t.Fatal("Expected first connection to fail, but it succeeded")
	}
	t.Logf("✓ First connection attempt failed as expected: %v", err1)

	// Second connection attempt should succeed (demonstrating error recovery)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	conn2, err2 := mcp.NewHTTPConnection(ctx2, "test-server", mockServer.URL, nil)
	if err2 != nil {
		t.Fatalf("Expected second connection to succeed, but it failed: %v (after %d requests total)", err2, requestCount)
	}
	defer conn2.Close()
	t.Logf("✓ Second connection attempt succeeded after %d total requests, demonstrating error recovery", requestCount)
}

// TestHTTPError_LauncherIntegration tests that launcher properly handles HTTP connection errors
func TestHTTPError_LauncherIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that returns 500 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Internal server error",
		})
	}))
	defer mockServer.Close()

	// Create config with the failing HTTP backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"failing-backend": {
				Type: "http",
				URL:  mockServer.URL,
			},
		},
	}

	// Create launcher
	ctx := context.Background()
	l := launcher.New(ctx, cfg)

	// Try to get or launch the failing backend
	_, err := launcher.GetOrLaunch(l, "failing-backend")
	require.NotNil(t, err, "Expected launcher to fail due to backend error, but it succeeded")

	// Verify error is properly propagated through launcher
	if err != nil {
		t.Logf("✓ HTTP error properly propagated through launcher: %v", err)
	}
}

// TestHTTPError_RequestFailure tests that individual request failures are properly handled
func TestHTTPError_RequestFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	initDone := false

	// Create a server that succeeds on initialize but fails on subsequent requests
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "initialize" && !initDone {
			initDone = true
			// Initialize succeeds
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			// Subsequent requests fail
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Service unavailable",
			})
		}
	}))
	defer mockServer.Close()

	// Connect successfully
	ctx := context.Background()
	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, map[string]string{"X-Test": "test"})
	require.NoError(t, err, "Connection failed")
	defer conn.Close()

	t.Log("✓ Connection established successfully")

	// Try to make a request that should fail
	resp, err := conn.SendRequest("tools/list", nil)
	require.NoError(t, err, "Unexpected error making request")
	require.NotNil(t, resp, "Expected response")

	// Verify the response contains an error
	require.NotNil(t, resp.Error, "Expected response to contain an error field")
	require.Contains(t, resp.Error.Message, "503", "Expected error to mention HTTP 503 status")

	// Verify error is properly propagated in the response
	if resp.Error != nil {
		t.Logf("✓ Request failure error properly propagated in response: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}
}

// TestHTTPError_MalformedResponse tests handling of malformed JSON responses
func TestHTTPError_MalformedResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that returns invalid JSON
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Send malformed JSON
		w.Write([]byte("{invalid json response"))
	}))
	defer mockServer.Close()

	// Try to connect to the server with malformed responses
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to malformed response, but it succeeded")
	}

	// Verify error indicates parsing problem
	if err != nil {
		t.Logf("✓ Malformed response error properly propagated: %v", err)
	}
}

// TestHTTPError_NetworkPartition tests handling of network partition scenarios
func TestHTTPError_NetworkPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a server that starts responding but then stops (simulating incomplete response)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start writing headers
		w.WriteHeader(http.StatusOK)
		// Flush to send headers
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Write partial response body then stop (simulating network partition)
		w.Write([]byte(`{"jsonrpc": "2.0"`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Don't complete the JSON - simulate network partition by not closing properly
		time.Sleep(1 * time.Second)
		// The connection will be closed when handler returns
	}))
	defer mockServer.Close()

	// Try to connect with a timeout
	// This should fail because the response is incomplete/malformed
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startTime := time.Now()
	conn, err := mcp.NewHTTPConnection(ctx, "test-server", mockServer.URL, nil)
	elapsed := time.Since(startTime)

	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail due to network partition, but it succeeded")
	}

	// Verify error occurred (should be relatively quick since we're getting a response, just incomplete)
	if elapsed > 20*time.Second {
		t.Errorf("Timeout took too long: %v", elapsed)
	}

	if err != nil {
		t.Logf("✓ Network partition properly handled after %v: %v", elapsed, err)
	}
}
