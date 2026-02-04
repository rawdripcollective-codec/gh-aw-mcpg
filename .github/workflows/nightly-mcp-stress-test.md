---
name: Nightly MCP Server Stress Test
description: Comprehensive stress test that loads 20 well-known MCP servers, tests their tools, and reports results with automated issue creation for failures
on:
  schedule: daily
  workflow_dispatch:

permissions:
  contents: read
  actions: read
  issues: read
  pull-requests: read

roles: [admin, maintainer, write]

steps:
  - name: Set up Go
    uses: actions/setup-go@v6
    with:
      go-version-file: go.mod
      cache: true

tools:
  github:
    toolsets: [default]
  bash: ["*"]

safe-outputs:
  create-issue:
    title-prefix: "[mcp-stress-test] "
    labels: [mcp-gateway, stress-test, automation]
    assignees: []
    max: 25
  missing-tool:
    create-issue: true

post-steps:
  - name: Upload Test Results
    if: always()
    uses: actions/upload-artifact@v4
    with:
      name: mcp-stress-test-results
      path: |
        /tmp/mcp-stress-results/
        /tmp/mcp-stress-test/logs/
      retention-days: 30

timeout-minutes: 30
strict: true
---

<!-- Edit the file linked below to modify the agent without recompilation. Feel free to move the entire markdown body to that file. -->
@./agentics/nightly-mcp-stress-test.md
