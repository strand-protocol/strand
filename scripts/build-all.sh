#!/usr/bin/env bash
# ============================================================
# build-all.sh -- Build every Strand Protocol module in the
# correct dependency order.
#
# Dependency graph:
#   Phase 1:  strandlink        (Zig, standalone)
#   Phase 2:  strandroute       (C/CMake, depends on strandlink)
#   Phase 3a: strandtrust       (Rust, standalone)
#   Phase 3b: strandstream      (Rust, depends on strandlink + strandtrust)
#   Phase 4:  strandapi         (Go, depends on strandstream + strandtrust)
#   Phase 5:  strandctl         (Go, depends on strandapi)
#             strand-cloud      (Go, depends on strandapi)
# ============================================================

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

failures=0

step() {
    echo -e "${YELLOW}=== [$1] $2 ===${NC}"
}

ok() {
    echo -e "${GREEN}=== [$1] SUCCESS ===${NC}"
}

fail() {
    echo -e "${RED}=== [$1] FAILED ===${NC}"
    failures=$((failures + 1))
}

# -----------------------------------------------------------
# Phase 1: StrandLink (Zig)
# -----------------------------------------------------------
step "1/7" "Building strandlink (Zig)"
if (cd "$REPO_ROOT/strandlink" && zig build -Dbackend=mock -Doptimize=ReleaseSafe); then
    ok "strandlink"
else
    fail "strandlink"
    echo "FATAL: strandlink is a root dependency; aborting." >&2
    exit 1
fi

# -----------------------------------------------------------
# Phase 2: StrandRoute (C / CMake)
# -----------------------------------------------------------
step "2/7" "Building strandroute (C)"
if (cd "$REPO_ROOT/strandroute" && mkdir -p build && cd build && \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DSTRANDLINK_INCLUDE_DIR="$REPO_ROOT/strandlink/include" \
             -DSTRANDLINK_LIB_DIR="$REPO_ROOT/strandlink/zig-out/lib" && \
    make); then
    ok "strandroute"
else
    fail "strandroute"
fi

# -----------------------------------------------------------
# Phase 3a: StrandTrust (Rust, standalone)
# -----------------------------------------------------------
step "3/7" "Building strandtrust (Rust)"
if (cd "$REPO_ROOT/strandtrust" && cargo build --release); then
    ok "strandtrust"
else
    fail "strandtrust"
fi

# -----------------------------------------------------------
# Phase 3b: StrandStream (Rust, depends on strandlink + strandtrust)
# -----------------------------------------------------------
step "4/7" "Building strandstream (Rust)"
if (cd "$REPO_ROOT/strandstream" && cargo build --release); then
    ok "strandstream"
else
    fail "strandstream"
fi

# -----------------------------------------------------------
# Phase 4: StrandAPI (Go)
# -----------------------------------------------------------
step "5/7" "Building strandapi (Go)"
if (cd "$REPO_ROOT/strandapi" && go build ./...); then
    ok "strandapi"
else
    fail "strandapi"
fi

# -----------------------------------------------------------
# Phase 5a: StrandCtl (Go)
# -----------------------------------------------------------
step "6/7" "Building strandctl (Go)"
if (cd "$REPO_ROOT/strandctl" && CGO_ENABLED=0 go build -ldflags "-s -w" -o strandctl .); then
    ok "strandctl"
else
    fail "strandctl"
fi

# -----------------------------------------------------------
# Phase 5b: Strand Cloud (Go)
# -----------------------------------------------------------
step "7/7" "Building strand-cloud (Go)"
if (cd "$REPO_ROOT/strand-cloud" && CGO_ENABLED=0 go build -ldflags "-s -w" -o strand-cloud ./cmd/strand-cloud && \
    CGO_ENABLED=0 go build -ldflags "-s -w" -o strand-allinone ./cmd/strand-allinone); then
    ok "strand-cloud"
else
    fail "strand-cloud"
fi

# -----------------------------------------------------------
# Summary
# -----------------------------------------------------------
echo ""
if [ "$failures" -eq 0 ]; then
    echo -e "${GREEN}All 7 modules built successfully.${NC}"
    exit 0
else
    echo -e "${RED}${failures} module(s) failed to build.${NC}"
    exit 1
fi
