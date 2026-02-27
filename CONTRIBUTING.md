# Contributing to Strand Protocol

Thank you for your interest in contributing to Strand Protocol.

---

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/strand.git
   cd strand
   ```
3. **Install dependencies**:
   ```bash
   ./scripts/install-deps.sh
   ```
4. **Create a branch** for your change:
   ```bash
   git checkout -b feature/your-feature-name
   ```

---

## Build Order

Modules have a strict dependency order. Always build in this sequence:

```
strandlink -> strandroute
strandlink -> strandtrust -> strandstream -> strandapi -> strandctl / strand-cloud
```

See [CLAUDE.md](CLAUDE.md) for the full dependency graph and build commands.

---

## Making Changes

- Keep changes focused. One logical change per pull request.
- Read the relevant spec file (`01_STRANDLINK_REQUIREMENTS.md` through `07_STRAND_CLOUD_REQUIREMENTS.md`) before modifying a module.
- Do not break the FFI interfaces defined in Section 3 of CLAUDE.md without a corresponding update to all consumers.

---

## Testing Requirements

All pull requests must pass the full test suite before merging.

```bash
make test              # All tests
make test-unit         # Unit tests only (fast)
make test-integration  # Integration tests
```

New code must include tests. Coverage expectations by module:

| Module | Required Coverage |
|--------|------------------|
| strandlink | frame encode/decode, CRC, ring buffer |
| strandroute | SAD encode/decode, routing table, resolution |
| strandtrust | MIC lifecycle, handshake FSM, crypto primitives |
| strandstream | connection FSM, per-mode stream ops, congestion |
| strandapi | StrandBuf encode/decode, message types, client/server |
| strandctl | command parsing, output formatting |
| strand-cloud | API CRUD, controller reconcile, CA issuance |

---

## Code Style

| Language | Formatter | Standard |
|----------|-----------|---------|
| Zig | `zig fmt` | Zig standard style |
| C | `clang-format` (LLVM style) | C17, snake_case |
| P4 | — | P4_16 spec style, snake_case |
| Rust | `rustfmt` | Edition 2021 defaults |
| Go | `gofmt` / `goimports` | Standard Go project layout |

Run formatters before committing:

```bash
# Zig
zig fmt strandlink/src/

# Rust
cargo fmt --all

# Go
gofmt -w ./strandapi/... ./strandctl/... ./strand-cloud/...
```

---

## Submitting a Pull Request

1. Ensure all tests pass locally.
2. Push your branch to your fork.
3. Open a pull request against `main` on `strand-protocol/strand`.
4. Fill out the pull request template.
5. A maintainer will review and provide feedback.

---

## Reporting Issues

Use [GitHub Issues](https://github.com/strand-protocol/strand/issues) to report bugs or request features. Include:

- Module affected
- Steps to reproduce
- Expected vs. actual behavior
- Environment (OS, tool versions)

---

## License

By contributing, you agree that your contributions will be licensed under the [Business Source License 1.1](LICENSE).
