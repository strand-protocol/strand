# Nexus Protocol -- Master Build Guide (CLAUDE.md)

This file is the single source of truth for building the Nexus Protocol monorepo.
Every agent working on any module MUST read this file first. It specifies the
build order, dependency graph, cross-module FFI contracts, architecture decisions,
testing strategy, and MVP priorities.

---

## 1. Build Order and Dependency Graph

Modules MUST be built in this exact order. A module in a later phase may only
depend on modules from earlier phases (never the reverse).

```
Phase 1 -- No dependencies
  nexlink      (Zig)     Standalone. Produces libnexlink.a + include/nexlink.h

Phase 2 -- Depends on nexlink
  nexroute     (C + P4)  Links libnexlink.a, includes nexlink.h

Phase 3a -- No FFI dependencies (standalone)
  nextrust     (Rust)    Standalone crypto library. Produces libnextrust.a + nextrust.h (via cbindgen)

Phase 3b -- Depends on nexlink + nextrust
  nexstream    (Rust)    Binds nexlink C FFI via bindgen; depends on nextrust for encryption.
                         Produces libnexstream.a + nexstream.h (via cbindgen)

Phase 4 -- Depends on nexstream + nextrust (via CGo) OR pure-Go overlay
  nexapi       (Go)      CGo path links libnexstream + libnextrust. Pure-Go overlay path has zero CGo.

Phase 5 -- Depends on nexapi
  nexctl       (Go)      Uses nexapi client SDK.
  nexus-cloud  (Go)      Uses nexapi client SDK + NexTrust Rust FFI (CGo) for CA operations.
```

Simplified ASCII dependency DAG:

```
nexlink -----> nexroute
   |
   +---------> nexstream -----> nexapi ------> nexctl
   |               ^               |
   |               |               +---------> nexus-cloud
   +-- nextrust ---+
```

---

## 2. Module Summaries and Key Architecture Decisions

### 2.1 nexlink (Zig) -- L1 Frame Protocol

Spec: `01_NEXLINK_REQUIREMENTS.md`

- Replaces IEEE 802.3 Ethernet frame format with an AI-native 64-byte fixed header.
- Header fields: version(4b), flags(8b), frame_type(16b), frame_length(32b),
  stream_id(32b), sequence_number(32b), src/dst node_id(128b each),
  priority(4b), qos_class(4b), tensor_dtype(8b), tensor_alignment(16b),
  options_length(16b), timestamp(64b).
- Options are TLV-encoded, max 256 bytes. 8 defined option types (0x01-0x08).
- CRC-32C (Castagnoli) over entire frame excluding CRC field.
- Lock-free SPSC ring buffer modeled after io_uring (cache-line-aligned head/tail).
- Platform backends: DPDK, XDP, overlay (UDP port 6477), mock (in-memory loopback).
- Overlay header: 8 bytes `[Version:4][Flags:4][Reserved:8][VNI:24][Reserved:24]`.
- Build: `zig build` with `-Dbackend=mock` (default) or `dpdk`/`xdp`/`overlay`.
- Key constraint: zero heap allocations on hot path after init.
- Performance targets: encode < 200ns, decode < 300ns, ring reserve+commit < 50ns.

### 2.2 nexroute (C + P4) -- L2 Semantic Routing

Spec: `02_NEXROUTE_REQUIREMENTS.md`

- Replaces IP routing + BGP with Semantic Address Descriptors (SADs).
- SAD binary format: version(8b), num_fields(8b), total_length(16b), then
  field entries (type:8b, length:8b, value:variable). Max 16 fields, 512 bytes.
- 11 defined SAD field types (MODEL_ARCH, CAPABILITY, CONTEXT_WINDOW, etc.).
- Resolution scoring: weighted multi-constraint function. Default weights:
  CAPABILITY=0.3, LATENCY=0.25, COST=0.2, CONTEXT_WINDOW=0.15, TRUST=0.1.
  Hard constraints (CONTEXT_WINDOW, TRUST_LEVEL, REGION_EXCLUDE) cause immediate disqualification.
- Routing table: RCU-based lock-free concurrent data structure (liburcu).
  Target: 100K+ entries, < 10us lookup.
- Gossip: HyParView protocol, default fanout=3, interval=1s, TTL=30s.
- P4 dataplane: TCAM tables for SAD field matching, exact-match for Node ID forwarding.
- Build: CMake. `-DNEXLINK_INCLUDE_DIR=../nexlink/include`.
- Links against nexlink static library for frame encode/decode.

### 2.3 nextrust (Rust) -- L4 Model Identity and Crypto

Spec: `04_NEXTRUST_REQUIREMENTS.md`

- Replaces X.509/TLS with Model Identity Certificates (MICs) and a 1-RTT
  mutual authentication handshake.
- MIC fields: version, serial_number(32B), node_id(16B), public_key(32B Ed25519),
  issuer_node_id, issuer_signature(64B Ed25519), validity timestamps, claims[], extensions[],
  optional provenance chain (Merkle tree).
- 10 attestation claim types (ARCHITECTURE_HASH, PARAMETER_COUNT, etc.).
- Handshake: TRUST_HELLO -> TRUST_ACCEPT -> TRUST_FINISH (1-RTT).
  Key exchange: X25519 ephemeral. Key derivation: HKDF-SHA256.
- Cipher suites: AES-256-GCM or ChaCha20-Poly1305, both with Ed25519 + X25519.
- ZK attestation: Groth16 zk-SNARKs on BLS12-381 (via arkworks). P1 priority.
- Node ID = first 128 bits of SHA-256(Ed25519 public key).
- Key Rust crates: ed25519-dalek, x25519-dalek, aes-gcm, chacha20poly1305, hkdf, sha2,
  ark-groth16 (feature-gated `zk`).
- Exports `extern "C"` API; cbindgen generates nextrust.h for Go CGo consumption.

### 2.4 nexstream (Rust) -- L3 Hybrid Transport

Spec: `03_NEXSTREAM_REQUIREMENTS.md`

- Replaces TCP/UDP with 4 delivery modes multiplexed on one connection:
  - Reliable-Ordered (RO) -- TCP-like byte stream, SACK, retransmit.
  - Reliable-Unordered (RU) -- message-oriented, exactly-once, any order.
  - Best-Effort (BE) -- fire-and-forget.
  - Probabilistic (PR) -- configurable delivery probability, FEC.
- Stream IDs: 32-bit. Client-initiated = odd (0x00000001-0x7FFFFFFF),
  server-initiated = even (0x80000000-0xFFFFFFFE). 0x00000000 and 0xFFFFFFFF reserved.
- Connection lifecycle: CONN_INIT -> CONN_ACCEPT -> ESTABLISHED -> CONN_CLOSE.
- 13 control frame types (0x01-0x40) carried in NexLink frame_type=StreamControl.
- Congestion control: CUBIC (default, RFC 8312), BBR (optional, RFC 9438).
  Pluggable via CongestionController trait.
- Loss detection: packet threshold (3 dup ACKs) + time threshold (9/8 x SRTT).
  RTT: Jacobson/Karels EWMA (RFC 6298).
- Depends on nexlink C FFI (via bindgen) for ring buffer integration.
- Depends on nextrust for session encryption (AEAD encrypt/decrypt).
- no_std support for embedded targets (feature-gated).
- Key Rust crates: tokio, bytes, crossbeam, parking_lot, tracing.
- Exports `extern "C"` API; cbindgen generates nexstream.h for Go CGo consumption.

### 2.5 nexapi (Go) -- L5 AI Application Protocol

Spec: `05_NEXAPI_REQUIREMENTS.md`

- Replaces HTTP/REST/gRPC/WebSocket with AI-native message types.
- 18 message types (INFERENCE_REQUEST through CUSTOM, 0x0001-0x0100).
- Message header: 16 bytes (type:16b, flags:16b, request_id:32b, body_length:32b, reserved:32b).
- NexBuf serialization: FlatBuffers-inspired zero-copy binary format with field IDs
  via struct tags (`nexbuf:"N"`). Must be 3x+ faster than JSON.
- Token streaming: server opens RU stream, sends TOKEN_STREAM_CHUNK with sequence
  numbers for client-side reassembly.
- Tensor transfer: dedicated RU stream with NexLink tensor_payload flag.
- CRITICAL: Pure-Go overlay transport MUST work with zero CGo dependencies.
  This means Go-native implementations of NexLink frame encode/decode, basic NexStream
  RO mode, and NexTrust handshake. The CGo path is the optimized production path.
- HTTP bridge: OpenAI-compatible REST -> NexAPI translation (P1).
- Go module path: `github.com/nexus-protocol/nexapi`.
- Key deps: golang.org/x/sync, go.opentelemetry.io/otel, prometheus/client_golang, uber/zap.
- 13 error codes (0x0000-0x00FF).

### 2.6 nexctl (Go) -- CLI Tool

Spec: `06_NEXCTL_REQUIREMENTS.md`

- kubectl-like CLI for Nexus operators.
- Top-level commands: node, firmware, route, stream, trust, diagnose, config, metrics, version.
- All commands support --output (table/json/yaml/wide), --context, --verbose, --dry-run.
- Config file: ~/.nexus/config.yaml.
- TUI dashboard via bubbletea/lipgloss.
- Built with cobra + viper.
- Single static binary < 30MB, cross-platform (linux/macos amd64+arm64, windows amd64).
- Depends on nexapi client SDK for control plane communication.

### 2.7 nexus-cloud (Go + Rust FFI) -- Control Plane

Spec: `07_NEXUS_CLOUD_REQUIREMENTS.md`

- Kubernetes-style control plane: API server + fleet controller + CA service +
  metrics aggregator + config distributor + tenant manager.
- State store: etcd (production) or in-memory (dev/testing).
- API server: REST (OpenAPI 3.1) + gRPC. Auth: API key, mTLS (NexTrust MICs), OIDC.
- RBAC: admin/operator/viewer per tenant.
- Fleet controller: reconciliation loop, leader election via etcd lease.
- Config distribution: push-based, xDS-like protocol, < 5s latency for 1000 nodes.
- CA service: MIC issuance, revocation, transparency log. HSM integration (P2).
- Multi-tenancy: full isolation, resource quotas, usage tracking.
- Node agent: runs on every Nexus node, heartbeat every 10s, auto-registers.
- Deployable as separate microservices on K8s or single all-in-one binary.
- Depends on nexapi client SDK + NexTrust Rust FFI (CGo) for MIC signing.

---

## 3. Cross-Module FFI Interfaces

### 3.1 nexlink -> nexroute (Zig exports C API)

Produced by: `nexlink/build.zig -Demit-h`
Header file: `nexlink/include/nexlink.h`
Library: `nexlink/zig-out/lib/libnexlink.a`

Key exports:
```c
// Frame encode/decode
int nexlink_frame_encode(const nexlink_frame_header_t *header,
    const uint8_t *options, uint16_t options_len,
    const uint8_t *payload, uint32_t payload_len,
    uint8_t *out_buf, uint32_t out_buf_len, uint32_t *out_frame_len);

int nexlink_frame_decode(const uint8_t *buf, uint32_t buf_len,
    nexlink_frame_header_t *out_header,
    const uint8_t **out_payload, uint32_t *out_payload_len);

// Ring buffer
nexlink_ring_buffer_t *nexlink_ring_alloc(uint32_t num_slots, uint32_t slot_size);
void nexlink_ring_free(nexlink_ring_buffer_t *ring);
uint8_t *nexlink_ring_reserve(nexlink_ring_buffer_t *ring);
void nexlink_ring_commit(nexlink_ring_buffer_t *ring);
const uint8_t *nexlink_ring_peek(nexlink_ring_buffer_t *ring);
void nexlink_ring_release(nexlink_ring_buffer_t *ring);
```

Consumer: NexRoute CMakeLists.txt adds `-I ../nexlink/include` and links `-lnexlink`.

### 3.2 nexlink -> nexstream (Zig exports C API, Rust consumes via bindgen)

Same header as above (`nexlink/include/nexlink.h`).
NexStream's `build.rs` runs bindgen against this header to generate Rust FFI bindings.
Cargo feature: `nexlink-ffi` (default on).

### 3.3 nextrust -> nexstream (Rust crate dependency)

Direct Rust crate dependency in nexstream's Cargo.toml:
```toml
[dependencies]
nextrust = { path = "../nextrust" }
```
No FFI needed -- same language.

### 3.4 nextrust -> nexapi (Rust exports C API, Go consumes via CGo)

Produced by: `cbindgen` during `cargo build` of nextrust.
Header file: `nextrust/target/nextrust.h`
Library: `nextrust/target/release/libnextrust.a`

Go CGo directive in nexapi:
```go
// #cgo LDFLAGS: -L${SRCDIR}/../nextrust/target/release -lnextrust
// #cgo CFLAGS: -I${SRCDIR}/../nextrust/target
// #include "nextrust.h"
import "C"
```

### 3.5 nexstream -> nexapi (Rust exports C API, Go consumes via CGo)

Produced by: `cbindgen` during `cargo build` of nexstream.
Header file: `nexstream/target/nexstream.h`
Library: `nexstream/target/release/libnexstream.a`

Go CGo directive in nexapi:
```go
// #cgo LDFLAGS: -L${SRCDIR}/../nexstream/target/release -lnexstream
// #cgo CFLAGS: -I${SRCDIR}/../nexstream/target
// #include "nexstream.h"
import "C"
```

### 3.6 Pure-Go Overlay Path (NO FFI)

nexapi MUST also compile and run with zero CGo dependencies. This is the
`overlay_transport.go` path. It re-implements in pure Go:
- NexLink frame header encode/decode (64-byte header + TLV options + CRC-32C)
- Basic NexStream Reliable-Ordered mode (simplified)
- NexTrust handshake + AEAD encryption (using Go's crypto/ed25519, golang.org/x/crypto)
- UDP overlay transport (port 6477)

This path is used for `go get` / developer adoption. The CGo path is the production-optimized path.

---

## 4. Testing Strategy

### 4.1 Per-Module Testing

| Module     | Unit Tests              | Integration Tests          | Fuzz Tests           | Benchmarks           |
|------------|-------------------------|----------------------------|----------------------|----------------------|
| nexlink    | frame, header, options, crc, ring_buffer | overlay encap/decap via UDP | frame parser (10M+ iter) | encode/decode latency, ring ops |
| nexroute   | SAD encode/decode, matching, routing table CRUD | NexLink frame -> routing -> forwarding | SAD parser | lookup latency (100K entries) |
| nextrust   | MIC create/serialize/validate, handshake, crypto | Full handshake -> encrypted stream | MIC parser, handshake FSM | handshake latency, encrypt throughput |
| nexstream  | connection FSM, per-mode stream, mux, congestion | NexLink mock -> NexStream roundtrip | frame decoder, connection FSM | throughput, latency per mode |
| nexapi     | protocol encode/decode, NexBuf, client/server | client -> server -> response (overlay) | NexBuf decoder | NexBuf vs JSON, inference latency |
| nexctl     | command parsing, API client (mock server) | E2E CLI tests | -- | -- |
| nexus-cloud| API server, controller, CA, tenant, configdist | Fleet management, firmware rollout | -- | API latency |

### 4.2 Cross-Module Integration Tests

Run after all modules build successfully:

1. `nexlink` frame encode -> `nexroute` forwarding -> `nexlink` frame output
2. `nextrust` handshake -> `nexstream` encrypted RO stream -> data exchange
3. `nexapi` client -> overlay transport -> `nexapi` server -> inference response
4. `nexctl` CLI -> `nexus-cloud` API server -> state store round-trip

### 4.3 Test Commands

```bash
make test             # All tests (all modules)
make test-unit        # Unit tests only (fast, no external deps)
make test-integration # Integration tests (requires built modules)
make test-fuzz        # Fuzz tests (long-running)
```

---

## 5. MVP Priorities (Minimum Viable Demo)

Implement in this order to reach a working demo as fast as possible:

1. **nexlink** -- frame encode/decode + CRC-32C + mock backend + overlay mode.
   Skip: DPDK, XDP, firmware backends, fragmentation, ring buffer DMA.

2. **nextrust** -- Ed25519 keypair generation, Node ID derivation, MIC builder/parser/validator,
   1-RTT handshake, AES-256-GCM encryption.
   Skip: ZK proofs, transparency log, provenance chain, session resumption.

3. **nexstream** -- Reliable-Ordered mode only + CUBIC congestion control +
   connection lifecycle + basic stream mux.
   Skip: RU/BE/PR modes, BBR, FEC, no_std, multipath.

4. **nexapi** -- NexBuf encoder/decoder + InferenceRequest/Response +
   TokenStream (start/chunk/end) + client SDK + server SDK + pure-Go overlay transport.
   Skip: tensor transfer, agent delegation, HTTP bridge, CGo transport.

5. **nexctl** -- cobra skeleton + version + node list + diagnose ping.
   Skip: firmware, TUI, capture, most subcommands.

6. **nexroute** -- SAD encode/decode + SAD matching + basic routing table + resolver.
   Skip: gossip protocol, P4 dataplane, SONiC integration, multipath.

7. **nexus-cloud** -- API server + in-memory store + node CRUD + all-in-one binary.
   Skip: etcd, fleet controller, CA service, multi-tenancy, metrics, K8s deploy.

MVP demo flow: Go client sends InferenceRequest to Go server over encrypted
NexStream, with semantic addressing and model identity, all running over UDP
on localhost.

---

## 6. Environment Requirements

| Tool                  | Version  | Required For                           |
|-----------------------|----------|----------------------------------------|
| Zig                   | 0.13+    | nexlink                                |
| GCC or Clang          | 12+ / 15+| nexroute                              |
| CMake                 | 3.20+    | nexroute                               |
| Rust (rustc + cargo)  | 1.75+    | nexstream, nextrust                    |
| Go                    | 1.22+    | nexapi, nexctl, nexus-cloud            |
| p4c (optional)        | Latest   | nexroute P4 compilation                |
| BMv2 (optional)       | Latest   | nexroute P4 behavioral testing         |
| Docker (optional)     | 24+      | nexus-cloud container builds           |
| etcd (optional)       | 3.5+     | nexus-cloud production state store     |

---

## 7. Key Constants and Wire Format Reference

### NexLink Frame Header (64 bytes)
```
Bytes 0-3:   version(4b) | flags(8b) | frame_type(16b) | padding(4b)
Bytes 4-7:   frame_length(32b)
Bytes 8-11:  stream_id(32b)
Bytes 12-15: sequence_number(32b)
Bytes 16-31: source_node_id(128b)
Bytes 32-47: dest_node_id(128b)
Bytes 48-51: priority(4b) | qos_class(4b) | tensor_dtype(8b) | padding(16b)
Bytes 52-55: tensor_alignment(16b) | options_length(16b)
Bytes 56-63: timestamp(64b)
```

### Frame Types
- 0x0001 Data, 0x0002 Control, 0x0003 Heartbeat, 0x0004 RouteAdvertisement,
  0x0005 TrustHandshake, 0x0006 TensorTransfer, 0x0007 StreamControl

### QoS Classes
- 0x0 BestEffort, 0x1 ReliableOrdered, 0x2 ReliableUnordered, 0x3 Probabilistic

### NexLink UDP Overlay Port: 6477

### SAD Field Type IDs
- 0x01 MODEL_ARCH, 0x02 CAPABILITY, 0x03 CONTEXT_WINDOW, 0x04 MAX_LATENCY_MS,
  0x05 MAX_COST_MILLI, 0x06 TRUST_LEVEL, 0x07 REGION_PREFER, 0x08 REGION_EXCLUDE,
  0x09 PUBLISHER_ID, 0x0A MIN_BENCHMARK, 0x0B CUSTOM

### NexTrust Cipher Suite IDs
- 0x0001 NEXUS_X25519_ED25519_AES256GCM_SHA256
- 0x0002 NEXUS_X25519_ED25519_CHACHA20POLY1305_SHA256

### NexAPI Message Types
- 0x0001 INFERENCE_REQUEST, 0x0002 INFERENCE_RESPONSE, 0x0003 TOKEN_STREAM_START,
  0x0004 TOKEN_STREAM_CHUNK, 0x0005 TOKEN_STREAM_END, 0x0006 TENSOR_TRANSFER,
  0x0007 CONTEXT_SHARE, 0x0008 CONTEXT_ACK, 0x0009 TOOL_INVOKE, 0x000A TOOL_RESULT,
  0x000B AGENT_NEGOTIATE, 0x000C AGENT_DELEGATE, 0x000D AGENT_RESULT,
  0x000E HEALTH_CHECK, 0x000F HEALTH_STATUS, 0x0010 METRICS_REPORT,
  0x0011 CANCEL, 0x0012 ERROR, 0x0100 CUSTOM

---

## 8. File Naming and Code Style Conventions

| Language | Files            | Style                            | Build System     |
|----------|------------------|----------------------------------|------------------|
| Zig      | `snake_case.zig` | Zig standard style               | build.zig        |
| C        | `snake_case.c/h` | C17, Linux kernel-ish style      | CMakeLists.txt   |
| P4       | `snake_case.p4`  | P4_16 spec style                 | p4c              |
| Rust     | `snake_case.rs`  | rustfmt defaults, edition 2021   | Cargo.toml       |
| Go       | `snake_case.go`  | gofmt, Go standard project layout| go.mod           |

---

## 9. Repository Layout

```
nexus/
  CLAUDE.md                 <-- You are here
  Makefile                  Top-level orchestrated build
  .gitignore                Multi-language gitignore
  00_NEXUS_MONOREPO_README.md
  01_NEXLINK_REQUIREMENTS.md
  02_NEXROUTE_REQUIREMENTS.md
  03_NEXSTREAM_REQUIREMENTS.md
  04_NEXTRUST_REQUIREMENTS.md
  05_NEXAPI_REQUIREMENTS.md
  06_NEXCTL_REQUIREMENTS.md
  07_NEXUS_CLOUD_REQUIREMENTS.md
  nexlink/
    build.zig
    src/                    Zig source files
    tests/                  Zig test files
    include/                Generated C FFI header (nexlink.h)
  nexroute/
    CMakeLists.txt
    src/                    C source files
    include/nexroute/       Public C headers
    p4/                     P4_16 dataplane programs
    tests/                  C test files
  nexstream/
    Cargo.toml
    src/                    Rust source files
    tests/                  Rust integration tests
  nextrust/
    Cargo.toml
    src/                    Rust source files
    tests/                  Rust integration tests
  nexapi/
    go.mod
    pkg/                    Go packages (client, server, protocol, nexbuf, sad, transport, observability)
    cmd/                    Go binaries (nexapi-codegen)
    examples/               Example applications
    tests/                  Go test files
  nexctl/
    go.mod
    main.go
    cmd/                    Cobra command definitions
    pkg/                    Support packages (api, firmware, capture, tui, output, config)
  nexus-cloud/
    go.mod
    cmd/                    Service binaries (apiserver, controller, ca, metrics, allinone)
    pkg/                    Go packages (apiserver, controller, configdist, ca, tenant, metrics, store, agent)
    tests/                  Go test files
  docs/                     Protocol specification documents
  schemas/                  NexBuf schema files (.nexbuf)
  scripts/                  Build, test, and CI scripts
```
