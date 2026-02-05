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
      ALLOWED_PATHS: "/workspace"
    mounts:
      - "/tmp/mcp-test-fs:/workspace/test-data:ro"
      - "/tmp/jq-payloads:/workspace/mcp-payloads:ro"

sandbox:
  mcp:
    container: "ghcr.io/github/gh-aw-mcpg"
    mounts:
      - "/tmp/mcp-test-fs:/tmp/mcp-test-fs:ro"
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
      # Create test data directory (payload directory will be created by gateway on-demand)
      mkdir -p /tmp/mcp-test-fs
      
      # Generate a unique secret for this test run
      # Use uuidgen if available, otherwise use timestamp with nanoseconds for better entropy
      if command -v uuidgen >/dev/null 2>&1; then
        TEST_SECRET="test-secret-$(uuidgen)"
      else
        TEST_SECRET="test-secret-$(date +%s%N)-$$"
      fi
      echo "$TEST_SECRET" > /tmp/mcp-test-fs/test-secret.txt
      
      # Create a large test file (~500KB) with the secret embedded in JSON
      # This file will be read by the filesystem MCP server, causing a large payload
      cat > /tmp/mcp-test-fs/large-test-file.json <<'EOF'
      {
        "test_run_id": "PLACEHOLDER_RUN_ID",
        "test_secret": "PLACEHOLDER_SECRET",
        "test_timestamp": "PLACEHOLDER_TIMESTAMP",
        "purpose": "Testing large MCP payload storage and retrieval",
        "data": {
          "large_array": [],
          "metadata": {
            "generated_by": "large-payload-tester workflow",
            "repository": "PLACEHOLDER_REPO",
            "workflow_run_url": "PLACEHOLDER_URL"
          }
        },
        "padding": ""
      }
      EOF
      
      # Use jq to properly populate the JSON with dynamic values and generate large array
      # Generating 2000 items + 400KB padding to create ~500KB file
      jq --arg secret "$TEST_SECRET" \
         --arg run_id "${{ github.run_id }}" \
         --arg timestamp "$(date -Iseconds)" \
         --arg repo "${{ github.repository }}" \
         --arg url "${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}" \
         '.test_secret = $secret | 
          .test_run_id = $run_id | 
          .test_timestamp = $timestamp | 
          .data.metadata.repository = $repo | 
          .data.metadata.workflow_run_url = $url | 
          .data.large_array = [range(2000) | {id: ., value: ("item-" + tostring), secret_reference: $secret, extra_data: ("data-" + tostring + "-" * 50)}] |
          .padding = ("X" * 400000)' \
         /tmp/mcp-test-fs/large-test-file.json > /tmp/mcp-test-fs/large-test-file.json.tmp
      
      mv /tmp/mcp-test-fs/large-test-file.json.tmp /tmp/mcp-test-fs/large-test-file.json
      
      # Verify file was created and is large enough
      FILE_SIZE=$(wc -c < /tmp/mcp-test-fs/large-test-file.json)
      echo "Created large-test-file.json with size: $FILE_SIZE bytes (~$(($FILE_SIZE / 1024))KB)"
      if [ "$FILE_SIZE" -lt 512000 ]; then
        echo "WARNING: Test file is smaller than expected ($FILE_SIZE bytes < 500KB)"
        echo "Continuing with test anyway..."
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
