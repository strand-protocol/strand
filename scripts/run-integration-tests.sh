#!/usr/bin/env bash
# run-integration-tests.sh
#
# Builds all modules and runs Go integration + unit tests across the Strand
# Protocol monorepo. Exits non-zero on any failure.
#
# Usage: ./scripts/run-integration-tests.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAILURES=0
PASS=0

# Color output helpers (no-op if not a terminal).
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BOLD='' NC=''
fi

log_header() {
    echo ""
    echo -e "${BOLD}=== $1 ===${NC}"
}

log_pass() {
    echo -e "  ${GREEN}PASS${NC} $1"
    PASS=$((PASS + 1))
}

log_fail() {
    echo -e "  ${RED}FAIL${NC} $1"
    FAILURES=$((FAILURES + 1))
}

# ------------------------------------------------------------------
# Step 1: Build all Go modules
# ------------------------------------------------------------------
log_header "Building Go modules"

GO_MODULES=(
    "strandapi"
    "strand-cloud"
    "strandctl"
)

for mod in "${GO_MODULES[@]}"; do
    if (cd "$REPO_ROOT/$mod" && go build ./... 2>&1); then
        log_pass "build $mod"
    else
        log_fail "build $mod"
    fi
done

# Build test modules (these are not in go.work but still must compile).
TEST_MODULES=(
    "tests/integration"
    "tests/bench"
    "tests/fuzz"
)

for mod in "${TEST_MODULES[@]}"; do
    if (cd "$REPO_ROOT/$mod" && go build ./... 2>&1); then
        log_pass "build $mod"
    else
        log_fail "build $mod"
    fi
done

# ------------------------------------------------------------------
# Step 2: Build Rust crates
# ------------------------------------------------------------------
log_header "Building Rust crates"

source "$HOME/.cargo/env" 2>/dev/null || true

if (cd "$REPO_ROOT" && cargo build 2>&1); then
    log_pass "cargo build (workspace)"
else
    log_fail "cargo build (workspace)"
fi

# ------------------------------------------------------------------
# Step 3: Run each module's unit tests
# ------------------------------------------------------------------
log_header "Running Go unit tests"

for mod in "${GO_MODULES[@]}"; do
    if (cd "$REPO_ROOT/$mod" && go test -v -count=1 -timeout 60s ./... 2>&1); then
        log_pass "unit tests: $mod"
    else
        log_fail "unit tests: $mod"
    fi
done

log_header "Running Rust unit tests"

if (cd "$REPO_ROOT" && cargo test 2>&1); then
    log_pass "cargo test (workspace)"
else
    log_fail "cargo test (workspace)"
fi

# ------------------------------------------------------------------
# Step 4: Run Go integration tests
# ------------------------------------------------------------------
log_header "Running Go integration tests"

if (cd "$REPO_ROOT/tests/integration" && go test -v -count=1 -timeout 120s ./... 2>&1); then
    log_pass "integration tests"
else
    log_fail "integration tests"
fi

# ------------------------------------------------------------------
# Step 5: Run Go fuzz tests (short run to validate they compile and start)
# ------------------------------------------------------------------
log_header "Running Go fuzz tests (short)"

FUZZ_TARGETS=(
    "FuzzStrandBufRoundtrip"
    "FuzzFrameRead"
    "FuzzSADParse"
    "FuzzInferenceResponseDecode"
    "FuzzTensorTransferDecode"
)

for target in "${FUZZ_TARGETS[@]}"; do
    if (cd "$REPO_ROOT/tests/fuzz" && go test -fuzz="$target" -fuzztime=5s -timeout 30s ./... 2>&1); then
        log_pass "fuzz: $target"
    else
        log_fail "fuzz: $target"
    fi
done

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
log_header "Summary"
echo -e "  Passed: ${GREEN}${PASS}${NC}"
echo -e "  Failed: ${RED}${FAILURES}${NC}"

if [ "$FAILURES" -gt 0 ]; then
    echo ""
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}All tests passed.${NC}"
exit 0
