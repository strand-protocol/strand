# Nexus Protocol

**An AI-native network protocol stack — replacing TCP/IP, HTTP, and DNS for AI workloads.**

[![License: BSL 1.1](https://img.shields.io/badge/License-BSL%201.1-blue.svg)](LICENSE)
[![CI](https://github.com/Artitus/nexus/actions/workflows/ci.yml/badge.svg)](https://github.com/Artitus/nexus/actions/workflows/ci.yml)

Nexus Protocol is a ground-up redesign of the network stack for AI-to-AI communication. It replaces Ethernet framing, IP routing, TCP/UDP transport, and HTTP application protocols with primitives built around model identity, semantic addressing, tensor-aware delivery, and encrypted attestation.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 5 – Application      nexapi  (Go)                    │
│  AI-native message types, NexBuf serialization, streaming   │
├─────────────────────────────────────────────────────────────┤
│  Layer 4 – Identity         nextrust  (Rust)                │
│  Model Identity Certs, 1-RTT handshake, AES-256-GCM/ChaCha │
├─────────────────────────────────────────────────────────────┤
│  Layer 3 – Transport        nexstream  (Rust)               │
│  4-mode delivery: Reliable-Ordered, Reliable-Unordered,     │
│  Best-Effort, Probabilistic — CUBIC/BBR congestion control  │
├─────────────────────────────────────────────────────────────┤
│  Layer 2 – Routing          nexroute  (C + P4)              │
│  Semantic Address Descriptors, capability-based routing,    │
│  RCU routing table, HyParView gossip                        │
├─────────────────────────────────────────────────────────────┤
│  Layer 1 – Framing          nexlink  (Zig)                  │
│  64-byte AI-native frame header, TLV options, CRC-32C,      │
│  lock-free ring buffer, DPDK/XDP/UDP overlay backends       │
└─────────────────────────────────────────────────────────────┘

Control Plane
┌───────────────────────────────┐   ┌───────────────────────┐
│  nexus-cloud  (Go)            │   │  nexctl  (Go)          │
│  API server, fleet controller │   │  kubectl-like CLI      │
│  CA service, multi-tenancy    │   │  for Nexus operators   │
└───────────────────────────────┘   └───────────────────────┘
```

---

## Modules

| Module | Language | Role | Spec |
|--------|----------|------|------|
| `nexlink` | Zig | L1 frame protocol — 64-byte AI-native header, ring buffer | [spec](01_NEXLINK_REQUIREMENTS.md) |
| `nexroute` | C + P4 | L2 semantic routing — SAD-based capability routing | [spec](02_NEXROUTE_REQUIREMENTS.md) |
| `nexstream` | Rust | L3 hybrid transport — 4 delivery modes, CUBIC/BBR | [spec](03_NEXSTREAM_REQUIREMENTS.md) |
| `nextrust` | Rust | L4 model identity — MICs, 1-RTT handshake, crypto | [spec](04_NEXTRUST_REQUIREMENTS.md) |
| `nexapi` | Go | L5 application protocol — NexBuf, inference, streaming | [spec](05_NEXAPI_REQUIREMENTS.md) |
| `nexctl` | Go | CLI tool — operator interface, kubectl-style | [spec](06_NEXCTL_REQUIREMENTS.md) |
| `nexus-cloud` | Go | Control plane — API server, fleet, CA, multi-tenancy | [spec](07_NEXUS_CLOUD_REQUIREMENTS.md) |

---

## Quick Start

### Prerequisites

| Tool | Version | Required For |
|------|---------|-------------|
| Zig | 0.13+ | nexlink |
| GCC / Clang | 12+ / 15+ | nexroute |
| CMake | 3.20+ | nexroute |
| Rust + Cargo | 1.75+ | nexstream, nextrust |
| Go | 1.22+ | nexapi, nexctl, nexus-cloud |

### Build All

```bash
# Install dependencies (macOS with Homebrew / Linux with apt)
./scripts/install-deps.sh

# Build all modules in dependency order
make build

# Run all tests
make test

# Run the MVP demo (Go client -> encrypted NexStream -> Go server over UDP)
./scripts/run-demo.sh
```

### Build Individual Modules

```bash
# nexlink (Zig)
cd nexlink && zig build && zig build test

# nexroute (C + CMake)
cd nexroute && mkdir -p build && cd build && cmake .. && make

# Rust workspace (nextrust + nexstream)
cargo build --workspace
cargo test --workspace

# Go modules (nexapi, nexctl, nexus-cloud)
go build ./nexapi/...
go build ./nexctl/...
go build ./nexus-cloud/...
go test ./nexapi/... ./nexctl/... ./nexus-cloud/...
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

The minimum viable demo runs a Go client sending an `InferenceRequest` to a Go server over an encrypted NexStream connection with semantic addressing and model identity, all on localhost via UDP:

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
- **NexBuf serialization** — FlatBuffers-inspired zero-copy binary, 3x+ faster than JSON

---

## Repository Layout

```
nexus/
  nexlink/        Zig — L1 frame protocol
  nexroute/       C + P4 — L2 semantic routing
  nexstream/      Rust — L3 hybrid transport
  nextrust/       Rust — L4 model identity & crypto
  nexapi/         Go — L5 application protocol
  nexctl/         Go — CLI operator tool
  nexus-cloud/    Go — control plane
  tests/          Cross-module integration tests
  scripts/        Build, test, demo scripts
  docs/           Protocol specifications
  schemas/        NexBuf schema files
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

Nexus Protocol is licensed under the [Business Source License 1.1](LICENSE).

- **Non-production and non-commercial use** (research, education, personal projects, evaluation) is freely permitted.
- **Commercial/production use** requires a commercial license until the Change Date.
- **Change Date**: 2030-02-26 — on this date the license converts to Apache 2.0.
