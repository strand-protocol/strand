{
  description = "Nexus Protocol -- reproducible development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          name = "nexus-dev";

          buildInputs = with pkgs; [
            # Zig (NexLink, NexRoute FFI)
            zig

            # Rust (NexTrust, NexStream)
            rustc
            cargo
            rustfmt
            clippy

            # Go (NexAPI, NexCtl, Nexus Cloud)
            go

            # C/C++ build tools (NexRoute)
            cmake
            clang

            # Supporting tools
            pkg-config
            openssl
            protobuf
            buf

            # Container tooling
            docker-compose
          ];

          shellHook = ''
            echo "=== Nexus Protocol development shell ==="
            echo "  zig     : $(zig version 2>/dev/null || echo 'N/A')"
            echo "  rustc   : $(rustc --version 2>/dev/null || echo 'N/A')"
            echo "  cargo   : $(cargo --version 2>/dev/null || echo 'N/A')"
            echo "  go      : $(go version 2>/dev/null || echo 'N/A')"
            echo "  cmake   : $(cmake --version 2>/dev/null | head -1 || echo 'N/A')"
            echo "  clang   : $(clang --version 2>/dev/null | head -1 || echo 'N/A')"
            echo ""

            # Put local tool wrappers on PATH.
            export PATH="$PWD/scripts:$PATH"

            # Go workspace mode.
            export GOWORK="$PWD/go.work"

            # Rust: point at workspace Cargo.toml.
            export CARGO_WORKSPACE_DIR="$PWD"
          '';
        };
      }
    );
}
