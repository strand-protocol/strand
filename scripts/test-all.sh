#!/usr/bin/env bash
# ============================================================
# test-all.sh -- Run all tests across every Strand Protocol
# module.  Collects results and exits non-zero on any failure.
# ============================================================

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

total=0
passed=0
failed=0
failed_modules=""

run_test() {
    local name="$1"
    shift
    local dir="$1"
    shift

    total=$((total + 1))
    echo -e "${YELLOW}--- [$name] Running tests ---${NC}"

    if (cd "$dir" && "$@"); then
        echo -e "${GREEN}--- [$name] PASSED ---${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}--- [$name] FAILED ---${NC}"
        failed=$((failed + 1))
        failed_modules="${failed_modules}  - ${name}\n"
    fi
    echo ""
}

# -----------------------------------------------------------
# StrandLink (Zig)
# -----------------------------------------------------------
run_test "strandlink (unit)" "$REPO_ROOT/strandlink" \
    zig build test

# -----------------------------------------------------------
# StrandRoute (C / CMake -- assumes already built)
# -----------------------------------------------------------
if [ -d "$REPO_ROOT/strandroute/build" ]; then
    run_test "strandroute (unit)" "$REPO_ROOT/strandroute/build" \
        ctest --output-on-failure
else
    echo -e "${YELLOW}--- [strandroute] Skipped (no build/ directory) ---${NC}"
fi

# -----------------------------------------------------------
# StrandTrust (Rust)
# -----------------------------------------------------------
run_test "strandtrust (unit)" "$REPO_ROOT/strandtrust" \
    cargo test

run_test "strandtrust (integration)" "$REPO_ROOT/strandtrust" \
    cargo test --test integration_test

# -----------------------------------------------------------
# StrandStream (Rust)
# -----------------------------------------------------------
run_test "strandstream (unit)" "$REPO_ROOT/strandstream" \
    cargo test

run_test "strandstream (integration)" "$REPO_ROOT/strandstream" \
    cargo test --test integration_test

# -----------------------------------------------------------
# StrandAPI (Go)
# -----------------------------------------------------------
run_test "strandapi (unit)" "$REPO_ROOT/strandapi" \
    go test ./... -v -race -count=1

run_test "strandapi (integration)" "$REPO_ROOT/strandapi" \
    go test ./tests/ -tags=integration -v -race -count=1

# -----------------------------------------------------------
# StrandCtl (Go)
# -----------------------------------------------------------
run_test "strandctl (unit)" "$REPO_ROOT/strandctl" \
    go test ./... -v -race -count=1

# -----------------------------------------------------------
# Strand Cloud (Go)
# -----------------------------------------------------------
run_test "strand-cloud (unit)" "$REPO_ROOT/strand-cloud" \
    go test ./... -v -race -count=1

run_test "strand-cloud (integration)" "$REPO_ROOT/strand-cloud" \
    go test ./tests/integration/ -tags=integration -v -race -count=1

# -----------------------------------------------------------
# Summary
# -----------------------------------------------------------
echo "============================================"
echo -e "Total: ${total}  |  ${GREEN}Passed: ${passed}${NC}  |  ${RED}Failed: ${failed}${NC}"
echo "============================================"

if [ "$failed" -gt 0 ]; then
    echo -e "\n${RED}Failed modules:${NC}"
    echo -e "$failed_modules"
    exit 1
fi

echo -e "\n${GREEN}All tests passed.${NC}"
exit 0
