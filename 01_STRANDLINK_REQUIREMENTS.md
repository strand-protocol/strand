# StrandLink — Layer 1: AI-Native Frame Protocol

## Module: `strandlink/`

## Language: Zig (0.13+)

## Build Target: Freestanding (no OS, no libc) + Linux userspace (for testing/development)

---

## 1. Overview

StrandLink is the lowest layer of the Strand Protocol stack. It defines a new frame format optimized for AI workloads — replacing the IEEE 802.3 Ethernet frame with a format that supports zero-copy DMA, tensor-aware memory alignment, AI metadata headers, and direct NIC-to-GPU data paths.

StrandLink runs as a firmware shim on programmable NICs (SmartNICs) or as a userspace driver via DPDK/XDP on standard NICs. The frame format is designed to be encapsulated inside standard Ethernet for backward compatibility (Tier 3 overlay mode).

---

## 2. Standards & RFCs Being Replaced / Extended

StrandLink replaces or extends the following standards. Claude Code **must** reference these specifications when implementing corresponding functionality:

| Standard | Title | Relevance |
|----------|-------|-----------|
| **IEEE 802.3** | Ethernet Frame Format | StrandLink replaces the Ethernet payload format while remaining encapsulatable inside 802.3 frames for overlay mode |
| **RFC 894** | A Standard for the Transmission of IP Datagrams over Ethernet Networks | Defines how IP is carried over Ethernet; StrandLink defines how Strand datagrams are carried |
| **RFC 7348** (VXLAN) | Virtual Extensible Local Area Network | Reference for overlay encapsulation design — StrandLink Tier 3 uses a similar UDP encapsulation model |
| **RFC 8926** (Geneve) | Generic Network Virtualization Encapsulation | Alternative overlay reference; StrandLink's overlay header design borrows Geneve's extensible TLV option model |
| **RFC 3031** (MPLS) | Multiprotocol Label Switching Architecture | Reference for label-based forwarding — StrandLink's stream ID operates similarly to MPLS labels |
| **IEEE 802.1Q** | VLAN Tagging | StrandLink supports QoS priority bits analogous to 802.1p/PCP fields |
| **RFC 2464** | Transmission of IPv6 Packets over Ethernet Networks | Reference for MTU handling and fragmentation at the link layer |

### Hardware SDK References (for firmware targets)

| SDK | Vendor | Purpose |
|-----|--------|---------|
| **NVIDIA DOCA SDK** | NVIDIA (Mellanox) | BlueField DPU / ConnectX SmartNIC firmware development |
| **Intel Infrastructure Processing Unit SDK** | Intel | E810 / IPU firmware development |
| **DPDK (libdpdk)** | Linux Foundation | Userspace NIC driver for kernel bypass on standard NICs |
| **XDP/eBPF** | Linux Kernel | In-kernel fast-path packet processing |
| **io_uring** | Linux Kernel | Async I/O ring buffer interface — reference for StrandLink's ring buffer design |

---

## 3. Frame Format Specification

### 3.1 StrandLink Frame Header (64 bytes fixed)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Version (4)  | Flags (8)     |        Frame Type (16)        |  Byte 0-3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Frame Length (32)                        |  Byte 4-7
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Stream ID (32)                          |  Byte 8-11
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     Sequence Number (32)                      |  Byte 12-15
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                    Source Node ID (128)                        |  Byte 16-31
|                                                               |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                 Destination Node ID (128)                      |  Byte 32-47
|                                                               |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Priority (4) | QoS Class (4) |   Tensor Dtype (8)          |  Byte 48-51
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Tensor Alignment (16) |     Options Length (16)     |  Byte 52-55
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Timestamp (64)                           |  Byte 56-63
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                   Options (variable, 0-256 bytes)             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Payload (variable)                      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Frame CRC (32)                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.2 Field Definitions

| Field | Size | Description |
|-------|------|-------------|
| `version` | 4 bits | Protocol version. Current = `0x1` |
| `flags` | 8 bits | Bitfield: `[0]` = more_fragments, `[1]` = compressed, `[2]` = encrypted, `[3]` = tensor_payload, `[4]` = priority_express, `[5]` = overlay_encap, `[6-7]` = reserved |
| `frame_type` | 16 bits | `0x0001` = Data, `0x0002` = Control, `0x0003` = Heartbeat, `0x0004` = RouteAdvertisement, `0x0005` = TrustHandshake, `0x0006` = TensorTransfer, `0x0007` = StreamControl |
| `frame_length` | 32 bits | Total frame length in bytes including header, options, payload, and CRC |
| `stream_id` | 32 bits | Multiplexed stream identifier (analogous to MPLS label / HTTP/2 stream ID) |
| `sequence_number` | 32 bits | Per-stream monotonic sequence number |
| `source_node_id` | 128 bits | Strand node identifier (NOT an IP address — derived from StrandTrust identity key) |
| `dest_node_id` | 128 bits | Destination node identifier or multicast group ID |
| `priority` | 4 bits | 0-15 priority level (0 = lowest, 15 = express). Maps to hardware QoS queues |
| `qos_class` | 4 bits | `0x0` = BestEffort, `0x1` = ReliableOrdered, `0x2` = ReliableUnordered, `0x3` = Probabilistic, `0x4-0xF` = Reserved |
| `tensor_dtype` | 8 bits | Tensor data type when `tensor_payload` flag set: `0x01` = float16, `0x02` = bfloat16, `0x03` = float32, `0x04` = float64, `0x05` = int8, `0x06` = int4, `0x07` = uint8, `0x08` = fp8_e4m3, `0x09` = fp8_e5m2 |
| `tensor_alignment` | 16 bits | Required memory alignment in bytes for tensor payload (must be power of 2, typically 64, 128, or 512) |
| `options_length` | 16 bits | Length of options section in bytes (0 if no options) |
| `timestamp` | 64 bits | Nanosecond-precision timestamp (Unix epoch). Used for latency measurement, ordering, and replay protection |
| `options` | Variable | TLV-encoded options (see §3.3). Max 256 bytes |
| `payload` | Variable | Frame payload data |
| `crc` | 32 bits | CRC-32C (Castagnoli) over entire frame excluding CRC field itself |

### 3.3 Options TLV Format

Each option follows a Type-Length-Value encoding:

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type (8)    |   Length (8)   |  Value (var)  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

Defined option types:

| Type | Name | Description |
|------|------|-------------|
| `0x01` | `FRAGMENT_INFO` | Fragment offset (32b) + total fragments (16b) for jumbo frame fragmentation |
| `0x02` | `COMPRESSION_ALG` | Compression algorithm ID: `0x01` = lz4, `0x02` = zstd, `0x03` = snappy |
| `0x03` | `ENCRYPTION_TAG` | 16-byte AEAD authentication tag when `encrypted` flag set |
| `0x04` | `TENSOR_SHAPE` | Tensor dimension descriptor: ndims (8b) + dims[] (32b each) |
| `0x05` | `TRACE_ID` | 16-byte distributed trace ID for observability |
| `0x06` | `HOP_COUNT` | Current hop count (8b) for loop prevention |
| `0x07` | `SEMANTIC_ADDR` | Compressed semantic address descriptor (for StrandRoute integration) |
| `0x08` | `GPU_HINT` | Target GPU device ID + memory pool hint for GPUDirect RDMA |

---

## 4. Architecture & Components

### 4.1 Source Tree Structure

```
strandlink/
├── build.zig                    # Top-level Zig build script
├── src/
│   ├── main.zig                 # Entry point for userspace test harness
│   ├── frame.zig                # Frame format encoding/decoding
│   ├── header.zig               # Header struct definitions and serialization
│   ├── options.zig              # TLV option parsing and generation
│   ├── crc.zig                  # CRC-32C implementation (hardware-accelerated where available)
│   ├── ring_buffer.zig          # DMA-capable ring buffer (io_uring inspired)
│   ├── memory_pool.zig          # Pre-allocated memory pool with alignment guarantees
│   ├── tx.zig                   # Transmit path: frame assembly → ring buffer → DMA
│   ├── rx.zig                   # Receive path: DMA → ring buffer → frame parsing → dispatch
│   ├── stats.zig                # Per-port and per-stream statistics counters
│   ├── overlay.zig              # Tier 3 overlay: StrandLink encapsulated in UDP/IP
│   └── platform/
│       ├── dpdk.zig             # DPDK backend (Linux userspace, kernel bypass)
│       ├── xdp.zig              # XDP/eBPF backend (Linux in-kernel fast path)
│       ├── connectx.zig         # NVIDIA ConnectX firmware shim (freestanding)
│       ├── e810.zig             # Intel E810 firmware shim (freestanding)
│       ├── bluefield.zig        # NVIDIA BlueField DPU firmware shim (freestanding)
│       └── mock.zig             # Mock backend for unit testing
├── tests/
│   ├── frame_test.zig           # Frame encode/decode roundtrip tests
│   ├── ring_buffer_test.zig     # Ring buffer concurrency tests
│   ├── overlay_test.zig         # Overlay encap/decap tests
│   ├── fuzz_test.zig            # Fuzz testing for frame parser
│   └── benchmark_test.zig       # Latency and throughput microbenchmarks
├── include/
│   └── strandlink.h                # C-compatible header for FFI (auto-generated from Zig exports)
└── firmware/
    ├── connectx/                # ConnectX-specific firmware build artifacts
    ├── e810/                    # E810-specific firmware build artifacts
    └── README.md                # Firmware flashing instructions per NIC model
```

### 4.2 Core Data Structures (Zig)

```zig
// frame.zig

pub const STRANDLINK_VERSION: u4 = 1;
pub const HEADER_SIZE: usize = 64;
pub const MAX_OPTIONS_SIZE: usize = 256;
pub const MAX_FRAME_SIZE: usize = 65535; // 64KB max (jumbo frames via fragmentation)
pub const MIN_FRAME_SIZE: usize = HEADER_SIZE + 4; // header + CRC, no payload

pub const FrameFlags = packed struct(u8) {
    more_fragments: bool,
    compressed: bool,
    encrypted: bool,
    tensor_payload: bool,
    priority_express: bool,
    overlay_encap: bool,
    _reserved: u2 = 0,
};

pub const FrameType = enum(u16) {
    data = 0x0001,
    control = 0x0002,
    heartbeat = 0x0003,
    route_advertisement = 0x0004,
    trust_handshake = 0x0005,
    tensor_transfer = 0x0006,
    stream_control = 0x0007,
    _,
};

pub const QosClass = enum(u4) {
    best_effort = 0x0,
    reliable_ordered = 0x1,
    reliable_unordered = 0x2,
    probabilistic = 0x3,
    _,
};

pub const TensorDtype = enum(u8) {
    float16 = 0x01,
    bfloat16 = 0x02,
    float32 = 0x03,
    float64 = 0x04,
    int8 = 0x05,
    int4 = 0x06,
    uint8 = 0x07,
    fp8_e4m3 = 0x08,
    fp8_e5m2 = 0x09,
    none = 0x00,
    _,
};

pub const NodeId = [16]u8; // 128-bit node identifier

pub const FrameHeader = packed struct {
    version: u4,
    flags: FrameFlags,
    frame_type: FrameType,
    frame_length: u32,
    stream_id: u32,
    sequence_number: u32,
    source_node_id: NodeId,
    dest_node_id: NodeId,
    priority: u4,
    qos_class: QosClass,
    tensor_dtype: TensorDtype,
    tensor_alignment: u16,
    options_length: u16,
    timestamp: u64,
};
```

### 4.3 Ring Buffer Interface

```zig
// ring_buffer.zig
// Modeled after io_uring's submission/completion queue design (see Linux kernel io_uring.c)

pub const RingBuffer = struct {
    /// Memory-mapped region accessible by both CPU and NIC DMA engine
    base_addr: [*]align(4096) u8,
    /// Total size of the ring buffer in bytes (must be power of 2)
    size: usize,
    /// Number of slots in the ring (size / slot_size)
    num_slots: u32,
    /// Size of each slot in bytes (must accommodate MAX_FRAME_SIZE + metadata)
    slot_size: u32,
    /// Producer index (written by producer, read by consumer) — cache-line aligned
    head: *align(64) volatile u32,
    /// Consumer index (written by consumer, read by producer) — cache-line aligned
    tail: *align(64) volatile u32,

    pub fn init(num_slots: u32, slot_size: u32) !RingBuffer { ... }
    pub fn deinit(self: *RingBuffer) void { ... }

    /// Reserve a slot for writing. Returns pointer to slot or null if full.
    /// Zero-copy: caller writes directly into DMA-capable memory.
    pub fn reserve(self: *RingBuffer) ?*align(64) [*]u8 { ... }

    /// Commit a previously reserved slot, making it visible to consumer.
    pub fn commit(self: *RingBuffer) void { ... }

    /// Peek at next available slot for reading. Returns null if empty.
    pub fn peek(self: *RingBuffer) ?*const [*]u8 { ... }

    /// Release a consumed slot back to the ring.
    pub fn release(self: *RingBuffer) void { ... }

    /// Get DMA-capable physical address for a slot (for NIC descriptor rings)
    pub fn physAddr(self: *RingBuffer, slot_index: u32) u64 { ... }
};
```

---

## 5. Functional Requirements

### 5.1 Frame Operations

| ID | Requirement | Priority |
|----|-------------|----------|
| NL-F-001 | Encode a `FrameHeader` + options + payload into a contiguous byte buffer with correct CRC-32C | P0 |
| NL-F-002 | Decode a byte buffer into a `FrameHeader` + parsed options + payload slice with CRC validation | P0 |
| NL-F-003 | Support zero-copy encoding: write header directly into a pre-allocated DMA ring buffer slot | P0 |
| NL-F-004 | Validate all header fields on decode: version, frame_type, length consistency, CRC | P0 |
| NL-F-005 | Parse and generate TLV options with type safety (reject unknown critical options, skip unknown non-critical) | P0 |
| NL-F-006 | Fragment frames exceeding MTU into multiple frames with `FRAGMENT_INFO` option and `more_fragments` flag | P1 |
| NL-F-007 | Reassemble fragmented frames on the receive path with timeout-based cleanup of incomplete fragments | P1 |
| NL-F-008 | Support tensor-aware payload alignment: when `tensor_payload` flag is set, payload start address must be aligned to `tensor_alignment` bytes | P0 |
| NL-F-009 | Populate `timestamp` field with nanosecond precision using hardware timestamping when available, software fallback otherwise | P1 |

### 5.2 Ring Buffer / DMA

| ID | Requirement | Priority |
|----|-------------|----------|
| NL-R-001 | Implement lock-free single-producer single-consumer ring buffer using atomic operations | P0 |
| NL-R-002 | All ring buffer memory must be allocated from hugepage-backed, DMA-capable regions (2MB or 1GB hugepages) | P0 |
| NL-R-003 | Ring buffer slots must be cache-line aligned (64 bytes) to prevent false sharing | P0 |
| NL-R-004 | Support configurable slot sizes: 2KB (small frames), 9KB (jumbo), 64KB (tensor bulk) | P0 |
| NL-R-005 | Provide memory pool with pre-allocated buffers to avoid runtime allocation | P0 |
| NL-R-006 | Expose physical address translation for NIC DMA descriptor programming | P1 |
| NL-R-007 | Implement batch operations: reserve/commit N slots atomically for burst transmit/receive | P1 |

### 5.3 Platform Backends

| ID | Requirement | Priority |
|----|-------------|----------|
| NL-P-001 | **DPDK backend**: Initialize DPDK EAL, bind NIC ports, configure RX/TX queues, implement poll-mode receive/transmit loops | P0 |
| NL-P-002 | **XDP backend**: Load eBPF/XDP program for fast-path frame classification, redirect StrandLink frames to userspace via AF_XDP socket | P1 |
| NL-P-003 | **Overlay backend**: Encapsulate StrandLink frames in UDP/IP (destination port 6477). Implement encap/decap with configurable outer source/dest IP. Reference RFC 7348 (VXLAN) and RFC 8926 (Geneve) encapsulation patterns | P0 |
| NL-P-004 | **Mock backend**: In-memory loopback for unit testing. Simulates ring buffer semantics without hardware | P0 |
| NL-P-005 | **ConnectX firmware shim**: Zig freestanding target. Interface with ConnectX NIC via NVIDIA DOCA SDK C API using `@cImport`. Implement custom RX/TX processing in the NIC pipeline | P2 |
| NL-P-006 | All backends must implement a common `Platform` interface trait for backend-agnostic upper layers | P0 |

### 5.4 Overlay Mode (Tier 3 Compatibility)

| ID | Requirement | Priority |
|----|-------------|----------|
| NL-O-001 | Encapsulate StrandLink frames inside UDP datagrams. Outer header: standard Ethernet + IPv4/IPv6 + UDP (port 6477) + StrandLink Overlay Header (8 bytes) + StrandLink Frame | P0 |
| NL-O-002 | Overlay header format: `[Version:4][Flags:4][Reserved:8][VNI:24][Reserved:24]` (modeled after Geneve, RFC 8926) | P0 |
| NL-O-003 | Support both IPv4 and IPv6 outer headers | P0 |
| NL-O-004 | Handle MTU calculation: outer Ethernet (14) + outer IP (20/40) + UDP (8) + overlay header (8) + StrandLink frame. Default inner MTU = 1422 bytes (for 1500 byte outer MTU with IPv4) | P0 |
| NL-O-005 | Implement UDP checksum offload hints for hardware that supports it | P1 |

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NL-NF-001 | Frame encode latency | < 200ns for standard frames (no options) |
| NL-NF-002 | Frame decode + validate latency | < 300ns including CRC verification |
| NL-NF-003 | Ring buffer reserve+commit | < 50ns per operation |
| NL-NF-004 | Zero heap allocations in hot path | 0 allocations after initialization |
| NL-NF-005 | Throughput (DPDK backend) | > 10 Mpps for minimum-size frames on 25GbE NIC |
| NL-NF-006 | Memory footprint | < 256MB for ring buffers + memory pool at default configuration |
| NL-NF-007 | Freestanding binary size (firmware) | < 512KB for ConnectX/E810 firmware shim |

---

## 7. Build & Compilation

```bash
# Development build (Linux userspace, links libc for testing)
zig build -Dbackend=dpdk

# Freestanding firmware build (no OS, no libc)
zig build -Dbackend=connectx -Dtarget=aarch64-freestanding-none

# Run tests
zig build test

# Run benchmarks
zig build bench

# Generate C header for FFI
zig build -Demit-h
```

### Build Configuration (`build.zig` options)

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `-Dbackend` | `dpdk`, `xdp`, `overlay`, `connectx`, `e810`, `bluefield`, `mock` | `mock` | Platform backend selection |
| `-Dtarget` | Any Zig cross-compile target | Native | Cross-compilation target |
| `-Doptimize` | `Debug`, `ReleaseSafe`, `ReleaseFast`, `ReleaseSmall` | `ReleaseSafe` | Optimization level |
| `-Dhugepage-size` | `2MB`, `1GB` | `2MB` | Hugepage size for ring buffers |
| `-Dmax-ring-slots` | Integer | `4096` | Default ring buffer slot count |
| `-Demit-h` | Boolean | `false` | Generate C-compatible header file |

---

## 8. Testing Requirements

| Test Type | Description | Coverage Target |
|-----------|-------------|-----------------|
| Unit tests | Every function in frame.zig, header.zig, options.zig, crc.zig, ring_buffer.zig | 95%+ line coverage |
| Roundtrip tests | Encode → decode for every frame type and option type, verify bit-exact reconstruction | All frame types × all option combinations |
| Fuzz tests | AFL-style fuzzing of frame decoder with random byte inputs. Must not crash, panic, or read out of bounds | 10M+ iterations |
| Property tests | Encode(Decode(x)) == x for all valid frames. Decode(random_bytes) returns error, never UB | Statistical coverage |
| Benchmark tests | Latency and throughput for encode/decode/ring_buffer operations. Results logged as JSON for regression tracking | All hot-path functions |
| Integration tests | DPDK backend: send/receive StrandLink frames between two DPDK-bound NICs (or virtual functions) | Basic TX/RX path |
| Overlay tests | Encap/decap roundtrip through standard UDP socket. Verify outer header correctness per RFC 7348/8926 patterns | Full encap/decap path |

---

## 9. Dependencies

| Dependency | Version | Purpose | Required For |
|------------|---------|---------|--------------|
| Zig compiler | 0.13+ | Build system and compiler | All |
| DPDK (libdpdk) | 23.11+ | Userspace NIC driver | DPDK backend only |
| libbpf | 1.3+ | XDP/eBPF loader | XDP backend only |
| NVIDIA DOCA SDK | 2.7+ | ConnectX/BlueField firmware | Firmware backends only |
| None (freestanding) | — | Firmware targets have zero dependencies | Firmware builds |

---

## 10. C FFI Exports

StrandLink must export a C-compatible API for integration with C-based switch OS code (StrandRoute) and other language bindings:

```c
// strandlink.h (auto-generated)

typedef struct strandlink_frame_header { /* ... packed struct matching Zig layout ... */ } strandlink_frame_header_t;
typedef struct strandlink_ring_buffer { /* ... opaque handle ... */ } strandlink_ring_buffer_t;

// Frame operations
int strandlink_frame_encode(const strandlink_frame_header_t *header, const uint8_t *options,
                         uint16_t options_len, const uint8_t *payload, uint32_t payload_len,
                         uint8_t *out_buf, uint32_t out_buf_len, uint32_t *out_frame_len);

int strandlink_frame_decode(const uint8_t *buf, uint32_t buf_len,
                         strandlink_frame_header_t *out_header, const uint8_t **out_payload,
                         uint32_t *out_payload_len);

// Ring buffer operations
strandlink_ring_buffer_t *strandlink_ring_alloc(uint32_t num_slots, uint32_t slot_size);
void strandlink_ring_free(strandlink_ring_buffer_t *ring);
uint8_t *strandlink_ring_reserve(strandlink_ring_buffer_t *ring);
void strandlink_ring_commit(strandlink_ring_buffer_t *ring);
const uint8_t *strandlink_ring_peek(strandlink_ring_buffer_t *ring);
void strandlink_ring_release(strandlink_ring_buffer_t *ring);
```

---

## 11. Open Questions / Future Work

- **Hardware timestamping**: PTP (IEEE 1588) integration for sub-microsecond timestamp accuracy across nodes
- **GPUDirect RDMA**: Direct NIC-to-GPU memory path integration with NVIDIA GPUDirect. Requires CUDA interop from Zig (via C FFI to `nvidia-peermem` kernel module)
- **Compression**: Whether to implement LZ4/ZSTD in StrandLink or defer to StrandStream. Current design allows both.
- **Jumbo frame negotiation**: Path MTU discovery equivalent for StrandLink. May use heartbeat frames to probe MTU.
