---
name: Large Payload Tester
description: Test the MCP Gateway's ability to handle large payloads and provide agent access to stored payload files
on:
  workflow_dispatch:
  schedule: daily

permissions:
  contents: read
  issues: read

roles: [admin, maintainer, write]

network:
  allowed:
    - defaults

tools:
  bash: ["*"]

mcp-servers:
  filesystem:
    type: stdio
    container: "mcp/filesystem"
    env:
      ALLOWED_PATHS: "/workspace,/tmp"
    mounts:
      - "/tmp/mcp-test-fs:/workspace/test-data:ro"
      - "/tmp/jq-payloads:/workspace/mcp-payloads:ro"

sandbox:
  mcp:
    container: "ghcr.io/github/gh-aw-mcpg"
    mounts:
      - "/tmp/jq-payloads:/tmp/jq-payloads:rw"

safe-outputs:
  create-issue:
    title-prefix: "[large-payload-test] "
    labels: [mcp-gateway, test, automation]
    max: 10
    close-older-issues: true

steps:
  - name: Setup Test Environment
    run: |
      # Create test directories
      mkdir -p /tmp/mcp-test-fs
      mkdir -p /tmp/jq-payloads
      
      # Generate a unique secret for this test run
      TEST_SECRET="test-secret-$(uuidgen || echo $RANDOM-$RANDOM-$RANDOM)"
      echo "$TEST_SECRET" > /tmp/mcp-test-fs/test-secret.txt
      
      # Create a large test file (>1KB) with the secret embedded in JSON
      # This file will be read by the filesystem MCP server, causing a large payload
      cat > /tmp/mcp-test-fs/large-test-file.json <<EOF
      {
        "test_run_id": "${{ github.run_id }}",
        "test_secret": "$TEST_SECRET",
        "test_timestamp": "$(date -Iseconds)",
        "purpose": "Testing large MCP payload storage and retrieval",
        "data": {
          "large_array": [
            $(for i in {1..100}; do echo "{\"id\": $i, \"value\": \"item-$i\", \"secret_reference\": \"$TEST_SECRET\"}"; done | paste -sd,)
          ],
          "metadata": {
            "generated_by": "large-payload-tester workflow",
            "repository": "${{ github.repository }}",
            "workflow_run_url": "${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}"
          }
        },
        "padding": "$(head -c 2000 /dev/zero | tr '\0' 'X')"
      }
      EOF
      
      # Verify file was created and is large enough
      FILE_SIZE=$(wc -c < /tmp/mcp-test-fs/large-test-file.json)
      echo "Created large-test-file.json with size: $FILE_SIZE bytes"
      if [ "$FILE_SIZE" -lt 1024 ]; then
        echo "ERROR: Test file is too small ($FILE_SIZE bytes < 1KB)"
        exit 1
      fi
      
      # Verify secret file was created
      if [ ! -f /tmp/mcp-test-fs/test-secret.txt ]; then
        echo "ERROR: Secret file was not created"
        exit 1
      fi
      
      echo "Test environment setup complete"
      echo "Secret stored in: /tmp/mcp-test-fs/test-secret.txt"
      echo "Large file stored in: /tmp/mcp-test-fs/large-test-file.json"

timeout-minutes: 10
strict: true
---

<!-- Edit the file linked below to modify the agent without recompilation. Feel free to move the entire markdown body to that file. -->
@./agentics/large-payload-tester.md
