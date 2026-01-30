# Serena MCP Server Container Image
# Provides AI-powered code intelligence with support for Python, Java, JavaScript/TypeScript, and Go

FROM python:3.11-slim

# OCI labels for GitHub Container Registry
LABEL org.opencontainers.image.title="Serena MCP Server"
LABEL org.opencontainers.image.description="A containerized MCP server with semantic code analysis for Python, Java, JavaScript/TypeScript, and Go"
LABEL org.opencontainers.image.source="https://github.com/githubnext/gh-aw-mcpg/tree/main/containers/serena-mcp-server"
LABEL org.opencontainers.image.documentation="https://github.com/githubnext/gh-aw-mcpg/blob/main/containers/serena-mcp-server/README.md"
LABEL org.opencontainers.image.url="https://github.com/githubnext/gh-aw-mcpg/pkgs/container/serena-mcp-server"
LABEL org.opencontainers.image.vendor="GitHub"

# Set working directory
WORKDIR /app

# Install system dependencies required for language servers and build tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Build essentials
    build-essential \
    git \
    curl \
    wget \
    # Java Development Kit (for Java language support and compilation)
    default-jdk \
    # Node.js (for JavaScript/TypeScript language support)
    nodejs \
    npm \
    # Go runtime (for Go language support)
    golang-go \
    # Additional utilities
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

# Set environment variables for language runtimes
ENV JAVA_HOME=/usr/lib/jvm/default-java
ENV PATH="${JAVA_HOME}/bin:${PATH}"
ENV GOPATH=/go
ENV PATH="${GOPATH}/bin:${PATH}"

# Install Serena MCP server from GitHub using pip
# Serena is a Python package, so we can install it directly
# Try GitHub first, then fall back to PyPI package name if available
RUN pip install --no-cache-dir git+https://github.com/oraios/serena.git || \
    (echo "GitHub installation failed, trying PyPI..." && \
     pip install --no-cache-dir serena-agent)

# Install common language servers that Serena may need
# These are installed globally to be available to Serena
RUN npm install -g \
    typescript \
    typescript-language-server \
    && rm -rf /root/.npm

# Install Python language server (pyright is included in Serena dependencies)
# Additional Python tools
RUN pip install --no-cache-dir \
    python-lsp-server \
    pylsp-mypy

# Install gopls (Go language server)
RUN go install golang.org/x/tools/gopls@latest

# Create directories for Serena configuration and caching
RUN mkdir -p /workspace /tmp/serena-cache /root/.serena

# Set environment variables for Serena
ENV SERENA_WORKSPACE=/workspace
ENV SERENA_CACHE_DIR=/tmp/serena-cache

# Create a dummy project to pre-warm Serena's LSP cache
# This initializes the language servers and caches so they're ready at runtime
# Focus on Python, Go, and TypeScript which are the primary languages for codex context
RUN mkdir -p /tmp/dummy-project && \
    echo 'def hello(): pass' > /tmp/dummy-project/main.py && \
    echo 'package main\nfunc main() {}' > /tmp/dummy-project/main.go && \
    echo 'export function hello() {}' > /tmp/dummy-project/index.ts

# Pre-index the dummy project to warm up the LSP caches
# Use --language to specify languages non-interactively and --index to index after creation
# This ensures pyright, gopls, and typescript-language-server are initialized
RUN serena project create /tmp/dummy-project --name dummy \
    --language python --language go --language typescript \
    --index --log-level INFO --timeout 120 || \
    echo "Pre-indexing completed (some warnings may be expected)"

# Clean up the dummy project but keep the caches
RUN rm -rf /tmp/dummy-project

# Expose the workspace directory as a volume mount point
VOLUME ["/workspace"]

# Set default entrypoint to run Serena MCP server
# This will start the MCP server on stdio transport
ENTRYPOINT ["serena-mcp-server"]

# Default arguments (can be overridden)
CMD []
