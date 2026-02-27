#!/usr/bin/env bash
# run-benchmarks.sh
#
# Runs Go and Rust benchmarks across the Strand Protocol monorepo and
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
(cd "$REPO_ROOT/tests/bench" && go test -bench=. -benchmem -count=1 -timeout 300s ./...) 2>&1 | tee /tmp/strand_go_bench.txt

# ------------------------------------------------------------------
# Rust benchmarks (StrandTrust)
# ------------------------------------------------------------------
log_header "Rust Benchmarks: StrandTrust (strandtrust)"

echo -e "${YELLOW}Running StrandTrust criterion benchmarks...${NC}"
if (cd "$REPO_ROOT" && cargo bench --package strandtrust 2>&1) | tee /tmp/strand_strandtrust_bench.txt; then
    echo -e "${GREEN}StrandTrust benchmarks complete.${NC}"
else
    echo -e "${YELLOW}StrandTrust benchmarks failed or criterion not installed.${NC}"
fi

# ------------------------------------------------------------------
# Rust benchmarks (StrandStream)
# ------------------------------------------------------------------
log_header "Rust Benchmarks: StrandStream (strandstream)"

echo -e "${YELLOW}Running StrandStream criterion benchmarks...${NC}"
if (cd "$REPO_ROOT" && cargo bench --package strandstream 2>&1) | tee /tmp/strand_strandstream_bench.txt; then
    echo -e "${GREEN}StrandStream benchmarks complete.${NC}"
else
    echo -e "${YELLOW}StrandStream benchmarks failed or criterion not installed.${NC}"
fi

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
log_header "Benchmark Summary"

echo ""
echo "Go benchmark results:          /tmp/strand_go_bench.txt"
echo "StrandTrust benchmark results:  /tmp/strand_strandtrust_bench.txt"
echo "StrandStream benchmark results: /tmp/strand_strandstream_bench.txt"
echo ""

# Extract key stats from Go benchmarks if available.
if [ -f /tmp/strand_go_bench.txt ]; then
    echo -e "${BOLD}Go benchmark highlights:${NC}"
    grep -E '^Benchmark' /tmp/strand_go_bench.txt | head -20 || true
fi

echo ""
echo -e "${GREEN}Benchmark run complete.${NC}"
