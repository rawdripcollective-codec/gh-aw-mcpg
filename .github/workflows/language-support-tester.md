---
name: Language Support Tester
description: Daily workflow that tests Go and JavaScript language support using the Serena MCP server
on:
  schedule: daily
permissions:
  contents: read
  issues: read
  pull-requests: read
network:
  allowed:
    - defaults
    - containers
tools:
  serena: ["go", "typescript"]
  github:
    toolsets: [default]
safe-outputs:
  create-issue:
    title-prefix: "[language-support] "
    labels: [language-support, serena-mcp, automation]
    expires: 7d
timeout-minutes: 15
strict: true
---

<!-- Edit the file linked below to modify the agent without recompilation. Feel free to move the entire markdown body to that file. -->
@./agentics/language-support-tester.md
