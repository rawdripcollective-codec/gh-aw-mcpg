#!/bin/bash
# Comprehensive test script for Serena MCP Server
# Tests multi-language support: Go, Java, JavaScript, Python
# Tests MCP protocol interactions and validates responses
#
# Portability: Compatible with bash 3.2+ (macOS) and bash 4+ (Ubuntu/Linux)
# - Uses helper functions instead of associative arrays (bash 4+ feature)
# - Arithmetic expressions use || true to work with set -e

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_IMAGE="${SERENA_IMAGE:-ghcr.io/github/serena-mcp-server:latest}"
TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
SAMPLES_DIR="${TEST_DIR}/samples"
EXPECTED_DIR="${TEST_DIR}/expected"
RESULTS_DIR="${TEST_DIR}/results"
TEMP_DIR="/tmp/serena-test-$$"

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
    log_info "Cleaning up temporary files..."
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

# Initialize
log_section "Serena MCP Server Comprehensive Test Suite"
log_info "Container Image: $CONTAINER_IMAGE"
log_info "Test Directory: $TEST_DIR"
log_info "Samples Directory: $SAMPLES_DIR"
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

# Test 2: Check if container image is available
log_section "Test 2: Container Image Availability"
count_test
log_info "Pulling container image (this may take a while)..."
if docker pull "$CONTAINER_IMAGE" >/dev/null 2>&1; then
    log_success "Container image is available"
else
    log_error "Failed to pull container image: $CONTAINER_IMAGE"
    log_warning "Make sure the image exists and you have proper credentials"
    exit 1
fi

# Test 3: Container help command
log_section "Test 3: Container Basic Functionality"
count_test
if docker run --rm "$CONTAINER_IMAGE" --help >/dev/null 2>&1; then
    log_success "Container help command works"
else
    log_error "Container help command failed"
fi

# Test 4: Verify language runtimes
log_section "Test 4: Language Runtime Verification"

# Python
count_test
if docker run --rm --entrypoint python3 "$CONTAINER_IMAGE" --version >/dev/null 2>&1; then
    PYTHON_VERSION=$(docker run --rm --entrypoint python3 "$CONTAINER_IMAGE" --version 2>&1)
    log_success "Python runtime available: $PYTHON_VERSION"
else
    log_error "Python runtime not found"
fi

# Java
count_test
if docker run --rm --entrypoint java "$CONTAINER_IMAGE" -version >/dev/null 2>&1; then
    JAVA_VERSION=$(docker run --rm --entrypoint java "$CONTAINER_IMAGE" -version 2>&1 | head -1)
    log_success "Java runtime available: $JAVA_VERSION"
else
    log_error "Java runtime not found"
fi

# Node.js
count_test
if docker run --rm --entrypoint node "$CONTAINER_IMAGE" --version >/dev/null 2>&1; then
    NODE_VERSION=$(docker run --rm --entrypoint node "$CONTAINER_IMAGE" --version 2>&1)
    log_success "Node.js runtime available: $NODE_VERSION"
else
    log_error "Node.js runtime not found"
fi

# Go
count_test
if docker run --rm --entrypoint go "$CONTAINER_IMAGE" version >/dev/null 2>&1; then
    GO_VERSION=$(docker run --rm --entrypoint go "$CONTAINER_IMAGE" version 2>&1)
    log_success "Go runtime available: $GO_VERSION"
else
    log_error "Go runtime not found"
fi

# Test 5: MCP Protocol - Initialize
log_section "Test 5: MCP Protocol Initialize"
count_test
log_info "Sending MCP initialize request..."

INIT_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'

INIT_RESPONSE=$(echo "$INIT_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null || echo '{"error": "failed"}')

echo "$INIT_RESPONSE" > "$RESULTS_DIR/initialize_response.json"

if echo "$INIT_RESPONSE" | grep -q '"jsonrpc"'; then
    if echo "$INIT_RESPONSE" | grep -q '"result"'; then
        log_success "MCP initialize succeeded"
        log_info "Response saved to: $RESULTS_DIR/initialize_response.json"
    else
        log_error "MCP initialize returned error"
        echo "$INIT_RESPONSE" | head -5
    fi
else
    log_error "MCP initialize failed - no valid JSON-RPC response"
    echo "$INIT_RESPONSE" | head -10
fi

# Test 6: MCP Protocol - List Tools
log_section "Test 6: MCP Protocol - List Available Tools"
count_test
log_info "Requesting list of available tools..."

# First send initialize, then tools/list
TOOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

TOOLS_RESPONSE=$(echo "$TOOLS_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$TOOLS_RESPONSE" > "$RESULTS_DIR/tools_list_response.json"

if echo "$TOOLS_RESPONSE" | grep -q '"tools"'; then
    TOOL_COUNT=$(echo "$TOOLS_RESPONSE" | grep -o '"name"' | wc -l)
    log_success "Tools list retrieved - found $TOOL_COUNT tools"
    log_info "Response saved to: $RESULTS_DIR/tools_list_response.json"
    
    # Display available tools
    log_info "Available Serena tools:"
    echo "$TOOLS_RESPONSE" | grep -o '"name":"[^"]*"' | sed 's/"name":"/  - /' | sed 's/"$//'
else
    log_error "Failed to retrieve tools list"
    echo "$TOOLS_RESPONSE" | head -10
fi

# Test 7: Go Code Analysis
log_section "Test 7: Go Code Analysis Tests"

if [ -f "$SAMPLES_DIR/go_project/main.go" ]; then
    log_info "Testing Go project at: $SAMPLES_DIR/go_project"
    
    # Test 7a: Go - Find symbols
    count_test
    log_info "Test 7a: Finding symbols in Go code..."
    
    SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"go_project/main.go"}}}'
    
    GO_RESPONSE=$(echo "$SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$GO_RESPONSE" > "$RESULTS_DIR/go_symbols_response.json"
    
    if echo "$GO_RESPONSE" | grep -q -E '(Calculator|NewCalculator|Add|Multiply)'; then
        log_success "Go symbol analysis working - found expected symbols"
    elif echo "$GO_RESPONSE" | grep -q '"result"'; then
        log_success "Go symbol analysis completed successfully"
    else
        log_error "Go symbol analysis failed"
    fi
    
    # Test 7b: Go - Find specific symbol
    count_test
    log_info "Test 7b: Finding specific Calculator symbol..."
    
    FIND_SYMBOL_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"go_project"}}}'
    
    FIND_RESPONSE=$(echo "$FIND_SYMBOL_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$FIND_RESPONSE" > "$RESULTS_DIR/go_find_symbol_response.json"
    
    if echo "$FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Go find_symbol completed successfully"
    else
        log_error "Go find_symbol failed"
    fi
else
    log_warning "Go project not found, skipping Go tests"
fi

# Test 8: Java Code Analysis
log_section "Test 8: Java Code Analysis Tests"

if [ -f "$SAMPLES_DIR/java_project/Calculator.java" ]; then
    log_info "Testing Java project at: $SAMPLES_DIR/java_project"
    
    # Test 8a: Java - Find symbols
    count_test
    log_info "Test 8a: Finding symbols in Java code..."
    
    JAVA_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"java_project/Calculator.java"}}}'
    
    JAVA_RESPONSE=$(echo "$JAVA_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JAVA_RESPONSE" > "$RESULTS_DIR/java_symbols_response.json"
    
    if echo "$JAVA_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Java symbol analysis working - found expected symbols"
    elif echo "$JAVA_RESPONSE" | grep -q '"result"'; then
        log_success "Java symbol analysis completed successfully"
    else
        log_error "Java symbol analysis failed"
    fi
    
    # Test 8b: Java - Find specific symbol
    count_test
    log_info "Test 8b: Finding specific Calculator symbol..."
    
    JAVA_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"java_project"}}}'
    
    JAVA_FIND_RESPONSE=$(echo "$JAVA_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JAVA_FIND_RESPONSE" > "$RESULTS_DIR/java_find_symbol_response.json"
    
    if echo "$JAVA_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Java find_symbol completed successfully"
    else
        log_error "Java find_symbol failed"
    fi
else
    log_warning "Java project not found, skipping Java tests"
fi

# Test 9: JavaScript Code Analysis
log_section "Test 9: JavaScript Code Analysis Tests"

if [ -f "$SAMPLES_DIR/js_project/calculator.js" ]; then
    log_info "Testing JavaScript project at: $SAMPLES_DIR/js_project"
    
    # Test 9a: JavaScript - Find symbols
    count_test
    log_info "Test 9a: Finding symbols in JavaScript code..."
    
    JS_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"js_project/calculator.js"}}}'
    
    JS_RESPONSE=$(echo "$JS_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JS_RESPONSE" > "$RESULTS_DIR/js_symbols_response.json"
    
    if echo "$JS_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "JavaScript symbol analysis working - found expected symbols"
    elif echo "$JS_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript symbol analysis completed successfully"
    else
        log_error "JavaScript symbol analysis failed"
    fi
    
    # Test 9b: JavaScript - Find specific symbol
    count_test
    log_info "Test 9b: Finding specific Calculator symbol..."
    
    JS_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"js_project"}}}'
    
    JS_FIND_RESPONSE=$(echo "$JS_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JS_FIND_RESPONSE" > "$RESULTS_DIR/js_find_symbol_response.json"
    
    if echo "$JS_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript find_symbol completed successfully"
    else
        log_error "JavaScript find_symbol failed"
    fi
else
    log_warning "JavaScript project not found, skipping JavaScript tests"
fi

# Test 10: Python Code Analysis
log_section "Test 10: Python Code Analysis Tests"

if [ -f "$SAMPLES_DIR/python_project/calculator.py" ]; then
    log_info "Testing Python project at: $SAMPLES_DIR/python_project"
    
    # Test 10a: Python - Find symbols
    count_test
    log_info "Test 10a: Finding symbols in Python code..."
    
    PY_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"python_project/calculator.py"}}}'
    
    PY_RESPONSE=$(echo "$PY_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$PY_RESPONSE" > "$RESULTS_DIR/python_symbols_response.json"
    
    if echo "$PY_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Python symbol analysis working - found expected symbols"
    elif echo "$PY_RESPONSE" | grep -q '"result"'; then
        log_success "Python symbol analysis completed successfully"
    else
        log_error "Python symbol analysis failed"
    fi
    
    # Test 10b: Python - Find specific symbol
    count_test
    log_info "Test 10b: Finding specific Calculator symbol..."
    
    PY_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"python_project"}}}'
    
    PY_FIND_RESPONSE=$(echo "$PY_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$PY_FIND_RESPONSE" > "$RESULTS_DIR/python_find_symbol_response.json"
    
    if echo "$PY_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Python find_symbol completed successfully"
    else
        log_error "Python find_symbol failed"
    fi
else
    log_warning "Python project not found, skipping Python tests"
fi

# Helper function to test a tool for a specific language
test_tool_for_language() {
    local language=$1
    local tool_name=$2
    local args=$3
    local project_path=$4
    local test_id=$5
    local result_file=$6
    
    count_test
    log_info "Testing ${tool_name} for ${language}..."
    
    TOOL_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'${test_id}',"method":"tools/call","params":{"name":"'${tool_name}'"'${args}'}}'
    
    TOOL_RESPONSE=$(echo "$TOOL_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$TOOL_RESPONSE" > "$RESULTS_DIR/${result_file}"
    
    if echo "$TOOL_RESPONSE" | grep -q '"result"'; then
        log_success "${language} ${tool_name} completed successfully"
        return 0
    else
        log_error "${language} ${tool_name} failed"
        return 1
    fi
}

# Test 11: Comprehensive Tool Testing for All Languages
log_section "Test 11: Comprehensive Tool Testing"

# Helper functions to get language-specific paths (bash 3.x compatible)
get_language_project() {
    case "$1" in
        "Go") echo "go_project" ;;
        "Java") echo "java_project" ;;
        "JavaScript") echo "js_project" ;;
        "Python") echo "python_project" ;;
    esac
}

get_language_file() {
    case "$1" in
        "Go") echo "go_project/main.go" ;;
        "Java") echo "java_project/Calculator.java" ;;
        "JavaScript") echo "js_project/calculator.js" ;;
        "Python") echo "python_project/calculator.py" ;;
    esac
}

TEST_ID=1000

# Test list_dir for all languages
log_section "Test 11a: list_dir Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    if [ -d "$SAMPLES_DIR/$project" ]; then
        ((TEST_ID++)) || true
        test_tool_for_language "$lang" "list_dir" ',"arguments":{"relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_list_dir_response.json"
    else
        log_warning "$lang project not found, skipping list_dir test"
    fi
done

# Test find_file for all languages
log_section "Test 11b: find_file Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    if [ -d "$SAMPLES_DIR/$project" ]; then
        ((TEST_ID++)) || true
        case $lang in
            "Go")
                test_tool_for_language "$lang" "find_file" ',"arguments":{"query":"*.go","relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_find_file_response.json"
                ;;
            "Java")
                test_tool_for_language "$lang" "find_file" ',"arguments":{"query":"*.java","relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_find_file_response.json"
                ;;
            "JavaScript")
                test_tool_for_language "$lang" "find_file" ',"arguments":{"query":"*.js","relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_find_file_response.json"
                ;;
            "Python")
                test_tool_for_language "$lang" "find_file" ',"arguments":{"query":"*.py","relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_find_file_response.json"
                ;;
        esac
    else
        log_warning "$lang project not found, skipping find_file test"
    fi
done

# Test search_for_pattern for all languages
log_section "Test 11c: search_for_pattern Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    if [ -d "$SAMPLES_DIR/$project" ]; then
        ((TEST_ID++)) || true
        test_tool_for_language "$lang" "search_for_pattern" ',"arguments":{"pattern":"Calculator","relative_path":"'$project'"}' "$project" $TEST_ID "${lang}_search_pattern_response.json"
    else
        log_warning "$lang project not found, skipping search_for_pattern test"
    fi
done

# Test find_referencing_symbols for all languages
log_section "Test 11d: find_referencing_symbols Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    file="$(get_language_file "$lang")"
    if [ -f "$SAMPLES_DIR/$file" ]; then
        ((TEST_ID++)) || true
        test_tool_for_language "$lang" "find_referencing_symbols" ',"arguments":{"symbol_name":"Calculator","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_find_refs_response.json"
    else
        log_warning "$lang file not found, skipping find_referencing_symbols test"
    fi
done

# Test replace_symbol_body for all languages
log_section "Test 11e: replace_symbol_body Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    file="$(get_language_file "$lang")"
    if [ -f "$SAMPLES_DIR/$file" ]; then
        ((TEST_ID++)) || true
        # Using a simple replacement that should work for all languages
        case $lang in
            "Go")
                test_tool_for_language "$lang" "replace_symbol_body" ',"arguments":{"symbol_name":"Add","new_body":"return a + b + 0","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_replace_body_response.json"
                ;;
            "Java")
                test_tool_for_language "$lang" "replace_symbol_body" ',"arguments":{"symbol_name":"add","new_body":"return a + b + 0;","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_replace_body_response.json"
                ;;
            "JavaScript")
                test_tool_for_language "$lang" "replace_symbol_body" ',"arguments":{"symbol_name":"add","new_body":"return a + b + 0;","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_replace_body_response.json"
                ;;
            "Python")
                test_tool_for_language "$lang" "replace_symbol_body" ',"arguments":{"symbol_name":"add","new_body":"return a + b + 0","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_replace_body_response.json"
                ;;
        esac
    else
        log_warning "$lang file not found, skipping replace_symbol_body test"
    fi
done

# Test insert_after_symbol for all languages
log_section "Test 11f: insert_after_symbol Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    file="$(get_language_file "$lang")"
    if [ -f "$SAMPLES_DIR/$file" ]; then
        ((TEST_ID++)) || true
        case $lang in
            "Go")
                test_tool_for_language "$lang" "insert_after_symbol" ',"arguments":{"symbol_name":"Add","code":"// Test comment","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_after_response.json"
                ;;
            "Java")
                test_tool_for_language "$lang" "insert_after_symbol" ',"arguments":{"symbol_name":"add","code":"// Test comment","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_after_response.json"
                ;;
            "JavaScript")
                test_tool_for_language "$lang" "insert_after_symbol" ',"arguments":{"symbol_name":"add","code":"// Test comment","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_after_response.json"
                ;;
            "Python")
                test_tool_for_language "$lang" "insert_after_symbol" ',"arguments":{"symbol_name":"add","code":"# Test comment","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_after_response.json"
                ;;
        esac
    else
        log_warning "$lang file not found, skipping insert_after_symbol test"
    fi
done

# Test insert_before_symbol for all languages
log_section "Test 11g: insert_before_symbol Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    file="$(get_language_file "$lang")"
    if [ -f "$SAMPLES_DIR/$file" ]; then
        ((TEST_ID++)) || true
        case $lang in
            "Go")
                test_tool_for_language "$lang" "insert_before_symbol" ',"arguments":{"symbol_name":"Multiply","code":"// Before multiply","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_before_response.json"
                ;;
            "Java")
                test_tool_for_language "$lang" "insert_before_symbol" ',"arguments":{"symbol_name":"multiply","code":"// Before multiply","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_before_response.json"
                ;;
            "JavaScript")
                test_tool_for_language "$lang" "insert_before_symbol" ',"arguments":{"symbol_name":"multiply","code":"// Before multiply","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_before_response.json"
                ;;
            "Python")
                test_tool_for_language "$lang" "insert_before_symbol" ',"arguments":{"symbol_name":"multiply","code":"# Before multiply","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_insert_before_response.json"
                ;;
        esac
    else
        log_warning "$lang file not found, skipping insert_before_symbol test"
    fi
done

# Test rename_symbol for all languages
log_section "Test 11h: rename_symbol Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    file="$(get_language_file "$lang")"
    if [ -f "$SAMPLES_DIR/$file" ]; then
        ((TEST_ID++)) || true
        case $lang in
            "Go")
                test_tool_for_language "$lang" "rename_symbol" ',"arguments":{"old_name":"Add","new_name":"AddNumbers","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_rename_symbol_response.json"
                ;;
            "Java")
                test_tool_for_language "$lang" "rename_symbol" ',"arguments":{"old_name":"add","new_name":"addNumbers","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_rename_symbol_response.json"
                ;;
            "JavaScript")
                test_tool_for_language "$lang" "rename_symbol" ',"arguments":{"old_name":"add","new_name":"addNumbers","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_rename_symbol_response.json"
                ;;
            "Python")
                test_tool_for_language "$lang" "rename_symbol" ',"arguments":{"old_name":"add","new_name":"add_numbers","relative_path":"'$file'"}' "$project" $TEST_ID "${lang}_rename_symbol_response.json"
                ;;
        esac
    else
        log_warning "$lang file not found, skipping rename_symbol test"
    fi
done

# Test 12: Memory Operations (language-independent)
log_section "Test 12: Memory Operations"

# Test write_memory
count_test
log_info "Test 12a: write_memory..."
((TEST_ID++)) || true
WRITE_MEM_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"write_memory","arguments":{"key":"test_key","value":"test_value","tags":["test"]}}}'

WRITE_MEM_RESPONSE=$(echo "$WRITE_MEM_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$WRITE_MEM_RESPONSE" > "$RESULTS_DIR/write_memory_response.json"

if echo "$WRITE_MEM_RESPONSE" | grep -q '"result"'; then
    log_success "write_memory completed successfully"
else
    log_error "write_memory failed"
fi

# Test read_memory
count_test
log_info "Test 12b: read_memory..."
((TEST_ID++)) || true
READ_MEM_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"read_memory","arguments":{"key":"test_key"}}}'

READ_MEM_RESPONSE=$(echo "$READ_MEM_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$READ_MEM_RESPONSE" > "$RESULTS_DIR/read_memory_response.json"

if echo "$READ_MEM_RESPONSE" | grep -q '"result"'; then
    log_success "read_memory completed successfully"
else
    log_error "read_memory failed"
fi

# Test list_memories
count_test
log_info "Test 12c: list_memories..."
((TEST_ID++)) || true
LIST_MEM_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"list_memories","arguments":{}}}'

LIST_MEM_RESPONSE=$(echo "$LIST_MEM_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$LIST_MEM_RESPONSE" > "$RESULTS_DIR/list_memories_response.json"

if echo "$LIST_MEM_RESPONSE" | grep -q '"result"'; then
    log_success "list_memories completed successfully"
else
    log_error "list_memories failed"
fi

# Test edit_memory
count_test
log_info "Test 12d: edit_memory..."
((TEST_ID++)) || true
EDIT_MEM_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"edit_memory","arguments":{"key":"test_key","value":"updated_value"}}}'

EDIT_MEM_RESPONSE=$(echo "$EDIT_MEM_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$EDIT_MEM_RESPONSE" > "$RESULTS_DIR/edit_memory_response.json"

if echo "$EDIT_MEM_RESPONSE" | grep -q '"result"'; then
    log_success "edit_memory completed successfully"
else
    log_error "edit_memory failed"
fi

# Test delete_memory
count_test
log_info "Test 12e: delete_memory..."
((TEST_ID++)) || true
DELETE_MEM_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"delete_memory","arguments":{"key":"test_key"}}}'

DELETE_MEM_RESPONSE=$(echo "$DELETE_MEM_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$DELETE_MEM_RESPONSE" > "$RESULTS_DIR/delete_memory_response.json"

if echo "$DELETE_MEM_RESPONSE" | grep -q '"result"'; then
    log_success "delete_memory completed successfully"
else
    log_error "delete_memory failed"
fi

# Test 13: Configuration and Project Management
log_section "Test 13: Configuration and Project Management"

# Test activate_project for each language
log_section "Test 13a: activate_project Tool"
for lang in "Go" "Java" "JavaScript" "Python"; do
    project="$(get_language_project "$lang")"
    if [ -d "$SAMPLES_DIR/$project" ]; then
        count_test
        log_info "Testing activate_project for ${lang}..."
        ((TEST_ID++)) || true
        
        ACTIVATE_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"activate_project","arguments":{"relative_path":"'$project'"}}}'
        
        ACTIVATE_RESPONSE=$(echo "$ACTIVATE_REQUEST" | docker run --rm -i \
            -v "$SAMPLES_DIR:/workspace:ro" \
            -w /workspace \
            "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
        
        echo "$ACTIVATE_RESPONSE" > "$RESULTS_DIR/${lang}_activate_project_response.json"
        
        if echo "$ACTIVATE_RESPONSE" | grep -q '"result"'; then
            log_success "${lang} activate_project completed successfully"
        else
            log_error "${lang} activate_project failed"
        fi
    else
        log_warning "$lang project not found, skipping activate_project test"
    fi
done

# Test get_current_config
count_test
log_info "Test 13b: get_current_config..."
((TEST_ID++)) || true
CONFIG_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"get_current_config","arguments":{}}}'

CONFIG_RESPONSE=$(echo "$CONFIG_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$CONFIG_RESPONSE" > "$RESULTS_DIR/get_current_config_response.json"

if echo "$CONFIG_RESPONSE" | grep -q '"result"'; then
    log_success "get_current_config completed successfully"
else
    log_error "get_current_config failed"
fi

# Test 14: Onboarding Operations
log_section "Test 14: Onboarding Operations"

# Test check_onboarding_performed
count_test
log_info "Test 14a: check_onboarding_performed..."
((TEST_ID++)) || true
CHECK_ONBOARD_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"check_onboarding_performed","arguments":{}}}'

CHECK_ONBOARD_RESPONSE=$(echo "$CHECK_ONBOARD_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$CHECK_ONBOARD_RESPONSE" > "$RESULTS_DIR/check_onboarding_response.json"

if echo "$CHECK_ONBOARD_RESPONSE" | grep -q '"result"'; then
    log_success "check_onboarding_performed completed successfully"
else
    log_error "check_onboarding_performed failed"
fi

# Test onboarding
count_test
log_info "Test 14b: onboarding..."
((TEST_ID++)) || true
ONBOARD_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"onboarding","arguments":{}}}'

ONBOARD_RESPONSE=$(echo "$ONBOARD_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$ONBOARD_RESPONSE" > "$RESULTS_DIR/onboarding_response.json"

if echo "$ONBOARD_RESPONSE" | grep -q '"result"'; then
    log_success "onboarding completed successfully"
else
    log_error "onboarding failed"
fi

# Test 15: Thinking Operations
log_section "Test 15: Thinking Operations"

# Test think_about_collected_information
count_test
log_info "Test 15a: think_about_collected_information..."
((TEST_ID++)) || true
THINK_INFO_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"think_about_collected_information","arguments":{"information":"Test information"}}}'

THINK_INFO_RESPONSE=$(echo "$THINK_INFO_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$THINK_INFO_RESPONSE" > "$RESULTS_DIR/think_info_response.json"

if echo "$THINK_INFO_RESPONSE" | grep -q '"result"'; then
    log_success "think_about_collected_information completed successfully"
else
    log_error "think_about_collected_information failed"
fi

# Test think_about_task_adherence
count_test
log_info "Test 15b: think_about_task_adherence..."
((TEST_ID++)) || true
THINK_TASK_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"think_about_task_adherence","arguments":{"task":"Test task"}}}'

THINK_TASK_RESPONSE=$(echo "$THINK_TASK_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$THINK_TASK_RESPONSE" > "$RESULTS_DIR/think_task_response.json"

if echo "$THINK_TASK_RESPONSE" | grep -q '"result"'; then
    log_success "think_about_task_adherence completed successfully"
else
    log_error "think_about_task_adherence failed"
fi

# Test think_about_whether_you_are_done
count_test
log_info "Test 15c: think_about_whether_you_are_done..."
((TEST_ID++)) || true
THINK_DONE_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"think_about_whether_you_are_done","arguments":{}}}'

THINK_DONE_RESPONSE=$(echo "$THINK_DONE_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$THINK_DONE_RESPONSE" > "$RESULTS_DIR/think_done_response.json"

if echo "$THINK_DONE_RESPONSE" | grep -q '"result"'; then
    log_success "think_about_whether_you_are_done completed successfully"
else
    log_error "think_about_whether_you_are_done failed"
fi

# Test 16: Initial Instructions
log_section "Test 16: Initial Instructions"

count_test
log_info "Test 16a: initial_instructions..."
((TEST_ID++)) || true
INITIAL_INSTR_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":'$TEST_ID',"method":"tools/call","params":{"name":"initial_instructions","arguments":{}}}'

INITIAL_INSTR_RESPONSE=$(echo "$INITIAL_INSTR_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    -w /workspace \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$INITIAL_INSTR_RESPONSE" > "$RESULTS_DIR/initial_instructions_response.json"

if echo "$INITIAL_INSTR_RESPONSE" | grep -q '"result"'; then
    log_success "initial_instructions completed successfully"
else
    log_error "initial_instructions failed"
fi

# Test 17: Error Handling - Invalid Request
log_section "Test 17: Error Handling Tests"
count_test
log_info "Test 17a: Testing invalid MCP request..."

INVALID_REQUEST='{"jsonrpc":"2.0","id":99,"method":"invalid_method","params":{}}'

INVALID_RESPONSE=$(echo "$INVALID_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null || echo '{"error": "failed"}')

echo "$INVALID_RESPONSE" > "$RESULTS_DIR/invalid_request_response.json"

if echo "$INVALID_RESPONSE" | grep -q '"error"'; then
    log_success "Invalid request properly rejected with error response"
else
    log_warning "Invalid request handling unclear"
fi

# Test 17b: Malformed JSON
count_test
log_info "Test 17b: Testing malformed JSON..."

MALFORMED_REQUEST='{"jsonrpc":"2.0","id":100,"method":"initialize"'

MALFORMED_RESPONSE=$(echo "$MALFORMED_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>&1 || echo "error")

echo "$MALFORMED_RESPONSE" > "$RESULTS_DIR/malformed_json_response.txt"

if echo "$MALFORMED_RESPONSE" | grep -q -i "error\|invalid\|parse"; then
    log_success "Malformed JSON properly rejected"
else
    log_warning "Malformed JSON handling unclear"
fi

# Test 18: Container Size Check
log_section "Test 18: Container Metrics"
count_test
log_info "Checking container size..."

SIZE=$(docker images "$CONTAINER_IMAGE" --format "{{.Size}}" 2>/dev/null || echo "unknown")
log_info "Container size: $SIZE"

if [ "$SIZE" != "unknown" ]; then
    log_success "Container size information retrieved"
else
    log_warning "Could not retrieve container size"
fi

# Summary
log_section "Test Summary"
echo ""
log_info "Total Tests: $TESTS_TOTAL"
log_success "Passed: $TESTS_PASSED"
if [ $TESTS_FAILED -gt 0 ]; then
    log_error "Failed: $TESTS_FAILED"
fi
echo ""

# Calculate success rate
if [ $TESTS_TOTAL -gt 0 ]; then
    SUCCESS_RATE=$((TESTS_PASSED * 100 / TESTS_TOTAL))
    log_info "Success Rate: $SUCCESS_RATE%"
else
    log_warning "No tests were run"
fi
echo ""

log_info "Detailed results saved to: $RESULTS_DIR"
echo ""

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    log_warning "Some tests failed. Review the output above for details."
    exit 1
else
    log_success "All tests passed!"
    exit 0
fi
