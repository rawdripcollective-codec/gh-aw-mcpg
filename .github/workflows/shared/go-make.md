---
safe-inputs:
  go:
    description: "Execute any Go command. This tool is accessible as 'safeinputs-go'. Provide the full command after 'go' (e.g., args: 'test ./...'). The tool will run: go <args>. Use single quotes ' for complex args to avoid shell interpretation issues."
    inputs:
      args:
        type: string
        description: "Arguments to pass to go CLI (without the 'go' prefix). Examples: 'test ./...', 'build ./cmd/gh-aw', 'mod tidy', 'fmt ./...', 'vet ./...'"
        required: true
    run: |
      echo "go $INPUT_ARGS"
      go $INPUT_ARGS

  make:
    description: "Execute any Make target. This tool is accessible as 'safeinputs-make'. Provide the target name(s) (e.g., args: 'build'). The tool will run: make <args>. Use single quotes ' for complex args to avoid shell interpretation issues."
    inputs:
      args:
        type: string
        description: "Arguments to pass to make (target names and options). Examples: 'build', 'test-unit', 'lint', 'recompile', 'agent-finish', 'fmt build test-unit'"
        required: true
    run: |
      echo "make $INPUT_ARGS"
      make $INPUT_ARGS
---

**IMPORTANT**: Always use the `safeinputs-go` and `safeinputs-make` tools for Go and Make commands instead of running them directly via bash. These safe-input tools provide consistent execution and proper logging.

**Correct**:
```
Use the safeinputs-go tool with args: "test ./..."
Use the safeinputs-make tool with args: "build"
Use the safeinputs-make tool with args: "lint"
Use the safeinputs-make tool with args: "test-unit"
```

**Incorrect**:
```
Use the go safe-input tool with args: "test ./..."  ❌ (Wrong tool name - use safeinputs-go)
Run: go test ./...  ❌ (Use safeinputs-go instead)
Execute bash: make build  ❌ (Use safeinputs-make instead)
```
