#!/usr/bin/env bash
# run-benchmarks.sh
#
# Runs Go and Rust benchmarks across the Nexus Protocol monorepo and
# outputs a summary.
#
# Usage: ./scripts/run-benchmarks.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Color output helpers.
if [ -t 1 ]; then
    BOLD='\033[1m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    NC='\033[0m'
else
    BOLD='' GREEN='' YELLOW='' NC=''
fi

log_header() {
    echo ""
    echo -e "${BOLD}=== $1 ===${NC}"
}

source "$HOME/.cargo/env" 2>/dev/null || true

# ------------------------------------------------------------------
# Go benchmarks
# ------------------------------------------------------------------
log_header "Go Benchmarks (tests/bench)"

echo -e "${YELLOW}Running Go benchmarks...${NC}"
(cd "$REPO_ROOT/tests/bench" && go test -bench=. -benchmem -count=1 -timeout 300s ./...) 2>&1 | tee /tmp/nexus_go_bench.txt

# ------------------------------------------------------------------
# Rust benchmarks (NexTrust)
# ------------------------------------------------------------------
log_header "Rust Benchmarks: NexTrust (nextrust)"

echo -e "${YELLOW}Running NexTrust criterion benchmarks...${NC}"
if (cd "$REPO_ROOT" && cargo bench --package nextrust 2>&1) | tee /tmp/nexus_nextrust_bench.txt; then
    echo -e "${GREEN}NexTrust benchmarks complete.${NC}"
else
    echo -e "${YELLOW}NexTrust benchmarks failed or criterion not installed.${NC}"
fi

# ------------------------------------------------------------------
# Rust benchmarks (NexStream)
# ------------------------------------------------------------------
log_header "Rust Benchmarks: NexStream (nexstream)"

echo -e "${YELLOW}Running NexStream criterion benchmarks...${NC}"
if (cd "$REPO_ROOT" && cargo bench --package nexstream 2>&1) | tee /tmp/nexus_nexstream_bench.txt; then
    echo -e "${GREEN}NexStream benchmarks complete.${NC}"
else
    echo -e "${YELLOW}NexStream benchmarks failed or criterion not installed.${NC}"
fi

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
log_header "Benchmark Summary"

echo ""
echo "Go benchmark results:       /tmp/nexus_go_bench.txt"
echo "NexTrust benchmark results: /tmp/nexus_nextrust_bench.txt"
echo "NexStream benchmark results:/tmp/nexus_nexstream_bench.txt"
echo ""

# Extract key stats from Go benchmarks if available.
if [ -f /tmp/nexus_go_bench.txt ]; then
    echo -e "${BOLD}Go benchmark highlights:${NC}"
    grep -E '^Benchmark' /tmp/nexus_go_bench.txt | head -20 || true
fi

echo ""
echo -e "${GREEN}Benchmark run complete.${NC}"
