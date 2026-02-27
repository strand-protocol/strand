# Nexus Protocol — Monorepo Build Guide

## For Claude Code Implementation

---

## Repository Structure

```
nexus/
├── README.md                  ← This file
├── Makefile                   # Top-level build orchestration
├── flake.nix                  # Nix flake for reproducible builds
├── nexlink/                   # L1: Frame protocol (Zig)
├── nexroute/                  # L2: Semantic routing (C + P4)
├── nexstream/                 # L3: Hybrid transport (Rust)
├── nextrust/                  # L4: Model identity & crypto (Rust)
├── nexapi/                    # L5: AI application protocol (Go)
├── nexctl/                    # CLI tool (Go)
├── nexus-cloud/               # Control plane (Go + Rust FFI)
├── docs/                      # Protocol specifications
├── schemas/                   # NexBuf schema files
└── scripts/                   # Build, test, CI scripts
```

---

## Build Order (Dependency Graph)

Modules **must** be built in this order due to FFI dependencies:

```
Phase 1 (no dependencies):
  ┌──────────┐
  │ nexlink  │  Zig — builds standalone, produces C headers (nexlink.h)
  └────┬─────┘
       │ C FFI headers
Phase 2 (depends on nexlink):
  ┌────▼─────┐
  │ nexroute │  C — links against nexlink C FFI for frame encode/decode
  └──────────┘
  
Phase 3 (depends on nexlink):
  ┌──────────┐
  │ nextrust │  Rust — standalone crypto library, no FFI dependencies
  └────┬─────┘
       │
  ┌────▼─────┐
  │nexstream │  Rust — depends on nexlink (C FFI via bindgen) and nextrust for encryption
  └────┬─────┘
       │ CGo FFI
Phase 4 (depends on nexstream + nextrust):
  ┌────▼─────┐
  │  nexapi  │  Go — depends on nexstream (via CGo) and nextrust (via CGo)
  └────┬─────┘
       │
Phase 5 (depends on nexapi):
  ┌────▼─────┐   ┌──────────────┐
  │  nexctl  │   │ nexus-cloud  │  Both Go, depend on nexapi client SDK
  └──────────┘   └──────────────┘
```

### Simplified Build Command

```bash
# Full build (all modules, correct order)
make all

# Individual modules
make nexlink          # Phase 1
make nexroute         # Phase 2
make nextrust         # Phase 3a
make nexstream        # Phase 3b
make nexapi           # Phase 4
make nexctl           # Phase 5a
make nexus-cloud      # Phase 5b

# Tests
make test             # All tests
make test-unit        # Unit tests only (fast)
make test-integration # Integration tests (requires running services)
make test-fuzz        # Fuzz tests (long-running)
```

---

## Module Specifications

Each module has a comprehensive requirements document. **Claude Code should read the relevant spec before implementing each module:**

| # | Module | Language | Spec File | Key RFCs |
|---|--------|----------|-----------|----------|
| 1 | `nexlink` | Zig | `01_NEXLINK_REQUIREMENTS.md` | IEEE 802.3, RFC 894, RFC 7348 (VXLAN), RFC 8926 (Geneve) |
| 2 | `nexroute` | C + P4 | `02_NEXROUTE_REQUIREMENTS.md` | RFC 791 (IPv4), RFC 8200 (IPv6), RFC 4271 (BGP), RFC 1035 (DNS), RFC 6830 (LISP) |
| 3 | `nexstream` | Rust | `03_NEXSTREAM_REQUIREMENTS.md` | RFC 9293 (TCP), RFC 768 (UDP), RFC 9000 (QUIC), RFC 9002 (QUIC Loss Detection), RFC 8312 (CUBIC), RFC 9438 (BBR) |
| 4 | `nextrust` | Rust | `04_NEXTRUST_REQUIREMENTS.md` | RFC 8446 (TLS 1.3), RFC 5280 (X.509), RFC 6962 (CT), RFC 9180 (HPKE), RFC 7748 (X25519), RFC 8032 (Ed25519) |
| 5 | `nexapi` | Go | `05_NEXAPI_REQUIREMENTS.md` | RFC 9110 (HTTP), RFC 9113 (HTTP/2), RFC 9114 (HTTP/3), RFC 6455 (WebSocket), RFC 8259 (JSON) |
| 6 | `nexctl` | Go | `06_NEXCTL_REQUIREMENTS.md` | (CLI tool — references kubectl, istioctl patterns) |
| 7 | `nexus-cloud` | Go + Rust | `07_NEXUS_CLOUD_REQUIREMENTS.md` | RFC 8040 (RESTCONF), OpenTelemetry Spec, Kubernetes API conventions |

---

## Implementation Strategy for Claude Code

### Recommended Approach: Bottom-Up, Test-First

**For each module, follow this pattern:**

1. **Read the spec** — Start by reading the full requirements .md file for the module
2. **Set up the project skeleton** — Create the directory structure, build files (build.zig / CMakeLists.txt / Cargo.toml / go.mod)
3. **Define types first** — Implement all data structures, enums, constants from the spec
4. **Write tests** — Write unit tests for encode/decode, state machines, etc. BEFORE implementing
5. **Implement core logic** — Fill in the implementations to make tests pass
6. **Add the mock/test backend** — Implement the mock platform backend for testing without hardware
7. **Wire up FFI** — If the module exports C FFI or consumes another module's FFI, implement the bindings
8. **Integration test** — Test the module against its dependencies

### Phase 1: Start with NexLink

```bash
# Initialize the Zig project
mkdir -p nexlink/src nexlink/tests nexlink/include
cd nexlink
# Implement in this order:
# 1. src/header.zig    — FrameHeader packed struct
# 2. src/crc.zig       — CRC-32C implementation
# 3. src/options.zig   — TLV option parser
# 4. src/frame.zig     — Frame encode/decode using header + options + crc
# 5. tests/frame_test.zig — Roundtrip tests
# 6. src/ring_buffer.zig — Lock-free ring buffer
# 7. src/memory_pool.zig — Pre-allocated memory pool
# 8. src/overlay.zig   — UDP encapsulation (Tier 3)
# 9. src/platform/mock.zig — Mock backend
# 10. Generate include/nexlink.h — C FFI header
```

### Phase 2: NexRoute (after NexLink C headers exist)

```bash
# Initialize the C project
mkdir -p nexroute/src nexroute/include/nexroute nexroute/p4 nexroute/tests
cd nexroute
# Implement in this order:
# 1. include/nexroute/sad.h + src/sad.c         — SAD encoding/decoding
# 2. src/sad_match.c                             — SAD matching engine
# 3. include/nexroute/routing_table.h + src/routing_table.c — RCU routing table
# 4. src/resolver.c                              — Multi-constraint resolution
# 5. src/gossip.c                                — HyParView gossip protocol
# 6. src/forwarding.c                            — Software dataplane
# 7. src/multipath.c                             — Weighted consistent hashing
# 8. p4/headers.p4 + p4/parser.p4               — P4 header definitions
# 9. p4/sad_lookup.p4 + p4/forwarding.p4        — P4 pipeline
```

### Phase 3: NexTrust + NexStream (Rust, can be parallel)

```bash
# NexTrust first (no FFI dependencies)
cd nextrust
# 1. src/crypto/keys.rs     — Ed25519 keypair, Node ID derivation
# 2. src/crypto/*.rs         — All crypto primitives
# 3. src/mic/mod.rs          — MIC data types
# 4. src/mic/builder.rs      — MIC construction
# 5. src/mic/parser.rs       — MIC serialization
# 6. src/mic/validator.rs    — Signature + chain validation
# 7. src/handshake/          — Handshake state machine
# 8. src/encrypt.rs          — AEAD encryption/decryption
# 9. src/zk/                 — Zero-knowledge proofs (P1, can defer)

# Then NexStream (depends on NexLink FFI + NexTrust)
cd nexstream
# 1. src/frame.rs            — NexStream control frame types
# 2. src/transport/mod.rs    — Transport mode trait
# 3. src/transport/reliable_ordered.rs — RO mode (most complex, start here)
# 4. src/rtt.rs + src/loss_detection.rs — RTT estimation + loss detection
# 5. src/congestion/cubic.rs — CUBIC congestion control
# 6. src/retransmission.rs   — Retransmission engine
# 7. src/flow_control.rs     — Window management
# 8. src/stream.rs           — Stream abstraction
# 9. src/mux.rs              — Stream multiplexer
# 10. src/connection.rs      — Connection state machine
# 11. Remaining transport modes: RU, BE, PR
```

### Phase 4-5: NexAPI → NexCtl → Nexus Cloud (Go)

```bash
# NexAPI
cd nexapi
# 1. pkg/nexbuf/             — Binary serialization (core building block)
# 2. pkg/protocol/           — All message type definitions
# 3. pkg/sad/                — SAD builder (Go-native, matches C format)
# 4. pkg/transport/overlay_transport.go — Pure Go overlay (no CGo, works immediately)
# 5. pkg/client/             — Client SDK
# 6. pkg/server/             — Server SDK
# 7. examples/               — Example applications

# NexCtl
cd nexctl
# 1. cmd/root.go             — CLI skeleton with Cobra
# 2. pkg/api/client.go       — Control plane API client
# 3. Commands in order: version → node → route → trust → diagnose → firmware → metrics

# Nexus Cloud
cd nexus-cloud
# 1. pkg/store/              — State store interface + memory backend
# 2. pkg/apiserver/          — REST/gRPC API server
# 3. pkg/controller/         — Fleet controller
# 4. pkg/ca/                 — CA service
# 5. pkg/agent/              — Node agent
# 6. cmd/nexus-allinone/     — All-in-one binary
```

---

## Key Implementation Notes

### Cross-Language FFI

**Zig → C (NexLink exports to NexRoute):**
- NexLink's `build.zig` generates `include/nexlink.h` via `zig build -Demit-h`
- NexRoute's CMakeLists.txt includes this header and links the NexLink static library
- All exported functions use `export` keyword in Zig which produces C-compatible symbols

**Rust → C (NexStream/NexTrust export to Go via CGo):**
- Rust crates expose `extern "C"` functions wrapped in a thin C API
- `cbindgen` generates `.h` files from Rust `extern "C"` functions
- Go consumes via CGo `#cgo LDFLAGS: -lnexstream -lnextrust`
- For the pure Go overlay transport, no CGo is needed (pure Go UDP implementation)

**Critical: Pure Go Overlay Mode**
- NexAPI MUST work in "overlay mode" with zero CGo dependencies
- This means implementing NexLink frame encode/decode, NexStream basic RO mode, and NexTrust handshake natively in Go
- This is essential for developer adoption — developers should be able to `go get` and start using NexAPI immediately
- The CGo path is the optimized path for production deployments

### Testing Without Hardware

All modules include mock/test backends:
- NexLink: `platform/mock.zig` — in-memory loopback
- NexRoute: BMv2 behavioral model for P4 testing
- NexStream: Uses NexLink mock backend
- NexTrust: Fully software-based, no hardware needed
- NexAPI: Overlay transport over localhost UDP
- NexCtl: Mock API server responses
- Nexus Cloud: In-memory state store, mock node agents

### Minimum Viable Implementation (MVP)

For a working demo, implement in this priority:

1. **NexLink frame encode/decode** (Zig) + mock backend + overlay mode
2. **NexTrust keypair generation + MIC creation** (Rust)  
3. **NexStream Reliable-Ordered mode only** (Rust) + CUBIC congestion control
4. **NexAPI client + server** (Go) with overlay transport + InferenceRequest/Response + TokenStream
5. **NexCtl** basic commands: `version`, `node list`, `diagnose ping`
6. **NexRoute** SAD encoding/decoding + basic routing table (defer gossip, P4)
7. **Nexus Cloud** API server + in-memory store + all-in-one binary

This MVP lets you demo: a Go client sending an inference request to a Go server over encrypted NexStream, with semantic addressing and model identity — all running over standard UDP on any machine.

---

## Environment Requirements

| Tool | Version | Purpose |
|------|---------|---------|
| Zig | 0.13+ | NexLink build |
| GCC or Clang | 12+ / 15+ | NexRoute C build |
| Rust (rustc + cargo) | 1.75+ | NexStream + NexTrust |
| Go | 1.22+ | NexAPI + NexCtl + Nexus Cloud |
| CMake | 3.20+ | NexRoute build system |
| p4c | Latest | P4 compilation (optional, for switch targets) |
| BMv2 | Latest | P4 behavioral model testing (optional) |
| Docker | 24+ | Container builds |
| etcd | 3.5+ | Nexus Cloud state store (production) |
| Nix | 2.18+ | Reproducible builds (optional but recommended) |
