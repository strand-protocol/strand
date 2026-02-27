# Strand Protocol

**An AI-native network protocol stack — replacing TCP/IP, HTTP, and DNS for AI workloads.**

[![License: BSL 1.1](https://img.shields.io/badge/License-BSL%201.1-blue.svg)](LICENSE)
[![CI](https://github.com/strand-protocol/strand/actions/workflows/ci.yml/badge.svg)](https://github.com/strand-protocol/strand/actions/workflows/ci.yml)

Strand Protocol is a ground-up redesign of the network stack for AI-to-AI communication. It replaces Ethernet framing, IP routing, TCP/UDP transport, and HTTP application protocols with primitives built around model identity, semantic addressing, tensor-aware delivery, and encrypted attestation.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 5 – Application      strandapi  (Go)                   │
│  AI-native message types, StrandBuf serialization, streaming  │
├─────────────────────────────────────────────────────────────┤
│  Layer 4 – Identity         strandtrust  (Rust)               │
│  Model Identity Certs, 1-RTT handshake, AES-256-GCM/ChaCha  │
├─────────────────────────────────────────────────────────────┤
│  Layer 3 – Transport        strandstream  (Rust)              │
│  4-mode delivery: Reliable-Ordered, Reliable-Unordered,      │
│  Best-Effort, Probabilistic — CUBIC/BBR congestion control   │
├─────────────────────────────────────────────────────────────┤
│  Layer 2 – Routing          strandroute  (C + P4)             │
│  Semantic Address Descriptors, capability-based routing,     │
│  RCU routing table, HyParView gossip                         │
├─────────────────────────────────────────────────────────────┤
│  Layer 1 – Framing          strandlink  (Zig)                 │
│  64-byte AI-native frame header, TLV options, CRC-32C,       │
│  lock-free ring buffer, DPDK/XDP/UDP overlay backends        │
└─────────────────────────────────────────────────────────────┘

Control Plane
┌───────────────────────────────┐   ┌───────────────────────┐
│  strand-cloud  (Go)            │   │  strandctl  (Go)        │
│  API server, fleet controller │   │  kubectl-like CLI      │
│  CA service, multi-tenancy    │   │  for Strand operators   │
└───────────────────────────────┘   └───────────────────────┘
```

---

## Modules

| Module | Language | Role | Spec |
|--------|----------|------|------|
| `strandlink` | Zig | L1 frame protocol — 64-byte AI-native header, ring buffer | [spec](01_STRANDLINK_REQUIREMENTS.md) |
| `strandroute` | C + P4 | L2 semantic routing — SAD-based capability routing | [spec](02_STRANDROUTE_REQUIREMENTS.md) |
| `strandstream` | Rust | L3 hybrid transport — 4 delivery modes, CUBIC/BBR | [spec](03_STRANDSTREAM_REQUIREMENTS.md) |
| `strandtrust` | Rust | L4 model identity — MICs, 1-RTT handshake, crypto | [spec](04_STRANDTRUST_REQUIREMENTS.md) |
| `strandapi` | Go | L5 application protocol — StrandBuf, inference, streaming | [spec](05_STRANDAPI_REQUIREMENTS.md) |
| `strandctl` | Go | CLI tool — operator interface, kubectl-style | [spec](06_STRANDCTL_REQUIREMENTS.md) |
| `strand-cloud` | Go | Control plane — API server, fleet, CA, multi-tenancy | [spec](07_STRAND_CLOUD_REQUIREMENTS.md) |

---

## Quick Start

### Prerequisites

| Tool | Version | Required For |
|------|---------|-------------|
| Zig | 0.13+ | strandlink |
| GCC / Clang | 12+ / 15+ | strandroute |
| CMake | 3.20+ | strandroute |
| Rust + Cargo | 1.75+ | strandstream, strandtrust |
| Go | 1.22+ | strandapi, strandctl, strand-cloud |

### Build All

```bash
# Install dependencies (macOS with Homebrew / Linux with apt)
./scripts/install-deps.sh

# Build all modules in dependency order
make build

# Run all tests
make test

# Run the MVP demo (Go client -> encrypted StrandStream -> Go server over UDP)
./scripts/run-demo.sh
```

### Build Individual Modules

```bash
# strandlink (Zig)
cd strandlink && zig build && zig build test

# strandroute (C + CMake)
cd strandroute && mkdir -p build && cd build && cmake .. && make

# Rust workspace (strandtrust + strandstream)
cargo build --workspace
cargo test --workspace

# Go modules (strandapi, strandctl, strand-cloud)
go build ./strandapi/...
go build ./strandctl/...
go build ./strand-cloud/...
go test ./strandapi/... ./strandctl/... ./strand-cloud/...
```

### Docker

```bash
# Build all service images
docker compose build

# Run the full stack
docker compose up
```

---

## MVP Demo

The minimum viable demo runs a Go client sending an `InferenceRequest` to a Go server over an encrypted StrandStream connection with semantic addressing and model identity, all on localhost via UDP:

```bash
./scripts/run-demo.sh
```

---

## Key Design Decisions

- **64-byte fixed frame header** — cache-line aligned, zero heap allocation on hot path
- **Semantic Address Descriptors (SADs)** — route by capability, latency, cost, trust; not IP address
- **Model Identity Certificates (MICs)** — replace X.509; Ed25519 keys, attestation claims, 1-RTT handshake
- **4 delivery modes** — choose Reliable-Ordered, Reliable-Unordered, Best-Effort, or Probabilistic per stream
- **Pure-Go overlay path** — zero CGo for developer adoption; CGo path for production performance
- **StrandBuf serialization** — FlatBuffers-inspired zero-copy binary, 3x+ faster than JSON

---

## Repository Layout

```
strand/
  strandlink/        Zig — L1 frame protocol
  strandroute/       C + P4 — L2 semantic routing
  strandstream/      Rust — L3 hybrid transport
  strandtrust/       Rust — L4 model identity & crypto
  strandapi/         Go — L5 application protocol
  strandctl/         Go — CLI operator tool
  strand-cloud/      Go — control plane
  tests/            Cross-module integration tests
  scripts/          Build, test, demo scripts
  website/          Next.js marketing site
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

Strand Protocol is licensed under the [Business Source License 1.1](LICENSE).

- **Non-production and non-commercial use** (research, education, personal projects, evaluation) is freely permitted.
- **Commercial/production use** requires a commercial license until the Change Date.
- **Change Date**: 2030-02-26 — on this date the license converts to Apache 2.0.
