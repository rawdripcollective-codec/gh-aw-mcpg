<!-- This prompt will be imported in the agentic workflow .github/workflows/nightly-mcp-stress-test.md at runtime. -->
<!-- You can edit this file to modify the agent behavior without recompiling the workflow. -->

# Nightly MCP Server Stress Test 🧪

You are an AI agent that performs comprehensive stress testing of the MCP Gateway by loading and testing 20 well-known MCP servers, executing their tools, and reporting the results.

## Mission

Test the MCP Gateway's ability to handle multiple diverse MCP servers simultaneously. For each server, attempt to discover and invoke at least one tool. Track successes, failures, and categorize issues (authentication, protocol, timeout, etc.).

## Step 1: Prepare Test Configuration 📋

Create a comprehensive test configuration that includes 20 well-known MCP servers:

### Server List

Configure the following MCP servers in the test:

1. **GitHub MCP Server** (ghcr.io/github/github-mcp-server:v0.30.2)
2. **Filesystem MCP Server** (mcp/filesystem)
3. **Memory MCP Server** (mcp/memory)
4. **SQLite MCP Server** (mcp/sqlite)
5. **Postgres MCP Server** (mcp/postgres)
6. **Brave Search MCP Server** (mcp/brave-search)
7. **Fetch MCP Server** (mcp/fetch)
8. **Puppeteer MCP Server** (mcp/puppeteer)
9. **Slack MCP Server** (mcp/slack)
10. **Google Drive MCP Server** (mcp/gdrive)
11. **Google Maps MCP Server** (mcp/google-maps)
12. **EverArt MCP Server** (mcp/everart)
13. **Sequential Thinking MCP Server** (mcp/sequential-thinking)
14. **AWS KB Retrieval MCP Server** (mcp/aws-kb-retrieval)
15. **Linear MCP Server** (mcp/linear)
16. **Sentry MCP Server** (mcp/sentry)
17. **Raygun MCP Server** (mcp/raygun)
18. **Git MCP Server** (mcp/git)
19. **Time MCP Server** (mcp/time)
20. **Axiom MCP Server** (mcp/axiom)

### Test Configuration Structure

Create a test configuration file at `/tmp/mcp-stress-test-config.json`. The agent should generate the actual JSON file dynamically using environment variables:

Example structure (the agent will create the actual file with the API_KEY variable):

```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:v0.30.2",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}"
      }
    },
    "filesystem": {
      "type": "stdio",
      "container": "mcp/filesystem",
      "mounts": [
        {
          "source": "/tmp",
          "target": "/workspace",
          "readOnly": false
        }
      ]
    },
    "memory": {
      "type": "stdio",
      "container": "mcp/memory"
    }
    // ... add remaining 17 servers
  },
  "gateway": {
    "port": 3000,
    "apiKey": "${API_KEY}",
    "startupTimeout": 60,
    "toolTimeout": 30
  }
}
```

**Note:** Include placeholders for authentication tokens. Document which servers require authentication.

## Step 2: Start MCP Gateway with Test Configuration 🚀

1. **Generate a unique test session ID:**
   ```bash
   TEST_SESSION="stress-test-$(date +%Y%m%d-%H%M%S)"
   echo "Test session: $TEST_SESSION"
   ```

2. **Set up test environment:**
   ```bash
   # Create test directories
   mkdir -p /tmp/mcp-stress-test
   mkdir -p /tmp/mcp-stress-test/logs
   
   # Export required environment variables
   # Note: GITHUB_TOKEN is automatically available in the workflow environment
   export GITHUB_TOKEN="${GITHUB_TOKEN}"
   
   # Generate secure API key for this test session (remove problematic characters)
   export API_KEY="stress-test-$(openssl rand -base64 45 | tr -d '/+=')"
   ```

3. **Build and start the gateway:**
   ```bash
   cd /home/runner/work/gh-aw-mcpg/gh-aw-mcpg
   make build
   
   # Start gateway in background with test config
   ./awmg --config /tmp/mcp-stress-test-config.json \
          --log-dir /tmp/mcp-stress-test/logs \
          2>&1 | tee /tmp/mcp-stress-test/gateway-startup.log &
   
   GATEWAY_PID=$!
   echo "Gateway PID: $GATEWAY_PID"
   
   # Wait for gateway to be ready
   sleep 10
   ```

4. **Verify gateway is running:**
   ```bash
   curl -f http://localhost:3000/health || echo "Gateway health check failed"
   ```

## Step 3: Test Each MCP Server 🔬

For each configured MCP server, perform the following tests:

### 3.1 Discover Available Tools

1. **Call `tools/list` for each server:**
   ```bash
   # Note: Per MCP spec 7.1, Authorization header contains API key directly (no "Bearer" prefix)
   curl -X POST http://localhost:3000/mcp/{server-name} \
     -H "Authorization: ${API_KEY}" \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "id": 1,
       "method": "tools/list",
       "params": {}
     }'
   ```

2. **Parse the response:**
   - Extract list of available tools
   - Record tool names and schemas
   - Note any errors (authentication, timeout, protocol)

### 3.2 Invoke Test Tools

For each server with available tools:

1. **Select a simple tool to test:**
   - Prefer read-only operations (list, get, search)
   - Avoid destructive operations (create, delete, update)
   - Choose tools that don't require complex parameters

2. **Invoke the selected tool:**
   ```bash
   # Note: Per MCP spec 7.1, Authorization header contains API key directly (no "Bearer" prefix)
   curl -X POST http://localhost:3000/mcp/{server-name} \
     -H "Authorization: ${API_KEY}" \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "id": 2,
       "method": "tools/call",
       "params": {
         "name": "{tool-name}",
         "arguments": {
           // minimal required arguments
         }
       }
     }'
   ```

3. **Record the result:**
   - Success: Tool executed without error
   - Authentication Failure: 401 or authentication-related error
   - Timeout: Request timed out
   - Protocol Error: JSON-RPC or MCP protocol violation
   - Other Error: Any other failure type

### 3.3 Categorize Failures

For each failure, determine the root cause:

**Authentication Required:**
- Error message contains "authentication", "unauthorized", "token", "API key"
- HTTP 401 status code
- Tool invocation fails due to missing credentials

**Protocol Error:**
- Invalid JSON-RPC response
- MCP protocol violation
- Malformed request/response

**Timeout:**
- Request exceeds toolTimeout (30 seconds)
- Server unresponsive
- Container startup timeout

**Container Error:**
- Docker container failed to start
- Container image not found
- Container crashed during execution

**Other:**
- Unexpected errors
- Network issues
- Gateway internal errors

## Step 4: Collect Gateway Logs 📝

After testing all servers:

1. **Stop the gateway gracefully:**
   ```bash
   kill -TERM $GATEWAY_PID
   wait $GATEWAY_PID
   ```

2. **Collect log files:**
   ```bash
   # Collect all logs
   cp -r /tmp/mcp-stress-test/logs /tmp/mcp-stress-results/
   
   # Parse for errors
   grep -i error /tmp/mcp-stress-test/logs/*.log > /tmp/mcp-stress-results/errors.txt
   ```

3. **Analyze gateway performance:**
   - Check for memory leaks
   - Measure startup time for each server
   - Count total requests and failures
   - Identify slowest servers

## Step 5: Generate Test Report 📊

Create a comprehensive test report with the following sections:

### Summary Statistics

```markdown
## Test Summary

- **Test Session:** $TEST_SESSION
- **Date:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
- **Total Servers Tested:** 20
- **Successful Servers:** X
- **Failed Servers:** Y
- **Authentication Required:** Z

### Success Rate

- Overall: X% (X/20 servers)
- With Authentication: Y% (Y/Z authenticated servers)
- Without Authentication: Z% (Z/W non-authenticated servers)
```

### Server Results Table

```markdown
| Server Name | Status | Tools Found | Test Tool | Result | Issue Type | Notes |
|-------------|--------|-------------|-----------|--------|------------|-------|
| github | ✅ Success | 25 | search_repositories | ✅ | - | All tests passed |
| slack | ❌ Failed | 0 | - | Auth Required | Authentication | Needs SLACK_TOKEN |
| filesystem | ✅ Success | 5 | list_directory | ✅ | - | All tests passed |
| ... | ... | ... | ... | ... | ... | ... |
```

### Detailed Failure Analysis

For each failed server, include:

```markdown
#### Server: {server-name}

**Status:** ❌ Failed

**Issue Type:** Authentication Required

**Error Message:**
```
Error: Missing or invalid SLACK_BOT_TOKEN environment variable
```

**Required Authentication:**
- Environment Variable: `SLACK_BOT_TOKEN`
- Type: OAuth Bot Token
- Documentation: https://api.slack.com/authentication/token-types

**Suggested Configuration:**
```json
{
  "slack": {
    "type": "stdio",
    "container": "mcp/slack",
    "env": {
      "SLACK_BOT_TOKEN": "${SLACK_BOT_TOKEN}"
    }
  }
}
```
```

### Performance Metrics

```markdown
## Performance Analysis

- **Gateway Startup Time:** Xs
- **Average Tool Response Time:** Xms
- **Slowest Server:** {server-name} (Xms)
- **Fastest Server:** {server-name} (Xms)
- **Total Test Duration:** Xm Ys
```

### Gateway Stability

```markdown
## Gateway Stability

- **Gateway Crashes:** 0
- **Memory Usage:** Peak XXX MB
- **Container Restarts:** 0
- **Protocol Errors:** X
```

## Step 6: Create GitHub Issues 🎫

Based on the test results, create GitHub issues:

### For Authentication-Required Servers

If any servers failed due to missing authentication:

1. **Create an issue for EACH server** using safe-outputs:

```markdown
Title: [mcp-stress-test] {Server Name} requires {AUTH_TOKEN_NAME} authentication

Body:
# MCP Server Authentication Required: {Server Name}

The nightly stress test detected that the **{Server Name}** MCP server requires authentication to function properly.

## Test Details

- **Test Session:** {TEST_SESSION}
- **Test Date:** {DATE}
- **Server Container:** {container-image}

## Required Authentication Token

**Environment Variable:** `{AUTH_TOKEN_NAME}`

**Token Type:** {token-type} (e.g., API Key, OAuth Token, Service Account)

**How to Obtain:**
1. Visit: {documentation-url}
2. Create an account or sign in
3. Generate {token-type}
4. Add to repository secrets as `{AUTH_TOKEN_NAME}`

## Error Message

```
{error-message-from-test}
```

## Suggested Configuration

Add the following to your MCP Gateway configuration:

```json
{
  "{server-name}": {
    "type": "stdio",
    "container": "{container-image}",
    "env": {
      "{AUTH_TOKEN_NAME}": "${{{AUTH_TOKEN_NAME}}}"
    }
  }
}
```

## Next Steps

- [ ] Obtain {AUTH_TOKEN_NAME} token
- [ ] Add token to repository secrets
- [ ] Update stress test configuration
- [ ] Verify server works in next nightly test

---
*Generated by Nightly MCP Stress Test*
*Test Session: {TEST_SESSION}*
```

### For Other Failures

If there are servers that failed for reasons other than authentication:

1. **Create a SINGLE summary issue** with all non-auth failures:

```markdown
Title: [mcp-stress-test] Server Failures Detected - {DATE}

Body:
# MCP Server Stress Test Failures

The nightly stress test detected {N} servers with non-authentication failures.

## Test Summary

- **Test Session:** {TEST_SESSION}
- **Test Date:** {DATE}
- **Total Failures:** {N}

## Failed Servers

### 1. {Server Name} - Protocol Error

**Container:** {container-image}

**Issue Type:** Protocol Error

**Error:**
```
{error-message}
```

**Analysis:**
{brief-analysis-of-what-went-wrong}

**Suggested Investigation:**
- [ ] Check container logs
- [ ] Verify MCP protocol compatibility
- [ ] Test server outside gateway

---

### 2. {Server Name} - Timeout

**Container:** {container-image}

**Issue Type:** Timeout (exceeded 30s)

**Error:**
```
{error-message}
```

**Analysis:**
{brief-analysis}

**Suggested Investigation:**
- [ ] Increase toolTimeout in configuration
- [ ] Check server performance
- [ ] Verify container resources

---

## Gateway Logs

Key errors from gateway logs:

```
{relevant-gateway-log-excerpts}
```

## Test Configuration

The test used the following configuration:
- Startup Timeout: 60s
- Tool Timeout: 30s
- Port: 3000

## Next Steps

1. Investigate each failure category
2. Apply fixes or configuration changes
3. Re-run stress test to verify

---
*Generated by Nightly MCP Stress Test*
*Test Session: {TEST_SESSION}*

**Full Logs:** See workflow run artifacts
```

## Step 7: Cleanup 🧹

Clean up test artifacts:

```bash
# Stop any remaining processes
pkill -f awmg || true

# Clean up test files (keep logs for artifacts)
rm -f /tmp/mcp-stress-test-config.json
rm -rf /tmp/mcp-stress-test/tmp
```

## Important Guidelines

### Testing Best Practices

1. **Non-Destructive Testing:**
   - Only use read-only operations
   - Don't create/modify/delete real data
   - Use test/sandbox environments when available

2. **Error Handling:**
   - Catch all errors gracefully
   - Continue testing remaining servers on failure
   - Don't let one failure stop entire test

3. **Timing:**
   - Allow adequate startup time for containers
   - Use reasonable timeouts (30s for tools, 60s for startup)
   - Measure and report timing metrics

### Issue Creation Guidelines

1. **Authentication Issues:**
   - Create ONE issue PER server that needs auth
   - Include clear instructions for obtaining tokens
   - Provide documentation links
   - Suggest exact configuration

2. **Other Failures:**
   - Create ONE summary issue for all non-auth failures
   - Group by failure type
   - Include enough detail for debugging
   - Suggest investigation steps

3. **Issue Quality:**
   - Use clear, descriptive titles
   - Include test session ID for traceability
   - Provide complete error messages
   - Add suggested next steps
   - Label appropriately

### Performance Considerations

- Test servers sequentially (not in parallel) to avoid resource contention
- Measure and report timing for each operation
- Monitor gateway memory and CPU usage
- Track container startup and shutdown times

## Expected Workflow Behavior

### Success Case (All Servers Pass)

If ALL 20 servers successfully respond to tools/list and tool invocation:

1. **DO NOT create any issues** (silence is success)
2. **Log summary to workflow output:**
   ```
   ✅ All 20 MCP servers passed stress test
   Test Session: stress-test-20260204-015230
   Total Test Duration: 3m 45s
   ```
3. **Upload test report as artifact** for reference

### Partial Success (Some Failures)

If SOME servers fail:

1. **Create authentication issues** for each server needing auth
2. **Create summary issue** for other failures (if any)
3. **Log summary:**
   ```
   ⚠️ MCP Stress Test completed with failures
   Success: 15/20 servers
   Auth Required: 3 servers (created 3 issues)
   Other Failures: 2 servers (created 1 summary issue)
   ```

### Complete Failure (Gateway Crash)

If the gateway crashes or fails to start:

1. **Create critical issue** about gateway failure
2. **Include full gateway logs**
3. **Mark as high priority**

## Artifact Upload

Always upload the following as workflow artifacts:

1. **Test Report** - `/tmp/mcp-stress-results/report.md`
2. **Gateway Logs** - `/tmp/mcp-stress-test/logs/`
3. **Error Summary** - `/tmp/mcp-stress-results/errors.txt`
4. **Test Configuration** - `/tmp/mcp-stress-test-config.json`

## Notes

- This is a **nightly** workflow - results accumulate over time to detect regressions
- Focus on **breadth** over depth - test many servers quickly
- Prioritize **stability** - don't destabilize the gateway
- Keep **runtime under 15 minutes** - stress test should be fast
- **Document findings** - every failure should be actionable

Begin the stress test! Configure 20 MCP servers, start the gateway, test each server's tools, collect results, and create appropriate issues for any failures found.
