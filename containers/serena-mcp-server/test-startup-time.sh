#!/bin/bash
# Test script to measure Serena MCP server startup time through the gateway
# This script launches the gateway with a stdin config and times the health check

set -e

# Parse command line arguments
usage() {
  echo "Usage: $0 [-w|--workspace <path>] [-h|--help]"
  echo ""
  echo "Options:"
  echo "  -w, --workspace <path>  Workspace directory to mount (default: current directory)"
  echo "  -h, --help              Show this help message"
  exit 1
}

WORKSPACE_ARG=""
while [[ $# -gt 0 ]]; do
  case $1 in
    -w|--workspace)
      WORKSPACE_ARG="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1"
      usage
      ;;
  esac
done

# Configuration
export MCP_GATEWAY_PORT="8080"
export MCP_GATEWAY_DOMAIN="host.docker.internal"
export MCP_GATEWAY_API_KEY=$(openssl rand -base64 45 | tr -d '/+=')
export WORKSPACE="${WORKSPACE_ARG:-${PWD}}"

GATEWAY_IMAGE="ghcr.io/githubnext/gh-aw-mcpg:v0.0.84"
SERENA_IMAGE="serena-mcp-server:test"

echo "=== Serena Startup Time Test ==="
echo "Gateway image: ${GATEWAY_IMAGE}"
echo "Serena image: ${SERENA_IMAGE}"
echo "Workspace: ${WORKSPACE}"
echo "Gateway port: ${MCP_GATEWAY_PORT}"
echo ""

# Create the config JSON with variable substitution
CONFIG_JSON=$(cat <<EOF
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "${SERENA_IMAGE}",
      "args": ["--network", "host"],
      "entrypoint": "serena",
      "entrypointArgs": ["start-mcp-server", "--context", "codex", "--project", "${WORKSPACE}"],
      "mounts": ["${WORKSPACE}:${WORKSPACE}:rw"]
    }
  },
  "gateway": {
    "port": ${MCP_GATEWAY_PORT},
    "domain": "${MCP_GATEWAY_DOMAIN}",
    "apiKey": "${MCP_GATEWAY_API_KEY}"
  }
}
EOF
)

echo "Config:"
echo "${CONFIG_JSON}" | jq .
echo ""

# Clean up any existing container
docker rm -f mcpg-test 2>/dev/null || true

echo "Starting gateway container..."
START_TIME=$(date +%s.%N)

# Start the gateway container in the background with stdin pipe
# Use -i to keep stdin open for config, run in background with &
# Note: --network host doesn't work on macOS Docker Desktop, use -p for port mapping
echo "${CONFIG_JSON}" | docker run -i --rm --name mcpg-test \
  -p "${MCP_GATEWAY_PORT}:${MCP_GATEWAY_PORT}" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "${WORKSPACE}:${WORKSPACE}:rw" \
  -e MCP_GATEWAY_PORT="${MCP_GATEWAY_PORT}" \
  -e MCP_GATEWAY_DOMAIN="${MCP_GATEWAY_DOMAIN}" \
  -e MCP_GATEWAY_API_KEY="${MCP_GATEWAY_API_KEY}" \
  "${GATEWAY_IMAGE}" &

DOCKER_PID=$!

echo "Gateway container started, waiting for health check..."
echo ""

# Poll the health endpoint
RETRY_COUNT=0
MAX_RETRIES=120
HEALTH_URL="http://localhost:${MCP_GATEWAY_PORT}/health"

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  RETRY_COUNT=$((RETRY_COUNT + 1))
  
  if curl -s -o /dev/null -w "%{http_code}" "${HEALTH_URL}" 2>/dev/null | grep -q "200"; then
    END_TIME=$(date +%s.%N)
    ELAPSED=$(echo "${END_TIME} - ${START_TIME}" | bc)
    
    echo "✓ Health check succeeded after ${RETRY_COUNT} attempts"
    echo ""
    echo "=== RESULTS ==="
    echo "Startup time: ${ELAPSED} seconds"
    echo "Retries: ${RETRY_COUNT}"
    echo ""
    
    # Show health response
    echo "Health response:"
    curl -s "${HEALTH_URL}" | jq .
    echo ""
    
    # Show container logs
    echo "Gateway logs (last 30 lines):"
    docker logs mcpg-test 2>&1 | tail -30
    
    # Cleanup
    echo ""
    echo "Cleaning up..."
    docker rm -f mcpg-test >/dev/null 2>&1
    kill $DOCKER_PID 2>/dev/null || true
    
    echo ""
    echo "=== STARTUP TIME: ${ELAPSED} seconds ==="
    exit 0
  fi
  
  # Show progress every 10 retries
  if [ $((RETRY_COUNT % 10)) -eq 0 ]; then
    CURRENT_TIME=$(date +%s.%N)
    ELAPSED=$(echo "${CURRENT_TIME} - ${START_TIME}" | bc)
    echo "  Waiting... (${RETRY_COUNT} retries, ${ELAPSED}s elapsed)"
  fi
  
  sleep 1
done

# Timeout
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "${END_TIME} - ${START_TIME}" | bc)

echo "✗ Health check failed after ${MAX_RETRIES} attempts (${ELAPSED}s)"
echo ""
echo "Container logs:"
docker logs mcpg-test 2>&1 | tail -50

# Cleanup
docker rm -f mcpg-test >/dev/null 2>&1
kill $DOCKER_PID 2>/dev/null || true

exit 1
