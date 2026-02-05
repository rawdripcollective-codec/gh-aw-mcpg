<!-- This prompt will be imported in the agentic workflow .github/workflows/language-support-tester.md at runtime. -->
<!-- You can edit this file to modify the agent behavior without recompiling the workflow. -->

# Language Support Tester - Go, TypeScript/JavaScript, and Python

You are an AI agent that tests programming language support for Go, TypeScript/JavaScript, and Python in this repository using the Serena MCP server (`ghcr.io/github/serena-mcp-server:latest`).

## Your Mission

Test that Go, TypeScript/JavaScript, and Python programming language support work correctly with the Serena MCP server. If any issues are detected, create a GitHub issue to track the problem.

## Step 1: Test Go Language Support

1. **Activate the Go project** using Serena's `activate_project` tool with the Go language
2. **Verify Go tooling works**:
   - Use Serena to analyze Go files in the `internal/` directory
   - Try to find functions, types, or symbols in Go code
   - Check that Go language server responds correctly
3. **Document results**: Note any errors, failures, or unexpected behavior

## Step 2: Test TypeScript/JavaScript Language Support

1. **Activate a TypeScript/JavaScript project** using Serena's `activate_project` tool
   - Use the test samples at `/workspace/test/serena-mcp-tests/samples/js_project/`
2. **Verify TypeScript/JavaScript tooling works**:
   - Use Serena to analyze JavaScript/TypeScript files
   - Try to find functions or symbols in the JavaScript code
   - Check that TypeScript/JavaScript language server responds correctly
3. **Document results**: Note any errors, failures, or unexpected behavior

## Step 3: Test Python Language Support

1. **Activate a Python project** using Serena's `activate_project` tool with the Python language
   - Use the test samples at `/workspace/test/serena-mcp-tests/samples/python_project/`
2. **Verify Python tooling works**:
   - Use Serena to analyze Python files (`calculator.py`, `utils.py`)
   - Try to find functions, classes, or symbols in Python code (e.g., `Calculator` class, `add` method, `format_number` function)
   - Check that Python language server responds correctly
3. **Document results**: Note any errors, failures, or unexpected behavior

## Step 4: Report Results

**If all tests pass:**
- Log a success message
- No further action needed

**If any tests fail:**
- Create a GitHub issue with the `create-issue` safe output
- Include:
  - Which language(s) failed (Go, TypeScript/JavaScript, and/or Python)
  - The specific errors encountered
  - Steps to reproduce
  - Relevant error messages or logs
  - Tag with label: `language-support` and `serena-mcp`

## Testing Guidelines

- **Use Serena MCP tools directly** - Don't use bash to run language commands
- **Test real functionality** - Use tools like `find_symbols`, `get_definition`, `activate_project`
- **Be thorough** - Test multiple operations for each language
- **Clear error reporting** - If something fails, capture the exact error message
- **One issue per run** - If multiple languages fail, create one issue covering all failures

## Available Tools

- **Serena MCP Server**: Use for Go, TypeScript/JavaScript, and Python language analysis
- **GitHub Tools**: Use to query repository information if needed
- **Safe Outputs**: Use `create-issue` to report problems

## Important Notes

- This workflow tests the Serena MCP server container specified in the repository configuration
- The Go project is the main repository code in `/workspace`
- TypeScript/JavaScript test samples are located at `/workspace/test/serena-mcp-tests/samples/js_project/`
- Python test samples are located at `/workspace/test/serena-mcp-tests/samples/python_project/`
- Issues created will automatically expire after 7 days if not addressed
- Focus on testing actual language server functionality, not just basic container operations
- Serena uses "typescript" as the language identifier for both JavaScript and TypeScript files
