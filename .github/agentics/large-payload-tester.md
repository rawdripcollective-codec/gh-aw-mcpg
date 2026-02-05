<!-- This prompt will be imported in the agentic workflow .github/workflows/large-payload-tester.md at runtime. -->
<!-- You can edit this file to modify the agent behavior without recompiling the workflow. -->

# Large MCP Payload Access Test

You are an AI agent testing the MCP Gateway's ability to handle large payloads and make them accessible to agents.

## Your Task

Test that when the MCP Gateway receives large responses from backend MCP servers:
1. It correctly stores payloads to disk with proper session isolation
2. It returns metadata including the payload file path
3. Agents can successfully read the payload files from their mounted session directory

## Test Protocol

This test uses a **secret-based verification approach**:
1. A secret UUID is embedded in a large test file (>1KB) before the test runs
2. You will use the filesystem MCP server to read a large file containing this secret
3. The gateway will intercept the large response, store it to disk, and return metadata with a `payloadPath`
4. You must then read the payload file from the path provided and extract the secret
5. Finally, report whether you successfully retrieved the secret from the payload

## Test Steps

### Step 1: Read the Test Secret
- Read `/workspace/test-data/test-secret.txt` to get the secret UUID that was generated for this test run
- This file contains ONLY the secret UUID (e.g., `abc123-def456-ghi789`)
- Store this secret - you'll need it to verify payload retrieval later

### Step 2: Trigger a Large Payload Response
- Use the filesystem MCP server's `read_file` tool to read `/workspace/test-data/large-test-file.json`
- This file is >1KB and contains the secret embedded in JSON data
- The gateway should intercept this response and store it to disk

### Step 3: Extract Metadata from Gateway Response
The gateway's jqschema middleware should transform the response to include:
- `payloadPath`: Full path to the stored payload file
- `preview`: First 500 characters of the response
- `schema`: JSON schema showing structure
- `originalSize`: Size of the full payload
- `queryID`: Unique identifier for this tool call
- `truncated`: Boolean indicating if preview was truncated

Extract and log:
- The `payloadPath` value
- The `queryID` value
- Whether `truncated` is `true`
- The `originalSize` value

### Step 4: Read the Payload File
The payload path will be in the format: `/tmp/jq-payloads/{sessionID}/{queryID}/payload.json`

**IMPORTANT**: The agent's payload directory is mounted to the agent's container. The path you receive from the gateway uses the gateway's filesystem perspective. To read the file:
- The gateway reports path as: `/tmp/jq-payloads/{sessionID}/{queryID}/payload.json`
- In the agent container, the entire `/tmp/jq-payloads` directory is mounted at: `/workspace/mcp-payloads`
- So translate the path by replacing `/tmp/jq-payloads` with `/workspace/mcp-payloads`
- Example: If gateway returns `/tmp/jq-payloads/session-abc123/query-def456/payload.json`, use `/workspace/mcp-payloads/session-abc123/query-def456/payload.json`
- The `{sessionID}` is the actual session identifier, not the literal word "session"
- Use the filesystem MCP server to read the translated path

Use the filesystem MCP server's `read_file` tool to read the payload file at the translated path.

### Step 5: Verify the Secret
- Parse the payload JSON you retrieved
- Search for the secret UUID in the payload
- Compare it with the secret you read in Step 1
- **Verification passes if**: The secret from the payload matches the secret from test-secret.txt
- **Verification fails if**: The secret is missing, doesn't match, or you couldn't read the payload file

### Step 6: Report Results
Create a summary of the test results including:
1. ✅ or ❌ for each test step
2. The secret value you expected (from test-secret.txt)
3. The secret value you found (from the payload file)
4. Whether secrets matched (PASS/FAIL)
5. Path information (gateway path and agent path used)
6. Any errors encountered

## Important Notes

- **Keep all outputs concise** - Use brief, factual statements
- **Log all key values** - Secret, paths, sizes, queryID
- **Be explicit about failures** - State exactly what went wrong if any step fails
- **Path translation is critical** - The gateway and agent see different filesystem paths due to volume mounts

## Expected Behavior

**Success scenario:**
1. Gateway receives large response from filesystem server
2. Gateway stores payload to: `/tmp/jq-payloads/{sessionID}/{queryID}/payload.json`
3. Gateway returns metadata with `payloadPath` and `truncated: true`
4. Agent reads payload from mounted path: `/workspace/mcp-payloads/session/{queryID}/payload.json`
5. Agent extracts secret from payload
6. Secret matches the expected value from test-secret.txt

**Failure scenarios to detect:**
- Gateway doesn't intercept/store large payloads (no payloadPath in response)
- Gateway path is incorrect or inaccessible
- Agent can't read payload file (permission/mount issues)
- Payload is corrupted or incomplete
- Secret doesn't match (data integrity issue)

## Output Format

After running all tests, create an issue with:
- Title: "Large Payload Test - ${{ github.run_id }}"
- Body with test results in this format:

```markdown
# Large MCP Payload Access Test Results

**Run ID:** ${{ github.run_id }}
**Status:** [PASS/FAIL]
**Timestamp:** [current time]

## Test Results

1. ✅/❌ Read test secret from control file
2. ✅/❌ Trigger large payload response (>1KB)
3. ✅/❌ Receive gateway metadata with payloadPath
4. ✅/❌ Translate and access payload file path
5. ✅/❌ Read payload file contents
6. ✅/❌ Extract and verify secret

## Details

- **Expected Secret:** [UUID from test-secret.txt]
- **Found Secret:** [UUID from payload] or "NOT FOUND"
- **Secret Match:** [YES/NO]
- **Gateway Path:** [path from response]
- **Agent Path:** [translated path used]
- **Payload Size:** [originalSize from metadata]
- **Query ID:** [queryID from metadata]

## Conclusion

[Brief summary of what worked and what failed, if anything]

---
Run URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
```
