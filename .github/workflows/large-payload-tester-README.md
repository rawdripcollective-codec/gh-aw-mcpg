# Large Payload Tester Workflow

## Purpose

This agentic workflow tests the MCP Gateway's large payload handling feature, specifically:
1. **Payload Storage**: Verifies that large responses (>500 chars) are automatically stored to disk
2. **Metadata Response**: Confirms the gateway returns correct metadata including `payloadPath`, `schema`, `preview`, etc.
3. **Agent Access**: Tests that agents can successfully read the payload files from their mounted directories
4. **Session Isolation**: Validates that payload files are organized by session ID for multi-agent isolation

## How It Works

### Test Architecture

```
┌─────────────────┐          ┌──────────────────┐          ┌────────────────┐
│  Agent          │          │  MCP Gateway      │          │  Filesystem    │
│  Container      │◄────────►│  Container        │◄────────►│  MCP Server    │
└─────────────────┘          └──────────────────┘          └────────────────┘
        │                            │                             │
        │                            │                             │
   Reads payload              Stores payload              Reads large file
   from mounted dir           to /tmp/jq-payloads         from /tmp/mcp-test-fs
        │                            │                             │
        ▼                            ▼                             ▼
   /workspace/                /tmp/jq-payloads/           /tmp/mcp-test-fs/
   mcp-payloads/              {sessionID}/                large-test-file.json
                              {queryID}/                  (contains secret)
                              payload.json
```

### Test Protocol

The workflow uses a **secret-based verification** approach:

1. **Setup Phase** (bash step):
   - Generate a unique UUID secret
   - Create `/tmp/mcp-test-fs/test-secret.txt` containing just the secret
   - Create `/tmp/mcp-test-fs/large-test-file.json` (>1KB) with:
     - The secret embedded in JSON data
     - Array of 100 items each referencing the secret
     - 2KB of padding to ensure size >1KB

2. **Test Phase** (agent):
   - **Step 1**: Read `/workspace/test-data/test-secret.txt` to get the expected secret
   - **Step 2**: Call filesystem MCP server to read `/workspace/test-data/large-test-file.json`
   - **Step 3**: Gateway intercepts large response, stores to disk, returns metadata
   - **Step 4**: Extract `payloadPath` and `queryID` from metadata
   - **Step 5**: Translate path and read from `/workspace/mcp-payloads/{sessionID}/{queryID}/payload.json`
   - **Step 6**: Extract secret from payload and verify it matches expected secret

3. **Report Phase** (safe-output):
   - Create GitHub issue with test results
   - Report pass/fail for each step
   - Include secret comparison results

### Volume Mounts

The workflow uses three volume mounts to enable the test:

1. **Test Data Mount** (filesystem MCP server):
   ```yaml
   /tmp/mcp-test-fs:/workspace/test-data:ro
   ```
   - Contains the control secret file and large test file
   - Read-only access for safety
   - Accessible to agent via `/workspace/test-data/`

2. **Payload Mount** (filesystem MCP server):
   ```yaml
   /tmp/jq-payloads:/workspace/mcp-payloads:ro
   ```
   - Allows agent to read stored payloads
   - Read-only to prevent accidental corruption
   - Accessible to agent via `/workspace/mcp-payloads/`

3. **Gateway Payload Mount** (MCP gateway container):
   ```yaml
   /tmp/jq-payloads:/tmp/jq-payloads:rw
   ```
   - Allows gateway to write payload files
   - Read-write for payload storage

### Path Translation

The agent must translate paths between gateway and agent perspectives:

- **Gateway reports**: `/tmp/jq-payloads/{sessionID}/{queryID}/payload.json`
- **Agent uses**: `/workspace/mcp-payloads/{sessionID}/{queryID}/payload.json`
- **Translation rule**: Replace `/tmp/jq-payloads` → `/workspace/mcp-payloads`

## Expected Behavior

### Success Scenario

When working correctly:
1. Gateway intercepts the large file read response
2. Gateway stores payload to disk with structure: `{payloadDir}/{sessionID}/{queryID}/payload.json`
3. Gateway returns metadata with `truncated: true` and `payloadPath`
4. Agent translates path and successfully reads payload file
5. Agent extracts secret from payload
6. Secret matches the expected value
7. Test reports **PASS**

### Failure Scenarios

The test is designed to detect:
- **Gateway not intercepting**: No `payloadPath` in response
- **Wrong path structure**: Agent can't find the file
- **Permission issues**: Agent can't read the payload file
- **Mount problems**: Volume mounts not configured correctly
- **Data corruption**: Secret in payload doesn't match expected
- **Session isolation broken**: Wrong session directory used

## Files

- `.github/workflows/large-payload-tester.md` - Workflow definition with frontmatter and setup steps
- `.github/agentics/large-payload-tester.md` - Agent prompt with detailed test instructions
- `.github/workflows/large-payload-tester.lock.yml` - Compiled GitHub Actions workflow

## Triggering

- **Manual**: `workflow_dispatch` - Can be triggered manually from GitHub UI
- **Scheduled**: Runs daily at a scattered time (around 1:12 AM UTC)

## Configuration

Key configuration in frontmatter:
- `strict: true` - Enforces security best practices
- `timeout-minutes: 10` - Reasonable timeout for the test
- `network.allowed: [defaults]` - Minimal network access
- `tools.bash: ["*"]` - Full bash access for setup steps
- `mcp-servers.filesystem` - Configured with two volume mounts

## Related Features

This workflow tests the jqschema middleware feature implemented in:
- `internal/middleware/jqschema.go` - Middleware that intercepts large responses
- `internal/config/config_payload.go` - Payload directory configuration
- `test/integration/large_payload_test.go` - Unit/integration tests for payload handling

## Security Considerations

- **Read-only mounts**: Agent has read-only access to payload directory
- **Session isolation**: Each session gets its own subdirectory
- **Payload cleanup**: Old payloads are not automatically cleaned up (manual cleanup needed)
- **File permissions**: Payload files created with `0600` (owner read/write only)
- **Secret handling**: Test secret is only used for this test and is not sensitive

## Future Enhancements

Potential improvements:
1. Test with multiple concurrent sessions
2. Test with very large payloads (>10MB)
3. Test payload cleanup mechanisms
4. Add performance metrics (storage time, read time)
5. Test error handling (disk full, permission denied)
6. Verify jq schema accuracy against complex data structures
