# Serena MCP Server Container Image (Slim)
# Optimized for Python, Go, and TypeScript support with minimal image size

# Stage 1: Build gopls
FROM golang:1.25-alpine AS gopls-builder
# Pin to gopls v0.21.0 which is compatible with Go 1.25
RUN go install golang.org/x/tools/gopls@v0.21.0

# Stage 2: Final image
FROM python:3.11-slim

# OCI labels for GitHub Container Registry
LABEL org.opencontainers.image.title="Serena MCP Server"
LABEL org.opencontainers.image.description="A containerized MCP server with semantic code analysis for Python, Go, and TypeScript"
LABEL org.opencontainers.image.source="https://github.com/github/gh-aw-mcpg/tree/main/containers/serena-mcp-server"
LABEL org.opencontainers.image.documentation="https://github.com/github/gh-aw-mcpg/blob/main/containers/serena-mcp-server/README.md"
LABEL org.opencontainers.image.url="https://github.com/github/gh-aw-mcpg/pkgs/container/serena-mcp-server"
LABEL org.opencontainers.image.vendor="GitHub"

# Set working directory
WORKDIR /app

# Install minimal system dependencies
# - git: required for some Serena operations
# - nodejs/npm: for TypeScript language server
# - ca-certificates: for HTTPS
# - golang-go: Go runtime for Go code analysis
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    nodejs \
    npm \
    ca-certificates \
    golang-go \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Copy pre-built gopls from builder stage
COPY --from=gopls-builder /go/bin/gopls /usr/local/bin/gopls

# Install Serena MCP server from GitHub
RUN pip install --no-cache-dir git+https://github.com/oraios/serena.git || \
    (echo "GitHub installation failed, trying PyPI..." && \
     pip install --no-cache-dir serena-agent)

# Install TypeScript language server (minimal)
RUN npm install -g typescript typescript-language-server && \
    rm -rf /root/.npm /tmp/*

# Create directories for Serena configuration and caching
RUN mkdir -p /workspace /tmp/serena-cache /root/.serena

# Set environment variables for Serena
ENV SERENA_WORKSPACE=/workspace
ENV SERENA_CACHE_DIR=/tmp/serena-cache

# Create a dummy project to pre-warm Serena's LSP cache
# Focus on Python, Go, and TypeScript for codex context
RUN mkdir -p /tmp/dummy-project && \
    echo 'def hello(): pass' > /tmp/dummy-project/main.py && \
    echo 'package main\nfunc main() {}' > /tmp/dummy-project/main.go && \
    echo 'export function hello() {}' > /tmp/dummy-project/index.ts

# Pre-index to warm up LSP caches
RUN serena project create /tmp/dummy-project --name dummy \
    --language python --language go --language typescript \
    --index --log-level INFO --timeout 120 || \
    echo "Pre-indexing completed (some warnings may be expected)"

# Clean up
RUN rm -rf /tmp/dummy-project

# Expose the workspace directory as a volume mount point
VOLUME ["/workspace"]

# Set default entrypoint to run Serena MCP server
ENTRYPOINT ["serena-mcp-server"]

CMD []
