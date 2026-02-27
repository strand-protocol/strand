# StrandStream — Layer 3: Hybrid Transport Protocol

## Module: `strandstream/`

## Language: Rust (Edition 2021, MSRV 1.75+)

## Crate Type: Library (`lib`) with `no_std` support + binary for standalone testing

---

## 1. Overview

StrandStream is the transport layer of the Strand Protocol stack. It replaces TCP and UDP with a hybrid transport that provides four delivery modes — Reliable-Ordered, Reliable-Unordered, Best-Effort, and Probabilistic — all multiplexed over a single connection. StrandStream is designed for AI workloads where different data types within the same session have fundamentally different reliability and ordering requirements.

StrandStream operates on top of StrandLink frames (received via StrandLink's ring buffer interface) and provides the stream abstraction consumed by StrandTrust and StrandAPI above it.

---

## 2. Standards & RFCs Being Replaced / Extended

| Standard | Title | Relevance |
|----------|-------|-----------|
| **RFC 9293** | Transmission Control Protocol (TCP) | StrandStream's Reliable-Ordered mode provides TCP-equivalent guarantees. TCP's congestion control, flow control, and reliability mechanisms are reimplemented with AI-workload optimizations |
| **RFC 768** | User Datagram Protocol (UDP) | StrandStream's Best-Effort mode provides UDP-equivalent unreliable delivery |
| **RFC 9000** | QUIC: A UDP-Based Multiplexed and Secure Transport | Primary architectural reference. StrandStream's multiplexing, connection migration, and stream model are heavily inspired by QUIC. Key differences: StrandStream adds Reliable-Unordered and Probabilistic modes, removes HTTP semantics, and operates over StrandLink instead of UDP |
| **RFC 9002** | QUIC Loss Detection and Congestion Control | Reference for StrandStream's congestion control implementation. StrandStream extends NewReno/CUBIC with AI-workload-specific adaptations |
| **RFC 5681** | TCP Congestion Control | Foundational reference for AIMD, slow start, congestion avoidance |
| **RFC 8312** | CUBIC for Fast Long-Distance Networks | Reference congestion control algorithm; StrandStream implements CUBIC as default with BBR as alternative |
| **RFC 9438** | BBR Congestion Control (BBRv3) | Alternative congestion control for high-bandwidth AI training workloads |
| **RFC 4960** | Stream Control Transmission Protocol (SCTP) | Reference for multi-stream multiplexing. SCTP pioneered multi-homing and multi-streaming; StrandStream extends these concepts |
| **RFC 6298** | Computing TCP's Retransmission Timer | Reference for RTO calculation (Jacobson/Karels algorithm) used in StrandStream's reliable modes |
| **RFC 7323** | TCP Extensions for High Performance | Reference for window scaling, timestamps, PAWS — StrandStream implements equivalents |
| **RFC 8684** | TCP Extensions for Multipath Operation (MPTCP) | Reference for multipath transport — StrandStream natively supports multipath via StrandRoute integration |
| **RFC 6347** | Datagram Transport Layer Security Version 1.2 (DTLS) | Reference for securing datagram-based transport — StrandStream delegates encryption to StrandTrust but references DTLS patterns for unreliable-mode security |

---

## 3. Transport Mode Specification

### 3.1 Delivery Modes

| Mode | ID | Guarantees | Use Cases |
|------|----|-----------|-----------|
| **Reliable-Ordered (RO)** | `0x01` | All data delivered, in order, exactly once | Control messages, authentication, transactional state, model weight synchronization |
| **Reliable-Unordered (RU)** | `0x02` | All data delivered, exactly once, order not guaranteed | Distributed training gradient aggregation, batch inference results, map-reduce shuffles |
| **Best-Effort (BE)** | `0x03` | No delivery guarantee, no ordering, no retransmission | Real-time telemetry, sensor fusion streams, monitoring metrics, speculative prefetch |
| **Probabilistic (PR)** | `0x04` | Delivered with configurable probability p (0.0-1.0). Network-layer redundancy coding | Speculative decoding, ensemble inference (send to N models, need K responses), gossip protocol traffic |

### 3.2 Stream Multiplexing

StrandStream multiplexes multiple independent streams over a single StrandLink connection (identified by source+dest Node ID pair). Each stream has:

- **Stream ID**: 32-bit identifier (from StrandLink header `stream_id` field)
- **Delivery Mode**: One of RO/RU/BE/PR, set at stream creation, immutable for stream lifetime
- **Priority**: 0-15 priority level, maps to StrandLink frame priority
- **Flow Control**: Independent per-stream send/receive windows (for RO and RU modes)
- **Congestion Control**: Shared per-connection congestion window with per-stream fairness

Stream ID allocation:
- `0x00000000`: Reserved (connection control)
- `0x00000001 - 0x7FFFFFFF`: Client-initiated streams (odd)
- `0x80000000 - 0xFFFFFFFE`: Server-initiated streams (even)
- `0xFFFFFFFF`: Reserved (broadcast/multicast)

### 3.3 Connection Lifecycle

```
Client                                Server
  |                                      |
  |--- CONN_INIT (version, node_id) ---->|
  |                                      |
  |<-- CONN_ACCEPT (version, node_id) ---|
  |                                      |
  |--- STREAM_OPEN (id, mode, prio) ---->|
  |<-- STREAM_ACK (id) -----------------|
  |                                      |
  |=== DATA / ACK / NACK exchange ======|
  |                                      |
  |--- STREAM_CLOSE (id) -------------->|
  |<-- STREAM_CLOSE_ACK (id) ------------|
  |                                      |
  |--- CONN_CLOSE ---------------------->|
  |<-- CONN_CLOSE_ACK ------------------|
```

### 3.4 Frame Types (StrandStream Control Frames)

These are carried inside StrandLink frames with `frame_type = StreamControl (0x0007)`:

| Type | ID | Payload | Description |
|------|----|---------|-------------|
| `CONN_INIT` | `0x01` | version(u16), node_id(128b), max_streams(u32), max_data(u64) | Connection initiation |
| `CONN_ACCEPT` | `0x02` | version(u16), node_id(128b), max_streams(u32), max_data(u64) | Connection acceptance |
| `CONN_CLOSE` | `0x03` | reason_code(u32), reason_phrase(var) | Connection teardown |
| `STREAM_OPEN` | `0x10` | stream_id(u32), mode(u8), priority(u8), initial_window(u32) | Open a new stream |
| `STREAM_ACK` | `0x11` | stream_id(u32) | Acknowledge stream open |
| `STREAM_CLOSE` | `0x12` | stream_id(u32), reason_code(u32) | Close a stream |
| `STREAM_RESET` | `0x13` | stream_id(u32), error_code(u32) | Abruptly reset a stream |
| `DATA_ACK` | `0x20` | stream_id(u32), ack_number(u32), window_update(u32) | Acknowledge received data |
| `DATA_NACK` | `0x21` | stream_id(u32), ranges[](start_seq, end_seq) | Selective negative acknowledgment |
| `WINDOW_UPDATE` | `0x22` | stream_id(u32), window_increment(u32) | Flow control window update |
| `PING` | `0x30` | ping_id(u64) | Latency probe |
| `PONG` | `0x31` | ping_id(u64) | Latency probe response |
| `CONGESTION` | `0x40` | ecn_ce_count(u64), bytes_in_flight(u64) | Explicit congestion notification |

---

## 4. Architecture & Components

### 4.1 Source Tree Structure

```
strandstream/
├── Cargo.toml
├── src/
│   ├── lib.rs                     # Crate root, public API
│   ├── connection.rs              # Connection state machine
│   ├── stream.rs                  # Individual stream state + operations
│   ├── mux.rs                     # Stream multiplexer / demultiplexer
│   ├── frame.rs                   # StrandStream control frame encoding/decoding
│   ├── transport/
│   │   ├── mod.rs                 # Transport mode trait definition
│   │   ├── reliable_ordered.rs    # RO mode: TCP-like reliability + ordering
│   │   ├── reliable_unordered.rs  # RU mode: reliability without ordering
│   │   ├── best_effort.rs         # BE mode: fire-and-forget
│   │   └── probabilistic.rs       # PR mode: probabilistic delivery with FEC
│   ├── congestion/
│   │   ├── mod.rs                 # Congestion controller trait
│   │   ├── cubic.rs               # CUBIC congestion control (RFC 8312)
│   │   ├── bbr.rs                 # BBRv3 congestion control (RFC 9438)
│   │   └── none.rs                # No congestion control (for testing / controlled networks)
│   ├── flow_control.rs            # Per-stream and per-connection flow control
│   ├── loss_detection.rs          # Loss detection (RFC 9002 adapted for StrandStream)
│   ├── retransmission.rs          # Retransmission engine for RO/RU modes
│   ├── rtt.rs                     # RTT estimation (RFC 6298 Jacobson/Karels + smoothing)
│   ├── fec.rs                     # Forward Error Correction for Probabilistic mode
│   ├── buffer.rs                  # Send/receive buffer management
│   ├── timer.rs                   # Timer wheel for retransmission, keepalive, idle timeout
│   ├── stats.rs                   # Per-connection and per-stream statistics
│   ├── config.rs                  # Configuration structs with defaults
│   └── error.rs                   # Error types
├── src/platform/
│   ├── mod.rs                     # Platform abstraction trait
│   ├── strandlink.rs                 # StrandLink integration (FFI to Zig ring buffer)
│   ├── tokio.rs                   # Tokio async runtime integration for server workloads
│   └── no_std.rs                  # no_std platform for embedded targets
├── tests/
│   ├── connection_test.rs         # Connection lifecycle tests
│   ├── stream_test.rs             # Per-mode stream behavior tests
│   ├── mux_test.rs                # Multiplexing correctness
│   ├── congestion_test.rs         # Congestion control behavior under simulated loss/delay
│   ├── loss_simulation_test.rs    # Reliability verification under random packet loss
│   ├── flow_control_test.rs       # Window management and backpressure tests
│   ├── fec_test.rs                # FEC encode/decode correctness
│   └── integration_test.rs        # Full stack: open connection → streams → exchange data → close
├── benches/
│   ├── throughput_bench.rs        # Goodput measurement across modes
│   ├── latency_bench.rs           # Per-frame processing latency
│   └── mux_bench.rs               # Multiplexer overhead per stream count
├── fuzz/
│   ├── fuzz_frame_decode.rs       # Fuzz StrandStream frame decoder
│   └── fuzz_connection_input.rs   # Fuzz connection state machine with random input
└── examples/
    ├── echo_server.rs             # Simple echo server demonstrating all 4 modes
    └── tensor_stream.rs           # Tensor streaming example (BF16 model weights)
```

### 4.2 Core Trait Definitions

```rust
// lib.rs - Public API

/// Transport mode for a stream
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum TransportMode {
    ReliableOrdered = 0x01,
    ReliableUnordered = 0x02,
    BestEffort = 0x03,
    Probabilistic = 0x04,
}

/// Configuration for a new stream
pub struct StreamConfig {
    pub mode: TransportMode,
    pub priority: u8,                    // 0-15
    pub initial_window: u32,             // Initial flow control window in bytes
    pub probability: Option<f32>,        // For Probabilistic mode: delivery probability 0.0-1.0
    pub fec_ratio: Option<f32>,          // For Probabilistic mode: FEC redundancy ratio
    pub max_retransmissions: Option<u32>, // For reliable modes: max retransmit attempts
}

/// A StrandStream connection
pub struct Connection { /* ... */ }

impl Connection {
    /// Initiate a connection to a remote node
    pub async fn connect(config: ConnectionConfig) -> Result<Self, StrandStreamError>;

    /// Accept an incoming connection
    pub async fn accept(config: ConnectionConfig) -> Result<Self, StrandStreamError>;

    /// Open a new stream on this connection
    pub async fn open_stream(&self, config: StreamConfig) -> Result<Stream, StrandStreamError>;

    /// Accept an incoming stream opened by the remote side
    pub async fn accept_stream(&self) -> Result<Stream, StrandStreamError>;

    /// Close the connection gracefully
    pub async fn close(&self) -> Result<(), StrandStreamError>;

    /// Get connection statistics
    pub fn stats(&self) -> ConnectionStats;
}

/// A single multiplexed stream
pub struct Stream { /* ... */ }

impl Stream {
    /// Send data on this stream (behavior depends on TransportMode)
    pub async fn send(&self, data: &[u8]) -> Result<usize, StrandStreamError>;

    /// Send data with zero-copy from a StrandLink ring buffer slot
    pub async fn send_zerocopy(&self, slot: RingBufferSlot) -> Result<(), StrandStreamError>;

    /// Receive data from this stream
    pub async fn recv(&self, buf: &mut [u8]) -> Result<usize, StrandStreamError>;

    /// Receive into a StrandLink ring buffer slot (zero-copy)
    pub async fn recv_zerocopy(&self) -> Result<RingBufferSlot, StrandStreamError>;

    /// Close this stream
    pub async fn close(&self) -> Result<(), StrandStreamError>;

    /// Get the stream's transport mode
    pub fn mode(&self) -> TransportMode;

    /// Get stream statistics
    pub fn stats(&self) -> StreamStats;
}

/// Congestion control algorithm trait
pub trait CongestionController: Send + Sync {
    fn on_packet_sent(&mut self, bytes: usize, now: Instant);
    fn on_ack(&mut self, bytes_acked: usize, rtt: Duration, now: Instant);
    fn on_loss(&mut self, bytes_lost: usize, now: Instant);
    fn on_ecn_ce(&mut self, now: Instant);
    fn congestion_window(&self) -> usize;
    fn bytes_in_flight(&self) -> usize;
    fn can_send(&self, bytes: usize) -> bool;
    fn pacing_rate(&self) -> Option<u64>;  // bytes per second, None = no pacing
}
```

### 4.3 Connection State Machine

```
                    ┌──────────┐
                    │  CLOSED  │
                    └────┬─────┘
                         │ connect() / accept()
                    ┌────▼─────┐
                ┌───│  INIT    │───┐
    CONN_ACCEPT │   └──────────┘   │ CONN_INIT received
                │                  │
           ┌────▼─────┐    ┌──────▼────┐
           │ESTABLISHED│    │ESTABLISHED │
           └────┬──────┘    └──────┬────┘
                │                  │
                │  close()         │  CONN_CLOSE received
           ┌────▼─────┐    ┌──────▼────┐
           │ CLOSING   │    │ CLOSING    │
           └────┬──────┘    └──────┬────┘
                │ CONN_CLOSE_ACK   │ send CONN_CLOSE_ACK
                │                  │
                └──────┬───────────┘
                  ┌────▼─────┐
                  │  CLOSED  │
                  └──────────┘
```

---

## 5. Functional Requirements

### 5.1 Connection Management

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-C-001 | Implement full connection lifecycle: INIT → ESTABLISHED → CLOSING → CLOSED | P0 |
| NS-C-002 | Support connection timeout: if CONN_ACCEPT not received within configurable timeout (default 5s), fail | P0 |
| NS-C-003 | Idle timeout: close connections with no stream activity for configurable duration (default 60s) | P1 |
| NS-C-004 | Connection migration: support changing the underlying StrandLink path (Node ID pair) without tearing down streams. Reference RFC 9000 §9 (QUIC Connection Migration) | P2 |
| NS-C-005 | Maximum concurrent streams per connection configurable (default 1024) | P0 |

### 5.2 Reliable-Ordered Mode (RFC 9293 / TCP equivalent)

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-RO-001 | Byte-stream abstraction with in-order delivery guarantee | P0 |
| NS-RO-002 | Selective acknowledgment (SACK) with NACK ranges for efficient loss recovery | P0 |
| NS-RO-003 | Retransmission with exponential backoff (RFC 6298 RTO calculation) | P0 |
| NS-RO-004 | Flow control: per-stream receive window, advertised via WINDOW_UPDATE frames | P0 |
| NS-RO-005 | Head-of-line blocking isolation: loss on one RO stream MUST NOT block other streams | P0 |
| NS-RO-006 | Maximum retransmission attempts configurable (default 10). After exhaustion, stream error | P0 |

### 5.3 Reliable-Unordered Mode

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-RU-001 | Message-oriented (not byte-stream): each send() produces one message, each recv() returns one message | P0 |
| NS-RU-002 | All messages delivered exactly once, but arrival order may differ from send order | P0 |
| NS-RU-003 | Selective ACK/NACK for loss detection and retransmission | P0 |
| NS-RU-004 | Flow control based on message count, not byte count | P1 |
| NS-RU-005 | No head-of-line blocking: received messages delivered immediately regardless of gaps | P0 |

### 5.4 Best-Effort Mode

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-BE-001 | Fire-and-forget: send() queues frame for transmission, no acknowledgment or retransmission | P0 |
| NS-BE-002 | No flow control: sender can transmit at will (congestion control still applies at connection level) | P0 |
| NS-BE-003 | No ordering guarantees | P0 |
| NS-BE-004 | Optional sequence numbers for receiver-side reordering hints (not enforced) | P1 |

### 5.5 Probabilistic Mode

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-PR-001 | Configurable delivery probability p: each frame independently delivered with probability p | P0 |
| NS-PR-002 | Forward Error Correction (FEC): Reed-Solomon or XOR-based erasure coding, configurable redundancy ratio | P1 |
| NS-PR-003 | Probabilistic multipath: when StrandRoute provides K paths, distribute frames across paths with configurable weights | P1 |
| NS-PR-004 | No retransmission: if a frame is lost beyond FEC recovery, it stays lost | P0 |
| NS-PR-005 | Receiver reports: periodic summary of received/lost frame counts for sender-side adaptation | P1 |

### 5.6 Congestion Control

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-CC-001 | Implement CUBIC congestion control as default (RFC 8312) | P0 |
| NS-CC-002 | Implement BBRv3 as alternative congestion control (RFC 9438) | P1 |
| NS-CC-003 | Per-connection congestion window shared across all streams with weighted fairness | P0 |
| NS-CC-004 | ECN (Explicit Congestion Notification) support: read ECN bits from StrandLink frames, reduce window on CE marks | P1 |
| NS-CC-005 | Pacing: spread packet transmissions evenly over RTT to avoid bursts | P1 |
| NS-CC-006 | Congestion controller pluggable via `CongestionController` trait | P0 |

### 5.7 Loss Detection

| ID | Requirement | Priority |
|----|-------------|----------|
| NS-LD-001 | Implement packet threshold loss detection: 3 duplicate ACKs trigger fast retransmit (per RFC 9002 §6.1) | P0 |
| NS-LD-002 | Implement time threshold loss detection: packets unacked for > 9/8 × smoothed_rtt declared lost (per RFC 9002 §6.1.2) | P0 |
| NS-LD-003 | Probe timeout (PTO): send probe packet when no ACKs received for PTO duration | P0 |
| NS-LD-004 | RTT estimation using exponentially weighted moving average (RFC 6298 Jacobson/Karels algorithm) | P0 |

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NS-NF-001 | Per-frame processing latency (mux + mode logic) | < 1μs |
| NS-NF-002 | Maximum goodput (RO mode, single stream, 100Gbps NIC) | > 80 Gbps |
| NS-NF-003 | Maximum goodput (RU mode, 100 concurrent streams) | > 70 Gbps aggregate |
| NS-NF-004 | Maximum concurrent streams per connection | 65,536 |
| NS-NF-005 | Memory per stream (idle) | < 4KB |
| NS-NF-006 | Memory per stream (active, default buffer sizes) | < 256KB |
| NS-NF-007 | Zero-copy path: no buffer copies between StrandLink ring buffer and application | For send_zerocopy/recv_zerocopy |
| NS-NF-008 | no_std support (embedded target) | All core logic excluding async runtime |

---

## 7. Build & Compilation

```bash
# Build (default features: std, tokio runtime)
cargo build --release

# Build no_std (for embedded targets)
cargo build --release --no-default-features --features no_std

# Run tests
cargo test

# Run benchmarks
cargo bench

# Run fuzz tests (requires cargo-fuzz)
cargo fuzz run fuzz_frame_decode -- -max_total_time=300

# Generate documentation
cargo doc --open
```

### Cargo Features

| Feature | Default | Description |
|---------|---------|-------------|
| `std` | Yes | Standard library support |
| `tokio` | Yes | Tokio async runtime integration |
| `no_std` | No | Embedded/no_std target (mutually exclusive with std) |
| `bbr` | No | BBRv3 congestion control (adds complexity, optional) |
| `fec` | No | Forward Error Correction for Probabilistic mode |
| `strandlink-ffi` | Yes | StrandLink C FFI bindings for ring buffer integration |

---

## 8. Testing Requirements

| Test Type | Description | Coverage |
|-----------|-------------|----------|
| Unit tests | Every function in every module. State machine transitions. Frame encode/decode | 95%+ |
| Property tests (proptest) | Encode/decode roundtrip. State machine invariants (no invalid transitions) | All public types |
| Simulation tests | Simulated network with configurable loss, delay, reordering. Verify RO delivers all data in order, RU delivers all data, BE doesn't crash, PR meets probability targets | All 4 modes |
| Congestion control tests | CUBIC/BBR behavior under step function bandwidth changes. Fairness between streams. No starvation | All CC algorithms |
| Fuzz tests | Frame decoder, connection state machine input | 10M+ iterations |
| Benchmark tests | Throughput, latency, memory per stream. Regression tracked in CI | Hot path |
| Integration tests | Full StrandLink → StrandStream roundtrip with mock StrandLink backend | All modes |

---

## 9. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `tokio` | 1.35+ | Async runtime (optional, feature-gated) |
| `bytes` | 1.5+ | Zero-copy byte buffer management |
| `crossbeam` | 0.8+ | Lock-free data structures for multiplexer |
| `parking_lot` | 0.12+ | Fast mutexes for non-hot-path synchronization |
| `tracing` | 0.1+ | Structured logging / observability |
| `proptest` | 1.4+ | Property-based testing (dev dependency) |
| `criterion` | 0.5+ | Benchmarking (dev dependency) |
| `cargo-fuzz` | — | Fuzz testing (dev dependency) |
| StrandLink FFI | — | C FFI bindings to StrandLink ring buffer (via `bindgen`) |
