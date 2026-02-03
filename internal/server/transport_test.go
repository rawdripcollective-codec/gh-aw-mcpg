package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/gh-aw-mcpg/internal/config"
)

// TestLoggingResponseWriter_WriteHeader tests the WriteHeader method
func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		wantStatusCode int
	}{
		{
			name:           "StatusOK",
			statusCode:     http.StatusOK,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "StatusNotFound",
			statusCode:     http.StatusNotFound,
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "StatusInternalServerError",
			statusCode:     http.StatusInternalServerError,
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "StatusUnauthorized",
			statusCode:     http.StatusUnauthorized,
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			lw := newResponseWriter(w)

			// Write header
			lw.WriteHeader(tt.statusCode)

			// Verify status code is captured
			assert.Equal(t, tt.wantStatusCode, lw.StatusCode(), "Status code should be captured")
		})
	}
}

// TestLoggingResponseWriter_Write tests the Write method
func TestLoggingResponseWriter_Write(t *testing.T) {
	tests := []struct {
		name        string
		writes      [][]byte
		wantBody    []byte
		wantWritten int
	}{
		{
			name:        "SingleWrite",
			writes:      [][]byte{[]byte("hello")},
			wantBody:    []byte("hello"),
			wantWritten: 5,
		},
		{
			name:        "MultipleWrites",
			writes:      [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			wantBody:    []byte("hello world"),
			wantWritten: 11,
		},
		{
			name:        "EmptyWrite",
			writes:      [][]byte{[]byte("")},
			wantBody:    []byte(""),
			wantWritten: 0,
		},
		{
			name:        "JSONWrite",
			writes:      [][]byte{[]byte(`{"status":"ok"}`)},
			wantBody:    []byte(`{"status":"ok"}`),
			wantWritten: 15,
		},
		{
			name:        "LargeWrite",
			writes:      [][]byte{bytes.Repeat([]byte("a"), 1000)},
			wantBody:    bytes.Repeat([]byte("a"), 1000),
			wantWritten: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			lw := newResponseWriter(w)

			totalWritten := 0
			for _, data := range tt.writes {
				n, err := lw.Write(data)
				require.NoError(t, err, "Write should not return error")
				totalWritten += n
			}

			// Verify total bytes written
			assert.Equal(t, tt.wantWritten, totalWritten, "Total bytes written should match")

			// Verify body is captured (use Len for empty check to handle nil vs empty slice)
			if len(tt.wantBody) == 0 {
				assert.Empty(t, lw.Body(), "Body should be empty")
				assert.Empty(t, w.Body.Bytes(), "Underlying writer body should be empty")
			} else {
				assert.Equal(t, tt.wantBody, lw.Body(), "Body should be captured correctly")
				assert.Equal(t, tt.wantBody, w.Body.Bytes(), "Body should be written to underlying writer")
			}
		})
	}
}

// TestLoggingResponseWriter_DefaultStatusCode tests that default status code is 200
func TestLoggingResponseWriter_DefaultStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	lw := newResponseWriter(w)

	// Write without explicit WriteHeader
	lw.Write([]byte("test"))

	// Default status code should be 200
	assert.Equal(t, http.StatusOK, lw.StatusCode(), "Default status code should be 200")
}

// TestWithResponseLogging tests the withResponseLogging middleware
func TestWithResponseLogging(t *testing.T) {
	tests := []struct {
		name           string
		handlerFunc    http.HandlerFunc
		method         string
		path           string
		wantStatusCode int
		wantBody       string
	}{
		{
			name: "SuccessfulRequest",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			method:         "GET",
			path:           "/test",
			wantStatusCode: http.StatusOK,
			wantBody:       "success",
		},
		{
			name: "ErrorRequest",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			method:         "GET",
			path:           "/missing",
			wantStatusCode: http.StatusNotFound,
			wantBody:       "not found\n",
		},
		{
			name: "JSONResponse",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			},
			method:         "POST",
			path:           "/api/test",
			wantStatusCode: http.StatusOK,
			wantBody:       `{"status":"ok"}` + "\n",
		},
		{
			name: "EmptyResponse",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			method:         "DELETE",
			path:           "/resource",
			wantStatusCode: http.StatusNoContent,
			wantBody:       "",
		},
		{
			name: "MultipleWrites",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("part1"))
				w.Write([]byte("-"))
				w.Write([]byte("part2"))
			},
			method:         "GET",
			path:           "/stream",
			wantStatusCode: http.StatusOK,
			wantBody:       "part1-part2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler with logging middleware
			handler := withResponseLogging(tt.handlerFunc)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.wantStatusCode, w.Code, "Status code should match")

			// Verify body
			assert.Equal(t, tt.wantBody, w.Body.String(), "Body should match")
		})
	}
}

// TestCreateHTTPServerForMCP_OAuth tests OAuth discovery endpoint
func TestCreateHTTPServerForMCP_OAuth(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		wantStatusCode int
	}{
		{
			name:           "OAuthDiscovery_GET",
			path:           "/mcp/.well-known/oauth-authorization-server",
			method:         "GET",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "OAuthDiscovery_POST",
			path:           "/mcp/.well-known/oauth-authorization-server",
			method:         "POST",
			wantStatusCode: http.StatusNotFound,
		},
	}

	// Create minimal unified server
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err)
	defer us.Close()

	// Create HTTP server without API key
	httpServer := CreateHTTPServerForMCP(":0", us, "")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			httpServer.Handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code, "Should return 404 for OAuth discovery")
		})
	}
}

// TestCreateHTTPServerForMCP_Health tests health endpoint
func TestCreateHTTPServerForMCP_Health(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "WithoutAPIKey",
			apiKey: "",
		},
		{
			name:   "WithAPIKey",
			apiKey: "test-key-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal unified server
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{},
			}
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err)
			defer us.Close()

			// Create HTTP server
			httpServer := CreateHTTPServerForMCP(":0", us, tt.apiKey)

			// Create test request
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			// Execute request
			httpServer.Handler.ServeHTTP(w, req)

			// Health endpoint should always return 200 (no auth required)
			assert.Equal(t, http.StatusOK, w.Code, "Health should return 200")

			// Verify response body
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			// Check required fields
			assert.Contains(t, response, "status", "Response should contain status")
			assert.Contains(t, response, "specVersion", "Response should contain specVersion")
			assert.Contains(t, response, "gatewayVersion", "Response should contain gatewayVersion")
			assert.Contains(t, response, "servers", "Response should contain servers")
		})
	}
}

// TestCreateHTTPServerForMCP_Close tests close endpoint
func TestCreateHTTPServerForMCP_Close(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		apiKey         string
		authHeader     string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "ValidPOST_NoAuth",
			method:         "POST",
			apiKey:         "",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "ValidPOST_WithValidAuth",
			method:         "POST",
			apiKey:         "secret-key",
			authHeader:     "secret-key",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "POST_WithInvalidAuth",
			method:         "POST",
			apiKey:         "secret-key",
			authHeader:     "wrong-key",
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
		},
		{
			name:           "POST_MissingAuth",
			method:         "POST",
			apiKey:         "secret-key",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantError:      true,
		},
		{
			name:           "InvalidMethod_GET",
			method:         "GET",
			apiKey:         "",
			authHeader:     "",
			wantStatusCode: http.StatusMethodNotAllowed,
			wantError:      true,
		},
		{
			name:           "InvalidMethod_DELETE",
			method:         "DELETE",
			apiKey:         "",
			authHeader:     "",
			wantStatusCode: http.StatusMethodNotAllowed,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal unified server with test mode enabled
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{},
			}
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err)
			defer us.Close()

			// Enable test mode to prevent os.Exit()
			us.SetTestMode(true)

			// Create HTTP server
			httpServer := CreateHTTPServerForMCP(":0", us, tt.apiKey)

			// Create test request
			req := httptest.NewRequest(tt.method, "/close", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			// Execute request
			httpServer.Handler.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.wantStatusCode, w.Code, "Status code should match")

			// Verify response based on error expectation
			if tt.wantError {
				switch tt.wantStatusCode {
				case http.StatusMethodNotAllowed:
					// http.Error writes plain text for 405
					assert.Contains(t, w.Body.String(), "Method not allowed")
				case http.StatusUnauthorized:
					assert.Contains(t, w.Body.String(), "Unauthorized")
				}
			} else {
				// Success response should be JSON
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Equal(t, "closed", response["status"], "Status should be 'closed'")
			}
		})
	}
}

// TestCreateHTTPServerForMCP_DoubleClose tests idempotent close behavior
func TestCreateHTTPServerForMCP_DoubleClose(t *testing.T) {
	// Create minimal unified server with test mode enabled
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err)
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	// Create HTTP server
	httpServer := CreateHTTPServerForMCP(":0", us, "")

	// First close request
	req1 := httptest.NewRequest("POST", "/close", nil)
	w1 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w1, req1)

	// Should succeed
	assert.Equal(t, http.StatusOK, w1.Code, "First close should succeed")

	// Second close request
	req2 := httptest.NewRequest("POST", "/close", nil)
	w2 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w2, req2)

	// Should return 410 Gone
	assert.Equal(t, http.StatusGone, w2.Code, "Second close should return 410 Gone")

	// Verify error message
	var response map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Contains(t, response["error"], "already been closed", "Should indicate already closed")
}

// TestCreateHTTPServerForMCP_MCPEndpoint tests MCP endpoint basic routing
func TestCreateHTTPServerForMCP_MCPEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		apiKey         string
		authHeader     string
		body           io.Reader
		wantStatusCode int
	}{
		{
			name:           "MCP_GET_RequiresSession",
			path:           "/mcp",
			method:         "GET",
			apiKey:         "",
			authHeader:     "",
			body:           nil,
			wantStatusCode: http.StatusBadRequest, // GET requires active session per MCP streamable HTTP spec
		},
		{
			name:           "MCP_POST_NoAuth_WithAPIKey",
			path:           "/mcp",
			method:         "POST",
			apiKey:         "secret",
			authHeader:     "",
			body:           bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`),
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "MCPTrailing_POST_NoAuth_WithAPIKey",
			path:           "/mcp/",
			method:         "POST",
			apiKey:         "secret",
			authHeader:     "",
			body:           bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`),
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal unified server
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{},
			}
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err)
			defer us.Close()

			// Create HTTP server
			httpServer := CreateHTTPServerForMCP(":0", us, tt.apiKey)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			// Execute request
			httpServer.Handler.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.wantStatusCode, w.Code, "Status code should match")
		})
	}
}

// TestHTTPTransport_Interface tests that HTTPTransport implements sdk.Transport interface
func TestHTTPTransport_Interface(t *testing.T) {
	transport := &HTTPTransport{
		Addr: ":8080",
	}

	// Test Start
	ctx := context.Background()
	err := transport.Start(ctx)
	assert.NoError(t, err, "Start should not return error")

	// Test Send
	err = transport.Send("test message")
	assert.NoError(t, err, "Send should not return error")

	// Test Recv
	msg, err := transport.Recv()
	assert.NoError(t, err, "Recv should not return error")
	assert.Nil(t, msg, "Recv should return nil message")

	// Test Close
	err = transport.Close()
	assert.NoError(t, err, "Close should not return error")
}

// TestHTTPTransport_MultipleCalls tests HTTPTransport with multiple calls
func TestHTTPTransport_MultipleCalls(t *testing.T) {
	transport := &HTTPTransport{
		Addr: "localhost:9090",
	}

	ctx := context.Background()

	// Multiple Start calls should not fail
	for i := 0; i < 3; i++ {
		err := transport.Start(ctx)
		assert.NoError(t, err, "Start should not return error on call %d", i)
	}

	// Multiple Send calls should not fail
	for i := 0; i < 3; i++ {
		err := transport.Send(map[string]string{"test": "data"})
		assert.NoError(t, err, "Send should not return error on call %d", i)
	}

	// Multiple Close calls should not fail
	for i := 0; i < 3; i++ {
		err := transport.Close()
		assert.NoError(t, err, "Close should not return error on call %d", i)
	}
}
