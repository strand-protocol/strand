# Strand Protocol — Pick Up Where We Left Off

**Paste this entire file as your first message when starting a new Claude Code session in the Strand Protocol repo.**

---

## What This Project Is

Strand Protocol is an AI-native network protocol stack (replacing TCP/IP, HTTP, DNS for AI workloads). It's a research project / building in public. The monorepo has 7 modules across 4 languages:

| Module | Language | Status | Tests |
|--------|----------|--------|-------|
| `strandlink/` | Zig | BUILT, ALL TESTS PASS | 93 tests |
| `strandroute/` | C + P4 | BUILT, ALL TESTS PASS | 20 tests |
| `strandtrust/` | Rust | BUILT, ALL TESTS PASS | 72 tests |
| `strandstream/` | Rust | BUILT, ALL TESTS PASS | 93 tests |
| `strandapi/` | Go | BUILT, ALL TESTS PASS | 26 tests |
| `strandctl/` | Go | BUILT, ALL TESTS PASS | 18 tests |
| `strand-cloud/` | Go | BUILT, ALL TESTS PASS | 20 tests |
| `tests/integration/` | Go | ALL TESTS PASS | 30 tests |

Total: 372+ tests all passing.

## Build Commands (all verified working)

```bash
# StrandLink (Zig)
cd strandlink && zig build && zig build test

# StrandRoute (C)
cd strandroute && mkdir -p build && cd build && cmake .. && make

# Rust workspace (StrandTrust + StrandStream) — NOTE: cargo may not be in PATH
$HOME/.cargo/bin/cargo build --workspace --manifest-path Cargo.toml
$HOME/.cargo/bin/cargo test --workspace --manifest-path Cargo.toml

# Go modules (StrandAPI, StrandCtl, Strand Cloud)
go build ./strandapi/...
go build ./strandctl/...
go build ./strand-cloud/...
go test ./strandapi/... ./strandctl/... ./strand-cloud/... ./tests/integration/...
```

## What's Already Done

- [x] All 7 modules fully implemented and tested
- [x] Cargo.toml workspace (root) for Rust crates
- [x] go.work for Go modules
- [x] Makefile (top-level build orchestration)
- [x] CLAUDE.md (implementation guide)
- [x] .gitignore (multi-language)
- [x] GitHub Actions CI: `.github/workflows/ci.yml`, `release.yml`, `security.yml`, `docs.yml`
- [x] `.github/dependabot.yml`, `CODEOWNERS`, PR template, issue templates
- [x] Dockerfiles for strand-cloud, strandctl, strandapi (multi-stage with distroless)
- [x] `docker-compose.yml` and `docker-compose.dev.yml`
- [x] `scripts/build-all.sh`, `test-all.sh`, `install-deps.sh`, `run-demo.sh`, `run-integration-tests.sh`, `run-benchmarks.sh`
- [x] `flake.nix` for reproducible builds
- [x] `.editorconfig`, `CHANGELOG.md`, `SECURITY.md`, `VERSION` (0.1.0)
- [x] Comprehensive test suite: integration tests, benchmarks (Go + Rust criterion), fuzz tests
- [x] Next.js marketing website with landing page
- [x] Full rebrand to Strand Protocol

## Spec Files (for reference)

- `00_STRAND_MONOREPO_README.md` — Master build guide
- `01_STRANDLINK_REQUIREMENTS.md` through `07_STRAND_CLOUD_REQUIREMENTS.md` — Per-module specs

## Tool Availability

| Tool | Available | Notes |
|------|-----------|-------|
| Zig | Yes | Installed via Homebrew |
| Rust/Cargo | Yes | `$HOME/.cargo/bin/cargo` (may need PATH fix) |
| Go | Yes | Installed via Homebrew |
| CMake | Yes | Installed via Homebrew |
| Clang | Yes | macOS default |
| gh (GitHub CLI) | Yes | Authenticated |
| Docker | Check | May or may not be running |
| Node.js | Yes | For website development |
