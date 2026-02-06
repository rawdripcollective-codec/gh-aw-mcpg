<!-- This prompt will be imported in the agentic workflow .github/workflows/large-payload-tester.md at runtime. -->
<!-- You can edit this file to modify the agent behavior without recompiling the workflow. -->

# Large MCP Payload Access Test

You are an AI agent trying verify a secret in a file hosted by the filesystem MCP server.

## Your Task

Use the filesystem MCP server to access a file called `large-test-file.json`, which contains a field `secret_reference`. Verify that the secret value matches the content of file `secret.txt` which should also be accessed from the filesystem MCP server.

## Important Notes

- **Keep all outputs concise** - Use brief, factual statements
- **Log all key values** - Secret, paths, sizes
- **Be explicit about failures** - State exactly what went wrong if any step fails

## Expected Behavior

**Success scenario:**
1. Agent requests file `large-test-file.json` from filesystem MCP server.
2. MCP server response includes path to full payload.
3. Agent reads full payload from subdirectory of: `/tmp/gh-aw/mcp-payloads`
5. Agent extracts secret from full payload.
6. Agent requests file `secret.txt` from filesystem MCP server.
7. Secrets in `large-test-file.json` and `secret.txt` match.

**Failure scenarios to detect:**
- Agent can't read payload file (permission/mount issues)
- MCP server does not return a payload path
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

- **Expected Secret:** [UUID from test-secret.txt]
- **Found Secret:** [UUID from payload] or "NOT FOUND"
- **Secret Match:** [YES/NO]
- **Payload Path:** [path from response]
- **Payload Size:** [originalSize from metadata]

## Conclusion

[Brief summary of what worked and what failed, if anything]

---
Run URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
```
