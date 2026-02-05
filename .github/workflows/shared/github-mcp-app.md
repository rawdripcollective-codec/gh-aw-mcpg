---
#tools:
#  github:
#    app:
#      app-id: ${{ vars.APP_ID }}
#      private-key: ${{ secrets.APP_PRIVATE_KEY }}
---

<!--
# Shared GitHub MCP App Configuration

This shared workflow provides repository-level GitHub App configuration for the GitHub MCP server.

## Configuration Variables

This shared workflow expects:
- **Repository Variable**: `APP_ID` - The GitHub App ID for MCP server authentication
- **Repository Secret**: `APP_PRIVATE_KEY` - The GitHub App private key for MCP server authentication

## Usage

Import this configuration in your workflows to enable GitHub App authentication for the GitHub MCP server:

```yaml
imports:
  - shared/github-mcp-app.md
```

The configuration will be automatically merged into your workflow's tools.github section.
-->
