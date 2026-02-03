# Serena MCP Server Test Suite

Comprehensive shell script tests for the Serena MCP Server (`ghcr.io/github/serena-mcp-server:latest`), covering all 23 MCP tools across 4 programming languages.

## Quick Start

### Run Tests with Make

The easiest way to run the tests:

```bash
# Test Serena with direct connection
make test-serena

# Test Serena through MCP Gateway
make test-serena-gateway
```

### Run Tests Directly

From the repository root:
```bash
# Direct connection tests
./test/serena-mcp-tests/test_serena.sh

# Gateway tests
./test/serena-mcp-tests/test_serena_via_gateway.sh
```

Or from this directory:
```bash
cd test/serena-mcp-tests

# Direct connection tests
./test_serena.sh

# Gateway tests
./test_serena_via_gateway.sh
```

## Test Suites

### 1. Direct Connection Tests (`test_serena.sh`)

These tests connect directly to the Serena MCP Server container via stdio (standard input/output). This validates the core functionality of Serena without any intermediary components.

- **Connection Method**: Direct stdio connection to Docker container
- **Results Directory**: `results/`
- **Use Case**: Testing Serena's core MCP implementation

### 2. Gateway Connection Tests (`test_serena_via_gateway.sh`)

These tests connect to Serena through the MCP Gateway container, which proxies requests to the backend Serena server. This validates that Serena works correctly when accessed through the gateway infrastructure.

- **Connection Method**: HTTP requests to MCP Gateway → Gateway proxies to Serena via stdio
- **Gateway Image**: `ghcr.io/github/gh-aw-mcpg:latest`
- **Results Directory**: `results-gateway/`
- **Use Case**: Testing Serena through production gateway setup
- **Purpose**: Identify any behavioral differences when using the gateway

### Comparing Results

Both test suites run the same test scenarios, allowing you to compare results and identify any differences in behavior:

```bash
# Run both test suites
make test-serena
make test-serena-gateway

# Compare results
diff -r test/serena-mcp-tests/results/ test/serena-mcp-tests/results-gateway/
```

**Important**: See [`GATEWAY_TEST_FINDINGS.md`](GATEWAY_TEST_FINDINGS.md) for documented behavioral differences between direct stdio connections and HTTP-proxied connections through the gateway. The stdio-based Serena server requires connection-level session state that is not currently maintained across independent HTTP requests.

## Overview

This test suite validates that the Serena MCP Server correctly supports multiple programming languages (Go, Java, JavaScript, and Python) through the Model Context Protocol (MCP). The tests comprehensively cover all 23 available tools including file operations, symbol analysis, memory management, configuration, onboarding, thinking operations, and instructions.

## Features

- **Multi-language Testing**: Tests Go, Java, JavaScript, and Python support
- **Comprehensive Tool Coverage**: Tests all 23 MCP tools provided by Serena
- **MCP Protocol Validation**: Tests initialization, tool listing, and tool invocation
- **Code Analysis Tests**: Validates symbol finding, references, refactoring, and semantic analysis
- **Memory & Configuration**: Tests memory operations, project configuration, and onboarding
- **Thinking Operations**: Tests AI-powered reasoning and decision-making tools
- **Error Handling**: Tests invalid requests and malformed JSON handling
- **Detailed Reporting**: Color-coded output with pass/fail status and JSON response logs
- **Sample Codebases**: Includes realistic code samples for each language

## Directory Structure

```
test/serena-mcp-tests/
├── README.md                    # This file
├── test_serena.sh              # Main test script
├── samples/                    # Sample codebases for testing
│   ├── go_project/            # Go calculator sample
│   │   ├── main.go
│   │   ├── go.mod
│   │   └── utils.go
│   ├── java_project/          # Java calculator sample
│   │   ├── Calculator.java
│   │   └── Utils.java
│   ├── js_project/            # JavaScript calculator sample
│   │   ├── calculator.js
│   │   ├── utils.js
│   │   └── package.json
│   └── python_project/        # Python calculator sample
│       ├── calculator.py
│       └── utils.py
├── expected/                   # Directory for expected results (future use)
└── results/                   # Generated test results (JSON responses)
```

## Requirements

- **Docker**: The test script requires Docker to be installed and running
- **Bash**: Shell script tested with Bash 4.0+
- **Network Access**: Ability to pull the Serena MCP Server Docker image

## Usage

### Basic Usage

Run the test script from this directory:

```bash
./test_serena.sh
```

### Custom Docker Image

To test a different version or local build:

```bash
SERENA_IMAGE="serena-mcp-server:local" ./test_serena.sh
```

### Running from Repository Root

```bash
./test/serena-mcp-tests/test_serena.sh
```

## Test Categories

### 1. Infrastructure Tests
- Docker availability
- Container image availability
- Container basic functionality

### 2. Runtime Verification
- Python runtime (3.11+)
- Java runtime (JDK 21)
- Node.js runtime
- Go runtime

### 3. MCP Protocol Tests
- Initialize connection
- List available tools
- Tool invocation

### 4. Language-Specific Analysis (All 4 Languages: Go, Java, JavaScript, Python)

#### File Operations
- `list_dir` - List directory contents
- `find_file` - Find files by pattern
- `search_for_pattern` - Search for text patterns in code

#### Symbol Operations
- `get_symbols_overview` - Get overview of symbols in a file
- `find_symbol` - Find specific symbol definitions
- `find_referencing_symbols` - Find references to a symbol
- `replace_symbol_body` - Replace symbol implementation
- `insert_after_symbol` - Insert code after a symbol
- `insert_before_symbol` - Insert code before a symbol
- `rename_symbol` - Rename symbols with refactoring

#### Configuration & Project Management
- `activate_project` - Activate a project for analysis
- `get_current_config` - Get current configuration

### 5. Memory Operations (Language-Independent)
- `write_memory` - Store information in memory
- `read_memory` - Retrieve stored information
- `list_memories` - List all stored memories
- `edit_memory` - Update stored information
- `delete_memory` - Remove stored information

### 6. Onboarding Operations (Language-Independent)
- `check_onboarding_performed` - Check if onboarding is complete
- `onboarding` - Perform onboarding process

### 7. Thinking Operations (Language-Independent)
- `think_about_collected_information` - Process collected information
- `think_about_task_adherence` - Evaluate task adherence
- `think_about_whether_you_are_done` - Assess completion status

### 8. Instructions (Language-Independent)
- `initial_instructions` - Get initial instructions

### 9. Error Handling
- Invalid MCP method requests
- Malformed JSON handling
- Proper error responses

## Sample Codebases

Each sample codebase contains a simple Calculator implementation with:
- Basic arithmetic operations (add, multiply)
- Properly documented code
- Utility functions for testing cross-file references
- Language-specific idioms and patterns

These samples provide:
- **Known symbols**: Functions/methods with predictable names for validation
- **Type information**: Proper typing for language server analysis
- **Documentation**: Comments and docstrings for documentation tests
- **Realistic structure**: Multiple files to test cross-file analysis

## Test Output

The script provides:
- **Color-coded results**: Green for pass, red for fail, yellow for warnings
- **Progress indicators**: Real-time test execution feedback
- **JSON response logs**: All MCP responses saved to `results/` directory
- **Summary statistics**: Total tests, pass/fail counts, success rate

### Example Output

```
========================================
Test 5: MCP Protocol Initialize
========================================
[INFO] Sending MCP initialize request...
[✓] MCP initialize succeeded
[INFO] Response saved to: results/initialize_response.json

========================================
Test Summary
========================================

[INFO] Total Tests: 35
[✓] Passed: 32
[✗] Failed: 3

[INFO] Success Rate: 91%
[INFO] Detailed results saved to: results/
```

## Response Files

All MCP responses are saved to `results/` directory with descriptive names:

### Protocol & Infrastructure
- `initialize_response.json` - MCP initialization response
- `tools_list_response.json` - Available tools listing
- `invalid_request_response.json` - Error handling test results
- `malformed_json_response.txt` - Malformed JSON test results

### Language-Specific Tool Results (for each language: Go, Java, JavaScript, Python)
- `{language}_list_dir_response.json` - Directory listing results
- `{language}_find_file_response.json` - File finding results
- `{language}_search_pattern_response.json` - Pattern search results
- `{language}_symbols_response.json` - Symbol overview results
- `{language}_find_symbol_response.json` - Symbol finding results
- `{language}_find_refs_response.json` - Symbol references results
- `{language}_replace_body_response.json` - Symbol body replacement results
- `{language}_insert_after_response.json` - Insert after symbol results
- `{language}_insert_before_response.json` - Insert before symbol results
- `{language}_rename_symbol_response.json` - Symbol renaming results
- `{language}_activate_project_response.json` - Project activation results

### Memory Operations
- `write_memory_response.json` - Memory write results
- `read_memory_response.json` - Memory read results
- `list_memories_response.json` - Memory listing results
- `edit_memory_response.json` - Memory edit results
- `delete_memory_response.json` - Memory deletion results

### Configuration & Management
- `get_current_config_response.json` - Configuration retrieval results

### Onboarding
- `check_onboarding_response.json` - Onboarding status check results
- `onboarding_response.json` - Onboarding process results

### Thinking Operations
- `think_info_response.json` - Information processing results
- `think_task_response.json` - Task adherence evaluation results
- `think_done_response.json` - Completion assessment results

### Instructions
- `initial_instructions_response.json` - Initial instructions results

## Interpreting Results

### Successful Test
```
[✓] Go symbol analysis working - found expected symbols
```

### Failed Test
```
[✗] Go symbol analysis failed
```

### Warning
```
[⚠] Go symbol analysis returned result but symbols not as expected
```

## Troubleshooting

### Container Pull Fails
- Verify network connectivity
- Check Docker registry access
- Ensure proper authentication for private registries

### All Language Tests Fail
- Check that sample files exist in `samples/` directory
- Verify workspace is properly mounted (`/workspace`)
- Review container logs for language server initialization errors

### Specific Language Fails
- Verify the language runtime is installed in the container
- Check that language-specific tools are in the tools list
- Review the response JSON files in `results/` for error details

### Tests Hang
- Some language servers may take time to initialize on first run
- Container may need more resources (CPU/memory)
- Check Docker daemon health

## Extending the Tests

To add new tests:

1. Add new sample code to `samples/` directory
2. Add test cases to `test_serena.sh` following the existing pattern
3. Document expected behavior in `expected/` directory (optional)
4. Update this README with new test descriptions

## CI/CD Integration

The test script returns:
- Exit code `0` on success (all tests pass)
- Exit code `1` on failure (one or more tests fail)

Example GitHub Actions usage:

```yaml
- name: Run Serena MCP Tests
  run: |
    cd test/serena-mcp-tests
    ./test_serena.sh
```

## Known Limitations

- Tests require Docker (no native binary testing)
- First run may be slow due to language server initialization
- Some language servers cache data - results may vary between runs
- Tool names and arguments depend on Serena MCP Server version

## References

- [Serena MCP Server Documentation](https://github.com/oraios/serena)
- [Model Context Protocol Specification](https://github.com/modelcontextprotocol)
- [MCP Gateway Configuration](https://github.com/github/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)

## Contributing

To improve these tests:
1. Add more comprehensive sample codebases
2. Add expected response validation
3. Add performance benchmarks
4. Test additional language features (refactoring, code completion, etc.)
5. Add integration tests with the MCP Gateway

## License

Part of the gh-aw-mcpg project. See repository root for license information.
