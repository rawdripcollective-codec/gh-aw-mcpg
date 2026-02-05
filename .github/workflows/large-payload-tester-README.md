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
│  (via Copilot)  │◄────────►│  Container        │◄────────►│  MCP Server    │
└─────────────────┘          └──────────────────┘          └────────────────┘
        │                            │                             │
        │                            │                             │
   Reads via                   Stores payload              Reads test file
   filesystem MCP              to /tmp/jq-payloads         from /tmp/mcp-test-fs
        │                            │                             │
        ▼                            ▼                             ▼
   /workspace/mcp-payloads/   /tmp/jq-payloads/           /workspace/test-data/
   (mounted from runner)      (gateway writes)            (mounted from runner)
                              {sessionID}/{queryID}/       large-test-file.json
                              payload.json                 (contains secret)

Runner Filesystem:
/tmp/mcp-test-fs/          →  Only accessible to filesystem MCP server
/tmp/jq-payloads/          →  Shared between gateway (writes) and filesystem server (reads)
```

**Flow**:
1. Agent requests file via gateway → filesystem MCP server
2. Filesystem server reads from its `/workspace/test-data/` (mounted from `/tmp/mcp-test-fs`)
3. Gateway intercepts large response
4. Gateway stores to `/tmp/jq-payloads/{sessionID}/{queryID}/payload.json`
5. Agent reads payload via filesystem server's `/workspace/mcp-payloads/` mount

### Test Protocol

The workflow uses a **secret-based verification** approach:

1. **Setup Phase** (bash step):
   - Generate a unique UUID secret
   - Create `/tmp/mcp-test-fs/test-secret.txt` containing just the secret
   - Create `/tmp/mcp-test-fs/large-test-file.json` (~500KB) with:
     - The secret embedded in JSON data
     - Array of 2000 items each referencing the secret
     - 400KB of padding to ensure size ~500KB

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

### Volume Mounts

The workflow uses a carefully structured mount configuration to ensure proper isolation:

1. **Test Data Mount** (filesystem MCP server ONLY):
   ```yaml
   /tmp/mcp-test-fs:/workspace/test-data:ro
   ```
   - Contains the control secret file and large test file on the actions runner
   - Mounted ONLY to the filesystem MCP server container (NOT to the gateway)
   - Read-only access for safety
   - Accessible to agent via filesystem MCP server at `/workspace/test-data/`
   - Gateway does NOT have direct access to test files

2. **Payload Mount** (filesystem MCP server):
   ```yaml
   /tmp/jq-payloads:/workspace/mcp-payloads:ro
   ```
   - Allows agent to read stored payloads through filesystem MCP server
   - Read-only to prevent accidental corruption
   - Accessible to agent via `/workspace/mcp-payloads/`
   - Initially empty/non-existent until gateway stores first payload

3. **Gateway Payload Mount** (MCP gateway container):
   ```yaml
   /tmp/jq-payloads:/tmp/jq-payloads:rw
   ```
   - Allows gateway to write payload files
   - Read-write for payload storage
   - Gateway creates directory structure on-demand
   - This is the ONLY directory the gateway container has mounted

**Key Design Principle**: The test data directory (`/tmp/mcp-test-fs`) is isolated from the gateway. The gateway only has access to the payload directory (`/tmp/jq-payloads`). This ensures that the gateway cannot directly access test files and must retrieve them through the filesystem MCP server, properly testing the MCP protocol flow.

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

This workflow tests the jqschema middleware feature. The related implementation files are:
- `internal/middleware/jqschema.go` - Middleware that intercepts large responses
- `internal/middleware/README.md` - Documentation of the jqschema middleware
- `internal/config/config_payload.go` - Payload directory configuration
- `test/integration/large_payload_test.go` - Unit/integration tests for payload handling

These files already exist in the repository and implement the feature being tested by this workflow.

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
