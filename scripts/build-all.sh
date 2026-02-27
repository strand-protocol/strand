#!/usr/bin/env bash
# ============================================================
# build-all.sh -- Build every Nexus Protocol module in the
# correct dependency order.
#
# Dependency graph:
#   Phase 1:  nexlink        (Zig, standalone)
#   Phase 2:  nexroute       (C/CMake, depends on nexlink)
#   Phase 3a: nextrust       (Rust, standalone)
#   Phase 3b: nexstream      (Rust, depends on nexlink + nextrust)
#   Phase 4:  nexapi         (Go, depends on nexstream + nextrust)
#   Phase 5:  nexctl         (Go, depends on nexapi)
#             nexus-cloud    (Go, depends on nexapi)
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
# Phase 1: NexLink (Zig)
# -----------------------------------------------------------
step "1/7" "Building nexlink (Zig)"
if (cd "$REPO_ROOT/nexlink" && zig build -Dbackend=mock -Doptimize=ReleaseSafe); then
    ok "nexlink"
else
    fail "nexlink"
    echo "FATAL: nexlink is a root dependency; aborting." >&2
    exit 1
fi

# -----------------------------------------------------------
# Phase 2: NexRoute (C / CMake)
# -----------------------------------------------------------
step "2/7" "Building nexroute (C)"
if (cd "$REPO_ROOT/nexroute" && mkdir -p build && cd build && \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DNEXLINK_INCLUDE_DIR="$REPO_ROOT/nexlink/include" \
             -DNEXLINK_LIB_DIR="$REPO_ROOT/nexlink/zig-out/lib" && \
    make); then
    ok "nexroute"
else
    fail "nexroute"
fi

# -----------------------------------------------------------
# Phase 3a: NexTrust (Rust, standalone)
# -----------------------------------------------------------
step "3/7" "Building nextrust (Rust)"
if (cd "$REPO_ROOT/nextrust" && cargo build --release); then
    ok "nextrust"
else
    fail "nextrust"
fi

# -----------------------------------------------------------
# Phase 3b: NexStream (Rust, depends on nexlink + nextrust)
# -----------------------------------------------------------
step "4/7" "Building nexstream (Rust)"
if (cd "$REPO_ROOT/nexstream" && cargo build --release); then
    ok "nexstream"
else
    fail "nexstream"
fi

# -----------------------------------------------------------
# Phase 4: NexAPI (Go)
# -----------------------------------------------------------
step "5/7" "Building nexapi (Go)"
if (cd "$REPO_ROOT/nexapi" && go build ./...); then
    ok "nexapi"
else
    fail "nexapi"
fi

# -----------------------------------------------------------
# Phase 5a: NexCtl (Go)
# -----------------------------------------------------------
step "6/7" "Building nexctl (Go)"
if (cd "$REPO_ROOT/nexctl" && CGO_ENABLED=0 go build -ldflags "-s -w" -o nexctl .); then
    ok "nexctl"
else
    fail "nexctl"
fi

# -----------------------------------------------------------
# Phase 5b: Nexus Cloud (Go)
# -----------------------------------------------------------
step "7/7" "Building nexus-cloud (Go)"
if (cd "$REPO_ROOT/nexus-cloud" && CGO_ENABLED=0 go build -ldflags "-s -w" -o nexus-cloud ./cmd/nexus-cloud && \
    CGO_ENABLED=0 go build -ldflags "-s -w" -o nexus-allinone ./cmd/nexus-allinone); then
    ok "nexus-cloud"
else
    fail "nexus-cloud"
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
