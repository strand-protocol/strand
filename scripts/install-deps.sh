#!/usr/bin/env bash
# ============================================================
# install-deps.sh -- Install all build dependencies for the
# Strand Protocol monorepo.
#
# Supported platforms:
#   - macOS  (Homebrew)
#   - Linux  (apt -- Debian / Ubuntu)
# ============================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# -----------------------------------------------------------
# Detect platform
# -----------------------------------------------------------
OS="$(uname -s)"

install_macos() {
    info "Detected macOS -- using Homebrew"

    if ! command -v brew &>/dev/null; then
        error "Homebrew is not installed. Install it from https://brew.sh"
        exit 1
    fi

    info "Updating Homebrew..."
    brew update

    local packages=(zig rust go cmake)

    for pkg in "${packages[@]}"; do
        if brew list "$pkg" &>/dev/null; then
            info "$pkg is already installed -- upgrading if needed"
            brew upgrade "$pkg" 2>/dev/null || true
        else
            info "Installing $pkg..."
            brew install "$pkg"
        fi
    done

    # Ensure rustup-managed toolchain is up to date if rustup is present.
    if command -v rustup &>/dev/null; then
        info "Updating Rust toolchain via rustup..."
        rustup update stable
    fi

    info "macOS dependency installation complete."
}

install_linux() {
    info "Detected Linux -- using apt"

    if ! command -v apt-get &>/dev/null; then
        error "apt-get not found. This script supports Debian/Ubuntu only."
        error "For other distributions, install: zig, rustc/cargo, go (>=1.22), cmake, clang"
        exit 1
    fi

    info "Updating package lists..."
    sudo apt-get update -y

    # -------------------------------------------------------
    # Core build tools
    # -------------------------------------------------------
    info "Installing core build tools..."
    sudo apt-get install -y \
        build-essential \
        cmake \
        clang \
        curl \
        wget \
        git \
        pkg-config \
        libssl-dev

    # -------------------------------------------------------
    # Go (official tarball -- apt versions are often too old)
    # -------------------------------------------------------
    GO_VERSION="1.22.5"
    if command -v go &>/dev/null; then
        CURRENT_GO="$(go version | awk '{print $3}' | sed 's/go//')"
        info "Go $CURRENT_GO is already installed"
    else
        info "Installing Go $GO_VERSION..."
        wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        export PATH="/usr/local/go/bin:$PATH"
        info "Go $GO_VERSION installed to /usr/local/go"
        warn "Add 'export PATH=/usr/local/go/bin:\$PATH' to your shell profile."
    fi

    # -------------------------------------------------------
    # Rust (via rustup)
    # -------------------------------------------------------
    if command -v rustc &>/dev/null; then
        info "Rust is already installed ($(rustc --version))"
        if command -v rustup &>/dev/null; then
            rustup update stable
        fi
    else
        info "Installing Rust via rustup..."
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        # shellcheck source=/dev/null
        source "$HOME/.cargo/env"
        info "Rust installed: $(rustc --version)"
    fi

    # -------------------------------------------------------
    # Zig (from official release)
    # -------------------------------------------------------
    ZIG_VERSION="0.13.0"
    if command -v zig &>/dev/null; then
        info "Zig is already installed ($(zig version))"
    else
        info "Installing Zig $ZIG_VERSION..."
        ZIG_ARCH="$(uname -m)"
        if [ "$ZIG_ARCH" = "aarch64" ]; then
            ZIG_ARCH="aarch64"
        else
            ZIG_ARCH="x86_64"
        fi
        ZIG_TARBALL="zig-linux-${ZIG_ARCH}-${ZIG_VERSION}.tar.xz"
        wget -q "https://ziglang.org/download/${ZIG_VERSION}/${ZIG_TARBALL}" -O "/tmp/${ZIG_TARBALL}"
        sudo mkdir -p /opt/zig
        sudo tar -C /opt/zig --strip-components=1 -xJf "/tmp/${ZIG_TARBALL}"
        rm "/tmp/${ZIG_TARBALL}"
        sudo ln -sf /opt/zig/zig /usr/local/bin/zig
        info "Zig $ZIG_VERSION installed to /opt/zig"
    fi

    info "Linux dependency installation complete."
}

# -----------------------------------------------------------
# Main
# -----------------------------------------------------------
case "$OS" in
    Darwin)
        install_macos
        ;;
    Linux)
        install_linux
        ;;
    *)
        error "Unsupported platform: $OS"
        error "Supported: macOS (Darwin), Linux"
        exit 1
        ;;
esac

# -----------------------------------------------------------
# Verify
# -----------------------------------------------------------
echo ""
info "Verifying installed tools:"
echo "  zig     : $(zig version 2>/dev/null || echo 'NOT FOUND')"
echo "  rustc   : $(rustc --version 2>/dev/null || echo 'NOT FOUND')"
echo "  cargo   : $(cargo --version 2>/dev/null || echo 'NOT FOUND')"
echo "  go      : $(go version 2>/dev/null || echo 'NOT FOUND')"
echo "  cmake   : $(cmake --version 2>/dev/null | head -1 || echo 'NOT FOUND')"
echo "  clang   : $(clang --version 2>/dev/null | head -1 || echo 'NOT FOUND')"
echo ""
info "All dependencies are ready. Run 'make all' from the repo root to build."
