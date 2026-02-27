#!/usr/bin/env bash
# ============================================================
# test-all.sh -- Run all tests across every Nexus Protocol
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
# NexLink (Zig)
# -----------------------------------------------------------
run_test "nexlink (unit)" "$REPO_ROOT/nexlink" \
    zig build test

# -----------------------------------------------------------
# NexRoute (C / CMake -- assumes already built)
# -----------------------------------------------------------
if [ -d "$REPO_ROOT/nexroute/build" ]; then
    run_test "nexroute (unit)" "$REPO_ROOT/nexroute/build" \
        ctest --output-on-failure
else
    echo -e "${YELLOW}--- [nexroute] Skipped (no build/ directory) ---${NC}"
fi

# -----------------------------------------------------------
# NexTrust (Rust)
# -----------------------------------------------------------
run_test "nextrust (unit)" "$REPO_ROOT/nextrust" \
    cargo test

run_test "nextrust (integration)" "$REPO_ROOT/nextrust" \
    cargo test --test integration_test

# -----------------------------------------------------------
# NexStream (Rust)
# -----------------------------------------------------------
run_test "nexstream (unit)" "$REPO_ROOT/nexstream" \
    cargo test

run_test "nexstream (integration)" "$REPO_ROOT/nexstream" \
    cargo test --test integration_test

# -----------------------------------------------------------
# NexAPI (Go)
# -----------------------------------------------------------
run_test "nexapi (unit)" "$REPO_ROOT/nexapi" \
    go test ./... -v -race -count=1

run_test "nexapi (integration)" "$REPO_ROOT/nexapi" \
    go test ./tests/ -tags=integration -v -race -count=1

# -----------------------------------------------------------
# NexCtl (Go)
# -----------------------------------------------------------
run_test "nexctl (unit)" "$REPO_ROOT/nexctl" \
    go test ./... -v -race -count=1

# -----------------------------------------------------------
# Nexus Cloud (Go)
# -----------------------------------------------------------
run_test "nexus-cloud (unit)" "$REPO_ROOT/nexus-cloud" \
    go test ./... -v -race -count=1

run_test "nexus-cloud (integration)" "$REPO_ROOT/nexus-cloud" \
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
