# Nexus Protocol — Pick Up Where We Left Off

**Paste this entire file as your first message when starting a new Claude Code session in `/Users/mason/Documents/nexus`.**

---

## What This Project Is

Nexus Protocol is an AI-native network protocol stack (replacing TCP/IP, HTTP, DNS for AI workloads). It's a research project / building in public. The monorepo has 7 modules across 4 languages:

| Module | Language | Status | Tests |
|--------|----------|--------|-------|
| `nexlink/` | Zig | BUILT, ALL TESTS PASS | 93 tests |
| `nexroute/` | C + P4 | BUILT, ALL TESTS PASS | 20 tests |
| `nextrust/` | Rust | BUILT, ALL TESTS PASS | 72 tests |
| `nexstream/` | Rust | BUILT, ALL TESTS PASS | 93 tests |
| `nexapi/` | Go | BUILT, ALL TESTS PASS | 26 tests |
| `nexctl/` | Go | BUILT, ALL TESTS PASS | 18 tests |
| `nexus-cloud/` | Go | BUILT, ALL TESTS PASS | 20 tests |
| `tests/integration/` | Go | ALL TESTS PASS | 30 tests |

Total: 372+ tests all passing.

## Build Commands (all verified working)

```bash
# NexLink (Zig)
cd nexlink && zig build && zig build test

# NexRoute (C)
cd nexroute && mkdir -p build && cd build && cmake .. && make

# Rust workspace (NexTrust + NexStream) — NOTE: cargo may not be in PATH
$HOME/.cargo/bin/cargo build --workspace --manifest-path Cargo.toml
$HOME/.cargo/bin/cargo test --workspace --manifest-path Cargo.toml

# Go modules (NexAPI, NexCtl, Nexus Cloud)
go build ./nexapi/...
go build ./nexctl/...
go build ./nexus-cloud/...
go test ./nexapi/... ./nexctl/... ./nexus-cloud/... ./tests/integration/...
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
- [x] Dockerfiles for nexus-cloud, nexctl, nexapi (multi-stage with distroless)
- [x] `docker-compose.yml` and `docker-compose.dev.yml`
- [x] `scripts/build-all.sh`, `test-all.sh`, `install-deps.sh`, `run-demo.sh`, `run-integration-tests.sh`, `run-benchmarks.sh`
- [x] `flake.nix` for reproducible builds
- [x] `.editorconfig`, `CHANGELOG.md`, `SECURITY.md`, `VERSION` (0.1.0)
- [x] Comprehensive test suite: integration tests, benchmarks (Go + Rust criterion), fuzz tests
- [x] Git initialized with 1 commit (specs only)

## What Still Needs To Be Done

### 1. Replace LICENSE with BSL 1.1 (Business Source License)
The current `LICENSE` file is Apache 2.0. The owner explicitly wants **BSL 1.1** so the project is visible/public but nobody else can profit from it. Parameters:
- **Licensor**: Nexus Protocol Contributors
- **Licensed Work**: Nexus Protocol v0.1.0
- **Change Date**: 2030-02-26 (4 years from now)
- **Change License**: Apache License 2.0
- **Additional Use Grant**: Non-production/non-commercial use is permitted. Use for research, education, personal projects, and evaluation is permitted.

### 2. Create README.md
Does NOT exist yet. Needs:
- Project name and tagline
- Architecture diagram (ASCII) showing the 5-layer stack
- Module table with descriptions
- Quick start / build instructions
- License badge (BSL 1.1)
- Link to spec files

### 3. Create CONTRIBUTING.md
Does NOT exist yet. Standard contributing guide: fork, branch, PR, testing requirements, code style per language.

### 4. Create CODE_OF_CONDUCT.md
Does NOT exist yet. **WARNING: A previous agent hit a content filter trying to generate the Contributor Covenant.** Keep it minimal/brief, or just write a short custom one instead of the full Contributor Covenant template.

### 5. Git Commit All Work
Only 1 commit exists (specs). ALL the implementation code, CI configs, Docker files, test suites, etc. need to be committed. Stage everything and commit.

### 6. Push to GitHub
- `gh` CLI is authenticated as the **Artitus** account
- Create new repo: `gh repo create Artitus/nexus --public --source . --push`
- Or if repo already exists, just push

### 7. Verify Docker Builds
Run `docker compose build` to verify the Docker images build correctly.

## Tool Availability

| Tool | Available | Notes |
|------|-----------|-------|
| Zig | Yes | Installed via Homebrew |
| Rust/Cargo | Yes | `$HOME/.cargo/bin/cargo` (may need PATH fix) |
| Go | Yes | Installed via Homebrew |
| CMake | Yes | Installed via Homebrew |
| Clang | Yes | macOS default |
| gh (GitHub CLI) | Yes | Authenticated as Artitus |
| Docker | Check | May or may not be running |

## Spec Files (for reference, do NOT modify)

- `00_NEXUS_MONOREPO_README.md` — Master build guide
- `01_NEXLINK_REQUIREMENTS.md` through `07_NEXUS_CLOUD_REQUIREMENTS.md` — Per-module specs
- `Nexus_Protocol_Business_Plan_GTM.docx.pdf` — Business plan

## One-Shot Instructions

Do all of these in order, using parallel agents where possible:

1. Write the BSL 1.1 LICENSE file
2. Write README.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md
3. Run a full build of all modules to verify everything still works
4. Run all tests across all modules
5. `git add` everything and commit with a descriptive message
6. Create GitHub repo on the Artitus account and push
7. Verify Docker builds if Docker is available
