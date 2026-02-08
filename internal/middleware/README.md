# jqschema Middleware

This middleware package implements the jqschema functionality from the gh-aw shared agentic workflow as a tool call middleware for the MCP Gateway.

## Features

- **Automatic JSON Schema Inference**: Uses the jq schema transformation logic to automatically infer the structure and types of JSON responses
- **Payload Storage**: Stores complete response payloads in `/tmp/gh-awmg/tools-calls/{randomID}/payload.json`
- **Response Rewriting**: Returns a transformed response containing:
  - First 500 characters of the payload (for quick preview)
  - Inferred JSON schema showing structure and types
  - Query ID for tracking
  - File path to complete payload
  - Metadata (original size, truncation status)

## How It Works

The middleware wraps tool handlers and intercepts their responses:

1. **Random ID Generation**: Each tool call gets a unique random ID (32 hex characters)
2. **Original Handler Execution**: The original tool handler is called normally
3. **Payload Storage**: The complete response is saved to disk
4. **Schema Inference**: The jq schema transformation is applied to extract types and structure
5. **Response Rewriting**: A new response is returned with preview + schema

## Usage

The middleware is automatically applied to all backend MCP server tools (except `sys___*` tools).

### Example Response

**Original response:**
```json
{
  "total_count": 1000,
  "items": [
    {
      "login": "user1",
      "id": 123,
      "verified": true
    },
    {
      "login": "user2",
      "id": 456,
      "verified": false
    }
  ]
}
```

**Transformed response:**
```json
{
  "agentInstructions": "The payload was too large for an MCP response. The payloadSchema approximates the structure of the full payload. The full response can be accessed through the local file system at the payloadPath.",
  "payloadPath": "/tmp/gh-awmg/tools-calls/a1b2c3d4e5f6.../payload.json",
  "payloadPreview": "{\"total_count\":1000,\"items\":[{\"login\":\"user1\",\"id\":123,\"verified\":true}...",
  "payloadSchema": {
    "total_count": "number",
    "items": [
      {
        "login": "string",
        "id": "number",
        "verified": "boolean"
      }
    ]
  },
  "originalSize": 234
}
```

## Implementation Details

### jq Schema Filter

The middleware uses the same jq filter logic as the gh-aw jqschema utility:

```jq
def walk(f):
  . as $in |
  if type == "object" then
    reduce keys[] as $k ({}; . + {($k): ($in[$k] | walk(f))})
  elif type == "array" then
    if length == 0 then [] else [.[0] | walk(f)] end
  else
    type
  end;
walk(.)
```

This recursively walks the JSON structure and replaces values with their type names.

### Go Implementation

The middleware is implemented using [gojq](https://github.com/itchyny/gojq), a pure Go implementation of jq, eliminating the need to spawn external processes.

## Configuration

The middleware can be controlled via the `ShouldApplyMiddleware` function:

```go
func ShouldApplyMiddleware(toolName string) bool {
    // Currently excludes sys tools
    return !strings.HasPrefix(toolName, "sys___")
}
```

### Future Enhancements

**Selective Middleware Mounting**: A configuration system could be added to:
- Enable/disable middleware per backend server
- Configure which tools get middleware applied
- Set custom truncation limits
- Configure storage locations
- Add multiple middleware types with ordering

Example future config structure:
```toml
[middleware.jqschema]
enabled = true
truncate_at = 500
storage_path = "/tmp/gh-awmg/tools-calls"
exclude_tools = ["sys___*"]
include_backends = ["github", "tavily"]
```

## Testing

The middleware includes comprehensive tests:

- **Unit tests**: Test individual functions (random ID generation, schema transformation, payload storage)
- **Integration tests**: Test complete middleware flow with mock handlers
- **Edge cases**: Test error handling, large payloads, truncation behavior

Run tests:
```bash
make test-unit
# or
go test ./internal/middleware/...
```

## Directory Structure

Payloads are stored in:
```
/tmp/gh-awmg/tools-calls/
  ├── {randomID1}/
  │   └── payload.json
  ├── {randomID2}/
  │   └── payload.json
  └── ...
```

## Benefits

1. **Reduced Token Usage**: Preview + schema is much smaller than full responses
2. **Better Understanding**: Schema shows structure without verbose data
3. **Audit Trail**: Complete payloads are saved for later inspection
4. **Debugging**: Query IDs enable tracking and correlation
5. **Performance**: Pure Go implementation (no external process spawning)

## References

- Original jqschema utility: [gh-aw/.github/workflows/shared/jqschema.md](https://github.com/github/gh-aw/blob/main/.github/workflows/shared/jqschema.md)
- gojq library: [github.com/itchyny/gojq](https://github.com/itchyny/gojq)
