# Quick Start Guide - Serena MCP Server Tests

## Running the Tests

### Prerequisites
1. Docker installed and running
2. Network access to pull `ghcr.io/github/serena-mcp-server:latest`

### Run All Tests

From the repository root:
```bash
./test/serena-mcp-tests/test_serena.sh
```

Or from the test directory:
```bash
cd test/serena-mcp-tests
./test_serena.sh
```

### Test with Local Image

If you've built the Serena MCP server locally:
```bash
SERENA_IMAGE="serena-mcp-server:local" ./test/serena-mcp-tests/test_serena.sh
```

## Expected Output

### Successful Run
```
========================================
Serena MCP Server Comprehensive Test Suite
========================================
[INFO] Container Image: ghcr.io/github/serena-mcp-server:latest
[INFO] Test Directory: /path/to/test/serena-mcp-tests
[INFO] Samples Directory: /path/to/test/serena-mcp-tests/samples

========================================
Test 1: Docker Availability
========================================
[✓] Docker is installed

========================================
Test 5: MCP Protocol Initialize
========================================
[INFO] Sending MCP initialize request...
[✓] MCP initialize succeeded
[INFO] Response saved to: results/initialize_response.json

... (more tests) ...

========================================
Test Summary
========================================

[INFO] Total Tests: 20
[✓] Passed: 20
[INFO] Success Rate: 100%
[INFO] Detailed results saved to: results/

[✓] All tests passed!
```

## What Gets Tested

1. **Infrastructure** (3 tests)
   - Docker availability
   - Container image availability  
   - Container basic functionality

2. **Language Runtimes** (4 tests)
   - Python 3.11+
   - Java JDK 21
   - Node.js
   - Go

3. **MCP Protocol** (2 tests)
   - Initialize connection
   - List available tools

4. **Go Analysis** (2 tests)
   - Symbol finding
   - Code diagnostics

5. **Java Analysis** (2 tests)
   - Symbol finding
   - Code diagnostics

6. **JavaScript Analysis** (2 tests)
   - Symbol finding
   - Code diagnostics

7. **Python Analysis** (2 tests)
   - Symbol finding
   - Code diagnostics

8. **Error Handling** (2 tests)
   - Invalid requests
   - Malformed JSON

9. **Container Metrics** (1 test)
   - Container size check

## Test Results Location

All MCP server responses are saved in `test/serena-mcp-tests/results/`:
- `initialize_response.json`
- `tools_list_response.json`
- `go_symbols_response.json`
- `go_diagnostics_response.json`
- `java_symbols_response.json`
- `java_diagnostics_response.json`
- `js_symbols_response.json`
- `js_diagnostics_response.json`
- `python_symbols_response.json`
- `python_diagnostics_response.json`
- `invalid_request_response.json`
- `malformed_json_response.txt`

## Troubleshooting

### "Docker is not installed"
Install Docker: https://docs.docker.com/get-docker/

### "Failed to pull container image"
- Check network connectivity
- Verify you have access to the GitHub Container Registry
- Try pulling manually: `docker pull ghcr.io/github/serena-mcp-server:latest`

### Tests timeout or hang
- Increase Docker resource limits (CPU/Memory)
- First run may be slow due to language server initialization
- Check Docker daemon health: `docker ps`

### Specific language tests fail
- Check the saved JSON responses in `results/` directory
- Verify the sample code exists in `samples/` directory
- Review container logs: `docker logs <container-id>`

## Understanding Test Output

| Symbol | Meaning |
|--------|---------|
| `[✓]` | Test passed |
| `[✗]` | Test failed |
| `[⚠]` | Warning - test completed with unexpected result |
| `[INFO]` | Informational message |

## Next Steps

After running tests:
1. Review the summary statistics
2. Check failed tests in the output
3. Examine JSON response files in `results/` directory
4. For failures, review the troubleshooting section in README.md

For detailed documentation, see [README.md](README.md).
