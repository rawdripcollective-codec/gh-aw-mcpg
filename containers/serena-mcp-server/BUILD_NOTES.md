# Serena MCP Server Container - Build Notes

## Current Status

The Serena MCP server container Dockerfile has been created with support for:
- Python 3.11
- Java (OpenJDK 21 via default-jdk)
- JavaScript/TypeScript (Node.js + npm)
- Go (golang-go package)

## Recent Fixes

### PATH Configuration Fix (2026-01-18)
Fixed an issue where the `go` command was not found in the container's PATH during runtime:
- **Problem**: Line 40 of the Dockerfile explicitly set `/usr/bin` in the PATH, which was redundant and potentially caused PATH resolution issues
- **Solution**: Changed `ENV PATH="${GOPATH}/bin:/usr/bin:${PATH}"` to `ENV PATH="${GOPATH}/bin:${PATH}"`
- **Impact**: Simplifies PATH configuration by relying on the base image's default PATH, which already includes `/usr/bin`
- **Testing**: Smoke tests should now successfully execute `go version` within the container

## Build Issues Encountered

During local testing, the container build encountered SSL/TLS certificate verification issues:
- `SSL: CERTIFICATE_VERIFY_FAILED certificate verify failed: self-signed certificate in certificate chain`
- This affects:
  - pip installations from PyPI and GitHub
  - npm package installations
  - Go module downloads

This appears to be an environment-specific issue related to network proxy/firewall configuration in the GitHub Actions runner environment.

## Solutions

### Option 1: Build in GitHub Actions (Recommended)
The GitHub Actions workflow (`..github/workflows/serena-container.yml`) should work correctly as it:
- Runs in GitHub's standard build environment
- Has proper network access without SSL interception
- Uses multi-arch buildx for amd64/arm64 support

### Option 2: Local Build with SSL Verification Disabled
For local testing only (NOT recommended for production):

```dockerfile
# Add before pip/npm commands:
ENV PIP_TRUSTED_HOST="pypi.org files.pythonhosted.org pypi.python.org"
ENV NODE_TLS_REJECT_UNAUTHORIZED="0"
```

### Option 3: Simplified Dockerfile
Create a minimal version that uses only packages available in Debian repos, then install Serena at runtime.

## Next Steps

1. The Dockerfile and workflow are ready for GitHub Actions to build
2. Once merged to main, the workflow will automatically build and push to GHCR
3. The container can then be tested end-to-end with actual MCP clients

## Testing After Build

Once the container is available, test with:

```bash
# Pull the image
docker pull ghcr.io/github/serena-mcp-server:latest

# Run basic test
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | \
docker run --rm -i \
  -v $(pwd):/workspace:ro \
  ghcr.io/github/serena-mcp-server:latest
```
