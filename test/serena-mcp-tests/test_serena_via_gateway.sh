#!/bin/bash
# Comprehensive test script for Serena MCP Server through MCP Gateway
# Tests multi-language support: Go, Java, JavaScript, Python
# Tests MCP protocol interactions through gateway HTTP endpoint
#
# This test suite connects to Serena through the MCP Gateway to identify
# any differences in behavior compared to direct connection tests.
#
# Portability: Compatible with bash 3.2+ (macOS) and bash 4+ (Ubuntu/Linux)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GATEWAY_IMAGE="${GATEWAY_IMAGE:-ghcr.io/github/gh-aw-mcpg:latest}"
SERENA_IMAGE="${SERENA_IMAGE:-ghcr.io/github/serena-mcp-server:latest}"
TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
SAMPLES_DIR="${TEST_DIR}/samples"
EXPECTED_DIR="${TEST_DIR}/expected"
RESULTS_DIR="${TEST_DIR}/results-gateway"
TEMP_DIR="/tmp/serena-gateway-test-$$"
GATEWAY_PORT=18080
GATEWAY_API_KEY="test-api-key-$$"
GATEWAY_CONTAINER_NAME="serena-gateway-test-$$"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
    ((TESTS_PASSED++)) || true
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
    ((TESTS_FAILED++)) || true
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Increment test counter
count_test() {
    ((TESTS_TOTAL++)) || true
}

# Cleanup function
cleanup() {
    log_info "Cleaning up..."
    
    # Kill background gateway process if it exists
    if [ -n "$GATEWAY_PID" ] && kill -0 "$GATEWAY_PID" 2>/dev/null; then
        log_info "Stopping gateway process (PID: $GATEWAY_PID)..."
        kill "$GATEWAY_PID" 2>/dev/null || true
        sleep 2
        # Force kill if still running
        kill -9 "$GATEWAY_PID" 2>/dev/null || true
    fi
    
    # Stop and remove container if it exists
    docker stop "$GATEWAY_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$GATEWAY_CONTAINER_NAME" >/dev/null 2>&1 || true
    
    # Clean up temp files
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

# Initialize
log_section "Serena MCP Server Test Suite (via Gateway)"
log_info "Gateway Image: $GATEWAY_IMAGE"
log_info "Serena Image: $SERENA_IMAGE"
log_info "Test Directory: $TEST_DIR"
log_info "Samples Directory: $SAMPLES_DIR"
log_info "Gateway Port: $GATEWAY_PORT"
echo ""

# Create temporary and results directories
mkdir -p "$TEMP_DIR"
mkdir -p "$RESULTS_DIR"

# Test 1: Check if Docker is available
log_section "Test 1: Docker Availability"
count_test
if command -v docker >/dev/null 2>&1; then
    log_success "Docker is installed"
else
    log_error "Docker is not installed"
    exit 1
fi

# Test 2: Check if curl is available
log_section "Test 2: Curl Availability"
count_test
if command -v curl >/dev/null 2>&1; then
    log_success "Curl is installed"
else
    log_error "Curl is not installed"
    exit 1
fi

# Test 3: Pull gateway container image
log_section "Test 3: Gateway Container Image Availability"
count_test
# Check if image exists locally first
if docker image inspect "$GATEWAY_IMAGE" >/dev/null 2>&1; then
    log_info "Using local gateway image (skipping pull)"
    log_success "Gateway container image is available"
elif docker pull "$GATEWAY_IMAGE" >/dev/null 2>&1; then
    log_info "Pulling gateway container image..."
    log_success "Gateway container image is available"
else
    log_error "Failed to pull gateway container image: $GATEWAY_IMAGE"
    exit 1
fi

# Test 4: Pull Serena container image
log_section "Test 4: Serena Container Image Availability"
count_test
log_info "Pulling Serena container image..."
if docker pull "$SERENA_IMAGE" >/dev/null 2>&1; then
    log_success "Serena container image is available"
else
    log_error "Failed to pull Serena container image: $SERENA_IMAGE"
    exit 1
fi

# Test 5: Create gateway config and start gateway
log_section "Test 5: Start MCP Gateway with Serena Backend"
count_test

# Create gateway configuration
cat > "$TEMP_DIR/gateway-config.json" <<EOF
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "$SERENA_IMAGE",
      "mounts": [
        "$SAMPLES_DIR:/workspace:ro"
      ],
      "env": {
        "NO_COLOR": "1",
        "TERM": "dumb"
      }
    }
  },
  "gateway": {
    "port": $GATEWAY_PORT,
    "domain": "localhost",
    "apiKey": "$GATEWAY_API_KEY"
  }
}
EOF

log_info "Gateway config created at: $TEMP_DIR/gateway-config.json"
log_info "Starting MCP Gateway..."

# Start gateway container with config via stdin
# We need to run it in the background since docker run -i is blocking
# Note: Environment variables are still required even with config-stdin
cat "$TEMP_DIR/gateway-config.json" | docker run --rm -i \
    --name "$GATEWAY_CONTAINER_NAME" \
    -e MCP_GATEWAY_PORT="$GATEWAY_PORT" \
    -e MCP_GATEWAY_DOMAIN=localhost \
    -e MCP_GATEWAY_API_KEY="$GATEWAY_API_KEY" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$SAMPLES_DIR:$SAMPLES_DIR:ro" \
    -p "$GATEWAY_PORT:$GATEWAY_PORT" \
    "$GATEWAY_IMAGE" > "$TEMP_DIR/gateway.log" 2>&1 &

GATEWAY_PID=$!

# Wait for gateway to be ready
log_info "Waiting for gateway to be ready (PID: $GATEWAY_PID)..."
sleep 5

# Check if process is still running
if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    log_error "Gateway process died unexpectedly"
    log_info "Gateway logs:"
    cat "$TEMP_DIR/gateway.log"
    exit 1
fi

# Wait for gateway to fully initialize (Serena backend takes ~20-25 seconds to start)
log_info "Waiting for Serena backend initialization (this may take 20-30 seconds)..."
sleep 25

# Check if process is still running after backend init
if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    log_error "Gateway process died during initialization"
    log_info "Gateway logs:"
    cat "$TEMP_DIR/gateway.log"
    exit 1
fi

# Check if gateway is responding with retries
log_info "Testing gateway connection..."
GATEWAY_TEST="0"
for i in 1 2 3 4 5; do
    GATEWAY_TEST=$(curl -s -X POST "http://localhost:$GATEWAY_PORT/mcp/serena" \
        -H "Content-Type: application/json" \
        -H "Authorization: $GATEWAY_API_KEY" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
        2>/dev/null | grep -c '"jsonrpc"' || echo "0")
    
    if [ "$GATEWAY_TEST" != "0" ]; then
        break
    fi
    log_info "Retry $i/5..."
    sleep 3
done

if [ "$GATEWAY_TEST" != "0" ]; then
    log_success "MCP Gateway started successfully and is responding"
    log_info "Gateway endpoint: http://localhost:$GATEWAY_PORT/mcp/serena"
else
    log_error "Failed to start MCP Gateway"
    docker logs "$GATEWAY_CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi

# Session ID for MCP Streamable HTTP transport
# Stored in a file to survive subshell boundaries
MCP_SESSION_FILE="$TEMP_DIR/mcp_session_id.txt"
echo "" > "$MCP_SESSION_FILE"

# Response file for avoiding subshell variable scope issues
MCP_RESPONSE_FILE="$TEMP_DIR/mcp_response.txt"

# Function to get the current session ID
get_session_id() {
    cat "$MCP_SESSION_FILE" 2>/dev/null || echo ""
}

# Function to send MCP request and capture session ID
# This writes response to MCP_RESPONSE_FILE and updates session ID file
send_mcp_request_direct() {
    local request="$1"
    local endpoint="http://localhost:$GATEWAY_PORT/mcp/serena"
    local headers_file="$TEMP_DIR/response_headers.txt"
    local session_id=$(get_session_id)
    
    # Run curl and save response to file
    if [ -n "$session_id" ]; then
        curl -s -X POST "$endpoint" \
            -H "Content-Type: application/json" \
            -H "Accept: application/json, text/event-stream" \
            -H "Authorization: $GATEWAY_API_KEY" \
            -H "Mcp-Session-Id: $session_id" \
            -D "$headers_file" \
            -d "$request" > "$MCP_RESPONSE_FILE" 2>/dev/null || echo '{"error": "request failed"}' > "$MCP_RESPONSE_FILE"
    else
        curl -s -X POST "$endpoint" \
            -H "Content-Type: application/json" \
            -H "Accept: application/json, text/event-stream" \
            -H "Authorization: $GATEWAY_API_KEY" \
            -D "$headers_file" \
            -d "$request" > "$MCP_RESPONSE_FILE" 2>/dev/null || echo '{"error": "request failed"}' > "$MCP_RESPONSE_FILE"
    fi
    
    # Capture session ID from response headers and save to file
    if [ -f "$headers_file" ]; then
        local new_session_id=$(grep -i "^mcp-session-id:" "$headers_file" | sed 's/^[Mm]cp-[Ss]ession-[Ii]d: *//;s/\r$//' | head -1)
        if [ -n "$new_session_id" ]; then
            echo "$new_session_id" > "$MCP_SESSION_FILE"
        fi
    fi
}

# Function to send MCP request and get the raw response
# Call send_mcp_request_direct first, then use get_mcp_response
get_mcp_response() {
    cat "$MCP_RESPONSE_FILE"
}

# Function to get parsed JSON from SSE response
get_mcp_response_json() {
    local raw_response=$(cat "$MCP_RESPONSE_FILE")
    
    # Check if response is SSE format
    if echo "$raw_response" | grep -q "^event: message"; then
        # Extract JSON from "data: {...}" line
        echo "$raw_response" | grep "^data: " | sed 's/^data: //' | tail -1
    else
        # Already JSON
        echo "$raw_response"
    fi
}

# Legacy function for backward compatibility
send_mcp_request() {
    local request="$1"
    send_mcp_request_direct "$request"
    get_mcp_response
}

# Legacy function for backward compatibility - parses SSE response
# NOTE: Session ID is now persisted to file, so it survives subshell usage
send_and_parse_mcp_request() {
    local request="$1"
    send_mcp_request_direct "$request"
    get_mcp_response_json
}

# Test 6: MCP Protocol - Initialize
log_section "Test 6: MCP Protocol Initialize (via Gateway)"
count_test
log_info "Sending MCP initialize request through gateway..."

INIT_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'

# Send request - session ID will be captured and saved to file
INIT_JSON=$(send_and_parse_mcp_request "$INIT_REQUEST")

echo "$INIT_JSON" > "$RESULTS_DIR/initialize_response.json"

MCP_SESSION_ID=$(get_session_id)
if echo "$INIT_JSON" | grep -q '"jsonrpc"'; then
    if echo "$INIT_JSON" | grep -q '"result"'; then
        log_success "MCP initialize succeeded through gateway"
        log_info "Response saved to: $RESULTS_DIR/initialize_response.json"
        if [ -n "$MCP_SESSION_ID" ]; then
            log_info "MCP Session ID: $MCP_SESSION_ID"
        else
            log_warning "No MCP Session ID captured - subsequent requests may fail"
        fi
    else
        log_error "MCP initialize returned error through gateway"
        echo "$INIT_JSON" | head -5
    fi
else
    log_error "MCP initialize failed - no valid JSON-RPC response"
    echo "$INIT_JSON" | head -10
fi

# Send initialized notification to complete handshake
log_info "Sending initialized notification to complete MCP handshake..."
INITIALIZED_NOTIF='{"jsonrpc":"2.0","method":"notifications/initialized"}'
send_mcp_request_direct "$INITIALIZED_NOTIF"

# Give the server a moment to process the notification
sleep 1

# Test 7: MCP Protocol - List Tools
log_section "Test 7: MCP Protocol - List Available Tools (via Gateway)"
count_test
log_info "Requesting list of available tools through gateway..."

TOOLS_REQUEST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

TOOLS_JSON=$(send_and_parse_mcp_request "$TOOLS_REQUEST")

echo "$TOOLS_JSON" > "$RESULTS_DIR/tools_list_response.json"

if echo "$TOOLS_JSON" | grep -q '"tools"'; then
    TOOL_COUNT=$(echo "$TOOLS_JSON" | grep -o '"name"' | wc -l)
    log_success "Tools list retrieved through gateway - found $TOOL_COUNT tools"
    log_info "Response saved to: $RESULTS_DIR/tools_list_response.json"
    
    # Display available tools
    log_info "Available Serena tools:"
    echo "$TOOLS_JSON" | grep -o '"name":"[^"]*"' | sed 's/"name":"/  - /' | sed 's/"$//'
else
    log_error "Failed to retrieve tools list through gateway"
    echo "$TOOLS_JSON" | head -10
fi

# Test 8: Go Code Analysis
log_section "Test 8: Go Code Analysis Tests (via Gateway)"

if [ -f "$SAMPLES_DIR/go_project/main.go" ]; then
    log_info "Testing Go project at: $SAMPLES_DIR/go_project"
    
    # Test 8a: Go - Find symbols
    count_test
    log_info "Test 8a: Finding symbols in Go code through gateway..."
    
    SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"go_project/main.go"}}}'
    
    GO_RESPONSE=$(send_and_parse_mcp_request "$SYMBOLS_REQUEST")
    
    echo "$GO_RESPONSE" > "$RESULTS_DIR/go_symbols_response.json"
    
    if echo "$GO_RESPONSE" | grep -q -E '(Calculator|NewCalculator|Add|Multiply)'; then
        log_success "Go symbol analysis working through gateway - found expected symbols"
    elif echo "$GO_RESPONSE" | grep -q '"result"'; then
        log_success "Go symbol analysis completed successfully through gateway"
    else
        log_error "Go symbol analysis failed through gateway"
        echo "$GO_RESPONSE" | head -10
    fi
    
    # Test 8b: Go - Find specific symbol
    count_test
    log_info "Test 8b: Finding specific Calculator symbol through gateway..."
    
    FIND_SYMBOL_REQUEST='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"go_project"}}}'
    
    FIND_RESPONSE=$(send_and_parse_mcp_request "$FIND_SYMBOL_REQUEST")
    
    echo "$FIND_RESPONSE" > "$RESULTS_DIR/go_find_symbol_response.json"
    
    if echo "$FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Go find_symbol completed successfully through gateway"
    else
        log_error "Go find_symbol failed through gateway"
        echo "$FIND_RESPONSE" | head -10
    fi
else
    log_warning "Go project not found, skipping Go tests"
fi

# Test 9: Java Code Analysis
log_section "Test 9: Java Code Analysis Tests (via Gateway)"

if [ -f "$SAMPLES_DIR/java_project/Calculator.java" ]; then
    log_info "Testing Java project at: $SAMPLES_DIR/java_project"
    
    # Test 9a: Java - Find symbols
    count_test
    log_info "Test 9a: Finding symbols in Java code through gateway..."
    
    JAVA_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"java_project/Calculator.java"}}}'
    
    JAVA_RESPONSE=$(send_and_parse_mcp_request "$JAVA_SYMBOLS_REQUEST")
    
    echo "$JAVA_RESPONSE" > "$RESULTS_DIR/java_symbols_response.json"
    
    if echo "$JAVA_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Java symbol analysis working through gateway - found expected symbols"
    elif echo "$JAVA_RESPONSE" | grep -q '"result"'; then
        log_success "Java symbol analysis completed successfully through gateway"
    else
        log_error "Java symbol analysis failed through gateway"
        echo "$JAVA_RESPONSE" | head -10
    fi
    
    # Test 9b: Java - Find specific symbol
    count_test
    log_info "Test 9b: Finding specific Calculator symbol through gateway..."
    
    JAVA_FIND_REQUEST='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"java_project"}}}'
    
    JAVA_FIND_RESPONSE=$(send_and_parse_mcp_request "$JAVA_FIND_REQUEST")
    
    echo "$JAVA_FIND_RESPONSE" > "$RESULTS_DIR/java_find_symbol_response.json"
    
    if echo "$JAVA_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Java find_symbol completed successfully through gateway"
    else
        log_error "Java find_symbol failed through gateway"
        echo "$JAVA_FIND_RESPONSE" | head -10
    fi
else
    log_warning "Java project not found, skipping Java tests"
fi

# Test 10: JavaScript Code Analysis
log_section "Test 10: JavaScript Code Analysis Tests (via Gateway)"

if [ -f "$SAMPLES_DIR/js_project/calculator.js" ]; then
    log_info "Testing JavaScript project at: $SAMPLES_DIR/js_project"
    
    # Test 10a: JavaScript - Find symbols
    count_test
    log_info "Test 10a: Finding symbols in JavaScript code through gateway..."
    
    JS_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"js_project/calculator.js"}}}'
    
    JS_RESPONSE=$(send_and_parse_mcp_request "$JS_SYMBOLS_REQUEST")
    
    echo "$JS_RESPONSE" > "$RESULTS_DIR/js_symbols_response.json"
    
    if echo "$JS_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "JavaScript symbol analysis working through gateway - found expected symbols"
    elif echo "$JS_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript symbol analysis completed successfully through gateway"
    else
        log_error "JavaScript symbol analysis failed through gateway"
        echo "$JS_RESPONSE" | head -10
    fi
    
    # Test 10b: JavaScript - Find specific symbol
    count_test
    log_info "Test 10b: Finding specific Calculator symbol through gateway..."
    
    JS_FIND_REQUEST='{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"js_project"}}}'
    
    JS_FIND_RESPONSE=$(send_and_parse_mcp_request "$JS_FIND_REQUEST")
    
    echo "$JS_FIND_RESPONSE" > "$RESULTS_DIR/js_find_symbol_response.json"
    
    if echo "$JS_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript find_symbol completed successfully through gateway"
    else
        log_error "JavaScript find_symbol failed through gateway"
        echo "$JS_FIND_RESPONSE" | head -10
    fi
else
    log_warning "JavaScript project not found, skipping JavaScript tests"
fi

# Test 11: Python Code Analysis
log_section "Test 11: Python Code Analysis Tests (via Gateway)"

if [ -f "$SAMPLES_DIR/python_project/calculator.py" ]; then
    log_info "Testing Python project at: $SAMPLES_DIR/python_project"
    
    # Test 11a: Python - Find symbols
    count_test
    log_info "Test 11a: Finding symbols in Python code through gateway..."
    
    PY_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"python_project/calculator.py"}}}'
    
    PY_RESPONSE=$(send_and_parse_mcp_request "$PY_SYMBOLS_REQUEST")
    
    echo "$PY_RESPONSE" > "$RESULTS_DIR/py_symbols_response.json"
    
    if echo "$PY_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Python symbol analysis working through gateway - found expected symbols"
    elif echo "$PY_RESPONSE" | grep -q '"result"'; then
        log_success "Python symbol analysis completed successfully through gateway"
    else
        log_error "Python symbol analysis failed through gateway"
        echo "$PY_RESPONSE" | head -10
    fi
    
    # Test 11b: Python - Find specific symbol
    count_test
    log_info "Test 11b: Finding specific Calculator symbol through gateway..."
    
    PY_FIND_REQUEST='{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"python_project"}}}'
    
    PY_FIND_RESPONSE=$(send_and_parse_mcp_request "$PY_FIND_REQUEST")
    
    echo "$PY_FIND_RESPONSE" > "$RESULTS_DIR/py_find_symbol_response.json"
    
    if echo "$PY_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Python find_symbol completed successfully through gateway"
    else
        log_error "Python find_symbol failed through gateway"
        echo "$PY_FIND_RESPONSE" | head -10
    fi
else
    log_warning "Python project not found, skipping Python tests"
fi

# Test 12: File Operations
log_section "Test 12: File Operations (via Gateway)"

# Test 12a: list_dir
count_test
log_info "Test 12a: Testing list_dir tool through gateway..."

LIST_DIR_REQUEST='{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"list_dir","arguments":{"relative_path":"go_project"}}}'

LIST_DIR_RESPONSE=$(send_and_parse_mcp_request "$LIST_DIR_REQUEST")

echo "$LIST_DIR_RESPONSE" > "$RESULTS_DIR/list_dir_response.json"

if echo "$LIST_DIR_RESPONSE" | grep -q '"result"'; then
    log_success "list_dir completed successfully through gateway"
else
    log_error "list_dir failed through gateway"
    echo "$LIST_DIR_RESPONSE" | head -10
fi

# Test 12b: find_file
count_test
log_info "Test 12b: Testing find_file tool through gateway..."

FIND_FILE_REQUEST='{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"find_file","arguments":{"pattern":"*.go","relative_path":"go_project"}}}'

FIND_FILE_RESPONSE=$(send_and_parse_mcp_request "$FIND_FILE_REQUEST")

echo "$FIND_FILE_RESPONSE" > "$RESULTS_DIR/find_file_response.json"

if echo "$FIND_FILE_RESPONSE" | grep -q '"result"'; then
    log_success "find_file completed successfully through gateway"
else
    log_error "find_file failed through gateway"
    echo "$FIND_FILE_RESPONSE" | head -10
fi

# Test 12c: search_for_pattern
count_test
log_info "Test 12c: Testing search_for_pattern tool through gateway..."

SEARCH_PATTERN_REQUEST='{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"search_for_pattern","arguments":{"pattern":"Calculator","relative_path":"go_project"}}}'

SEARCH_PATTERN_RESPONSE=$(send_and_parse_mcp_request "$SEARCH_PATTERN_REQUEST")

echo "$SEARCH_PATTERN_RESPONSE" > "$RESULTS_DIR/search_pattern_response.json"

if echo "$SEARCH_PATTERN_RESPONSE" | grep -q '"result"'; then
    log_success "search_for_pattern completed successfully through gateway"
else
    log_error "search_for_pattern failed through gateway"
    echo "$SEARCH_PATTERN_RESPONSE" | head -10
fi

# Test 13: Memory Operations
log_section "Test 13: Memory Operations (via Gateway)"

# Test 13a: write_memory
count_test
log_info "Test 13a: Testing write_memory tool through gateway..."

WRITE_MEMORY_REQUEST='{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"write_memory","arguments":{"key":"test_key","value":"test_value"}}}'

WRITE_MEMORY_RESPONSE=$(send_and_parse_mcp_request "$WRITE_MEMORY_REQUEST")

echo "$WRITE_MEMORY_RESPONSE" > "$RESULTS_DIR/write_memory_response.json"

if echo "$WRITE_MEMORY_RESPONSE" | grep -q '"result"'; then
    log_success "write_memory completed successfully through gateway"
else
    log_error "write_memory failed through gateway"
    echo "$WRITE_MEMORY_RESPONSE" | head -10
fi

# Test 13b: read_memory
count_test
log_info "Test 13b: Testing read_memory tool through gateway..."

READ_MEMORY_REQUEST='{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"read_memory","arguments":{"key":"test_key"}}}'

READ_MEMORY_RESPONSE=$(send_and_parse_mcp_request "$READ_MEMORY_REQUEST")

echo "$READ_MEMORY_RESPONSE" > "$RESULTS_DIR/read_memory_response.json"

if echo "$READ_MEMORY_RESPONSE" | grep -q '"result"'; then
    if echo "$READ_MEMORY_RESPONSE" | grep -q "test_value"; then
        log_success "read_memory completed successfully and returned expected value through gateway"
    else
        log_success "read_memory completed successfully through gateway"
    fi
else
    log_error "read_memory failed through gateway"
    echo "$READ_MEMORY_RESPONSE" | head -10
fi

# Test 13c: list_memories
count_test
log_info "Test 13c: Testing list_memories tool through gateway..."

LIST_MEMORIES_REQUEST='{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"list_memories","arguments":{}}}'

LIST_MEMORIES_RESPONSE=$(send_and_parse_mcp_request "$LIST_MEMORIES_REQUEST")

echo "$LIST_MEMORIES_RESPONSE" > "$RESULTS_DIR/list_memories_response.json"

if echo "$LIST_MEMORIES_RESPONSE" | grep -q '"result"'; then
    log_success "list_memories completed successfully through gateway"
else
    log_error "list_memories failed through gateway"
    echo "$LIST_MEMORIES_RESPONSE" | head -10
fi

# Test 14: Error Handling
log_section "Test 14: Error Handling (via Gateway)"

# Test 14a: Invalid tool name
count_test
log_info "Test 14a: Testing invalid tool name error handling through gateway..."

INVALID_TOOL_REQUEST='{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"nonexistent_tool","arguments":{}}}'

INVALID_TOOL_RESPONSE=$(send_and_parse_mcp_request "$INVALID_TOOL_REQUEST")

echo "$INVALID_TOOL_RESPONSE" > "$RESULTS_DIR/invalid_tool_response.json"

if echo "$INVALID_TOOL_RESPONSE" | grep -q '"error"'; then
    log_success "Invalid tool error handling working through gateway"
else
    log_warning "Expected error response for invalid tool through gateway"
fi

# Test 14b: Malformed JSON
count_test
log_info "Test 14b: Testing malformed JSON error handling through gateway..."

MALFORMED_REQUEST='{"jsonrpc":"2.0","id":18,"method":"tools/call","params":{'

MALFORMED_RESPONSE=$(send_and_parse_mcp_request "$MALFORMED_REQUEST")

echo "$MALFORMED_RESPONSE" > "$RESULTS_DIR/malformed_json_response.json"

if echo "$MALFORMED_RESPONSE" | grep -q '"error"'; then
    log_success "Malformed JSON error handling working through gateway"
else
    log_warning "Expected error response for malformed JSON through gateway"
fi

# Final Summary
log_section "Test Summary"
echo ""
log_info "Total Tests: $TESTS_TOTAL"
log_info "Passed: $TESTS_PASSED"
log_info "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    log_success "All tests passed! ✨"
    log_info "Results saved to: $RESULTS_DIR"
    echo ""
    log_info "Gateway logs (last 30 lines):"
    if [ -f "$TEMP_DIR/gateway.log" ]; then
        tail -30 "$TEMP_DIR/gateway.log"
    else
        echo "No gateway logs available"
    fi
    exit 0
else
    log_error "Some tests failed. Please review the results."
    log_info "Results saved to: $RESULTS_DIR"
    echo ""
    log_info "Gateway logs (last 50 lines):"
    if [ -f "$TEMP_DIR/gateway.log" ]; then
        tail -50 "$TEMP_DIR/gateway.log"
    else
        echo "No gateway logs available"
    fi
    exit 1
fi
