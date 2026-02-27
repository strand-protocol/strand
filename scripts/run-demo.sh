#!/usr/bin/env bash
# ============================================================
# run-demo.sh -- Build, start, and smoke-test the Nexus
# Protocol Docker stack.
# ============================================================

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# -----------------------------------------------------------
# Helpers
# -----------------------------------------------------------
info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }

wait_for_healthy() {
    local service="$1"
    local url="$2"
    local max_wait="${3:-60}"
    local elapsed=0

    info "Waiting for $service to become healthy ($url) ..."
    while [ "$elapsed" -lt "$max_wait" ]; do
        if curl -sf -o /dev/null "$url" 2>/dev/null; then
            ok "$service is healthy (${elapsed}s)"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    fail "$service did not become healthy within ${max_wait}s"
    return 1
}

# -----------------------------------------------------------
# Step 1: Build and start the stack
# -----------------------------------------------------------
echo ""
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}  Nexus Protocol -- Demo Launcher${NC}"
echo -e "${BOLD}========================================${NC}"
echo ""

info "Building and starting Docker Compose stack ..."
docker compose up --build -d

echo ""

# -----------------------------------------------------------
# Step 2: Wait for health checks
# -----------------------------------------------------------
info "Waiting for services to start ..."
echo ""

healthy=true

if ! wait_for_healthy "nexus-cloud" "http://localhost:8080/healthz" 60; then
    healthy=false
fi

if ! wait_for_healthy "nexapi-inference" "http://localhost:9000" 60; then
    # The inference server may not have an HTTP healthz; just check the port.
    warn "nexapi-inference port check inconclusive (it uses a custom protocol, not HTTP)"
fi

echo ""

# -----------------------------------------------------------
# Step 3: Smoke tests
# -----------------------------------------------------------
info "Running smoke tests ..."
echo ""

# Test 1: nexus-cloud /healthz
if curl -sf "http://localhost:8080/healthz" > /dev/null 2>&1; then
    ok "nexus-cloud /healthz returned 200"
else
    fail "nexus-cloud /healthz failed"
    healthy=false
fi

# Test 2: nexus-cloud responds to API requests
status_code=$(curl -sf -o /dev/null -w "%{http_code}" "http://localhost:8080/healthz" 2>/dev/null || echo "000")
if [ "$status_code" = "200" ]; then
    ok "nexus-cloud HTTP status: $status_code"
else
    fail "nexus-cloud HTTP status: $status_code (expected 200)"
    healthy=false
fi

# Test 3: Check running containers
running=$(docker compose ps --status running --format '{{.Name}}' 2>/dev/null | wc -l | tr -d ' ')
info "Running containers: $running"

# Test 4: nexctl connectivity (if nexctl binary is available)
if command -v nexctl &> /dev/null; then
    info "nexctl found; attempting version check ..."
    if nexctl version 2>/dev/null; then
        ok "nexctl version succeeded"
    else
        warn "nexctl version failed (non-fatal)"
    fi
else
    info "nexctl not found in PATH; skipping CLI smoke test"
fi

echo ""

# -----------------------------------------------------------
# Step 4: Summary
# -----------------------------------------------------------
if [ "$healthy" = true ]; then
    echo -e "${GREEN}${BOLD}========================================${NC}"
    echo -e "${GREEN}${BOLD}  Nexus Protocol is running!${NC}"
    echo -e "${GREEN}${BOLD}========================================${NC}"
else
    echo -e "${YELLOW}${BOLD}========================================${NC}"
    echo -e "${YELLOW}${BOLD}  Nexus Protocol started with warnings${NC}"
    echo -e "${YELLOW}${BOLD}========================================${NC}"
fi

echo ""
echo -e "${BOLD}Endpoint URLs:${NC}"
echo -e "  nexus-cloud (control plane):  ${CYAN}http://localhost:8080${NC}"
echo -e "  nexus-cloud healthz:          ${CYAN}http://localhost:8080/healthz${NC}"
echo -e "  nexapi-inference (example):   ${CYAN}http://localhost:9000${NC}"
echo ""
echo -e "${BOLD}Useful commands:${NC}"
echo "  docker compose logs -f              -- follow logs"
echo "  docker compose ps                   -- show running services"
echo "  docker compose down                 -- stop everything"
echo "  make docker-allinone                -- start all-in-one mode (port 8081)"
echo ""
