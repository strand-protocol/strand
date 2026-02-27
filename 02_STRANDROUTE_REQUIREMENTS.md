# StrandRoute — Layer 2: Semantic Routing & Intent-Based Addressing

## Module: `strandroute/`

## Language: C (C17) + P4 (P4_16)

## Build Target: Switch ASIC dataplane (P4), SONiC userspace daemon, Linux dataplane

---

## 1. Overview

StrandRoute replaces traditional IP routing and BGP with a semantic, intent-based routing layer designed for AI workloads. Instead of routing packets to IP addresses (which identify network interfaces), StrandRoute routes frames to **capabilities** — expressed as Semantic Address Descriptors (SADs). An AI agent can request "an LLM with code generation capability, 128K context, latency < 200ms" and StrandRoute resolves and routes to the optimal endpoint in real-time.

StrandRoute operates as a dataplane forwarding engine (in P4 for programmable switches, or in C for software dataplanes) paired with a control plane that maintains distributed capability routing tables via a gossip-based protocol.

---

## 2. Standards & RFCs Being Replaced / Extended

| Standard | Title | Relevance |
|----------|-------|-----------|
| **RFC 791** | Internet Protocol (IPv4) | StrandRoute replaces IPv4 addressing and forwarding with semantic addressing |
| **RFC 8200** | Internet Protocol, Version 6 (IPv6) | StrandRoute's 128-bit Node IDs are analogous to IPv6 addresses but semantically derived |
| **RFC 4271** | A Border Gateway Protocol 4 (BGP-4) | StrandRoute's capability advertisement protocol replaces BGP's path-vector routing with capability-vector routing |
| **RFC 4601** | Protocol Independent Multicast - Sparse Mode (PIM-SM) | Reference for multicast group management — StrandRoute supports capability-based multicast groups |
| **RFC 1035** | Domain Names - Implementation and Specification (DNS) | StrandRoute eliminates DNS for AI agent communication; semantic addressing replaces hostname resolution |
| **RFC 6830** | The Locator/ID Separation Protocol (LISP) | Architectural reference — LISP separates identity from location; StrandRoute extends this to separate capability from location |
| **RFC 7047** | The Open vSwitch Database Management Protocol (OVSDB) | Reference for control plane database management in software switches |
| **RFC 7938** | Use of BGP for Routing in Large-Scale Data Centers (BGP in DC) | Reference for datacenter routing patterns that StrandRoute must coexist with |
| **RFC 4655** | A Path Computation Element (PCE) Architecture | Reference for centralized path computation — StrandRoute's semantic resolver acts as a capability-aware PCE |
| **RFC 8040** | RESTCONF Protocol | Reference for control plane configuration APIs |

### Additional References

| Reference | Purpose |
|-----------|---------|
| **P4_16 Language Specification v1.2.4** | Switch dataplane programming language |
| **Portable Switch Architecture (PSA) v1.1** | P4 target abstraction for multi-vendor switch compatibility |
| **SONiC Architecture Guide** | Open Network Operating System integration target |
| **SAI (Switch Abstraction Interface) v1.13** | Vendor-neutral switch ASIC API used by SONiC |
| **HyParView Protocol** (Leitão et al., 2007) | Gossip protocol reference for capability advertisement |
| **FlatBuffers Encoding Spec** | Zero-copy serialization format reference for SAD binary encoding |

---

## 3. Semantic Address Descriptor (SAD) Specification

### 3.1 SAD Binary Format

SADs are variable-length binary structures that describe a set of capabilities or constraints for routing. They are encoded in a FlatBuffers-inspired zero-copy format for parsing at ASIC line-rate.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  SAD Version  |  Num Fields   |         Total Length          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Field Type  | Field Length  |        Field Value ...        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     ... (repeated)                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.2 SAD Field Types

| Type ID | Name | Value Format | Description |
|---------|------|-------------|-------------|
| `0x01` | `MODEL_ARCH` | uint32 (enum) | Model architecture: `0x01`=transformer, `0x02`=diffusion, `0x03`=moe, `0x04`=cnn, `0x05`=rnn, `0x06`=rl_agent |
| `0x02` | `CAPABILITY` | uint32 (bitfield) | Capability flags: bit 0=text_gen, 1=code_gen, 2=image_gen, 3=audio_gen, 4=embedding, 5=classification, 6=tool_use, 7=reasoning |
| `0x03` | `CONTEXT_WINDOW` | uint32 | Minimum context window size in tokens |
| `0x04` | `MAX_LATENCY_MS` | uint32 | Maximum acceptable end-to-end latency in milliseconds |
| `0x05` | `MAX_COST_MILLI` | uint32 | Maximum cost per request in millionths of a dollar |
| `0x06` | `TRUST_LEVEL` | uint8 | Minimum StrandTrust attestation level: 0=none, 1=identity, 2=provenance, 3=safety_eval, 4=full_audit |
| `0x07` | `REGION_PREFER` | uint16[] | Preferred geographic region codes (ISO 3166-1 numeric) |
| `0x08` | `REGION_EXCLUDE` | uint16[] | Excluded geographic region codes (for data sovereignty) |
| `0x09` | `PUBLISHER_ID` | uint128 | Required model publisher Node ID (for routing to specific providers) |
| `0x0A` | `MIN_BENCHMARK` | uint32 | Minimum benchmark score (field-specific, e.g., HumanEval pass@1 × 1000) |
| `0x0B` | `CUSTOM` | bytes | Vendor/application-defined extension field |

### 3.3 SAD Examples

**Example 1: "Find me a code-gen LLM with 128K context, under 200ms"**
```
SAD{
  MODEL_ARCH: transformer,
  CAPABILITY: text_gen | code_gen | tool_use,
  CONTEXT_WINDOW: 131072,
  MAX_LATENCY_MS: 200,
}
```

**Example 2: "Any embedding model in EU, with full provenance attestation"**
```
SAD{
  CAPABILITY: embedding,
  TRUST_LEVEL: 2,  // provenance
  REGION_PREFER: [276, 250, 528],  // DE, FR, NL
}
```

---

## 4. Architecture & Components

### 4.1 Source Tree Structure

```
strandroute/
├── Makefile                       # Top-level build
├── CMakeLists.txt                 # CMake build for C components
├── include/
│   ├── strandroute/
│   │   ├── sad.h                  # SAD encoding/decoding API
│   │   ├── routing_table.h        # Capability routing table interface
│   │   ├── resolver.h             # SAD → Node ID resolution engine
│   │   ├── gossip.h               # Capability advertisement gossip protocol
│   │   ├── forwarding.h           # Dataplane forwarding engine (software)
│   │   ├── multipath.h            # Weighted ECMP / probabilistic multipath
│   │   ├── topology.h             # Network topology graph
│   │   ├── metrics.h              # Routing metrics (latency, load, cost)
│   │   └── api.h                  # Control plane gRPC/REST API definitions
│   └── strandlink.h                  # Imported from StrandLink (C FFI header)
├── src/
│   ├── sad.c                      # SAD binary encoding/decoding
│   ├── sad_match.c                # SAD matching engine (does a capability set satisfy a SAD query?)
│   ├── routing_table.c            # Lock-free concurrent routing table (RCU-based)
│   ├── resolver.c                 # Multi-constraint SAD resolution with scoring
│   ├── gossip.c                   # HyParView-based gossip protocol for capability distribution
│   ├── forwarding.c               # Software dataplane: StrandLink frame → lookup → forward
│   ├── multipath.c                # Weighted multipath with consistent hashing
│   ├── topology.c                 # Topology discovery and graph maintenance
│   ├── metrics_collector.c        # Real-time latency/load/cost metric collection
│   ├── control_plane.c            # Control plane daemon main loop
│   └── sonic_integration.c        # SONiC SAI plugin for hardware forwarding offload
├── p4/
│   ├── strandroute.p4                # P4_16 dataplane program for programmable switches
│   ├── headers.p4                 # StrandLink + StrandRoute header definitions in P4
│   ├── parser.p4                  # P4 parser for StrandLink frames
│   ├── sad_lookup.p4              # P4 SAD field matching via TCAM tables
│   ├── forwarding.p4              # P4 forwarding pipeline
│   └── psa_portable.p4            # PSA-compatible wrapper for multi-vendor targets
├── tests/
│   ├── test_sad.c                 # SAD encode/decode tests
│   ├── test_sad_match.c           # SAD matching correctness tests
│   ├── test_routing_table.c       # Concurrent routing table tests
│   ├── test_resolver.c            # Resolution scoring tests
│   ├── test_gossip.c              # Gossip protocol convergence tests
│   ├── test_multipath.c           # Multipath distribution uniformity tests
│   └── test_p4_behavioral.py      # P4 behavioral model tests (BMv2)
├── config/
│   ├── strandroute.yaml              # Default configuration
│   └── sonic/
│       ├── strandroute.json          # SONiC ConfigDB schema for StrandRoute
│       └── strandroute.service       # systemd service unit for SONiC
└── tools/
    ├── strandroute-cli.c             # CLI tool for inspecting routing tables
    └── sad-debug.c                # Tool for encoding/decoding SAD descriptors for debugging
```

### 4.2 Core Data Structures

```c
// sad.h

#define SAD_VERSION 1
#define SAD_MAX_FIELDS 16
#define SAD_MAX_SIZE 512  // bytes

typedef enum {
    SAD_FIELD_MODEL_ARCH     = 0x01,
    SAD_FIELD_CAPABILITY     = 0x02,
    SAD_FIELD_CONTEXT_WINDOW = 0x03,
    SAD_FIELD_MAX_LATENCY_MS = 0x04,
    SAD_FIELD_MAX_COST_MILLI = 0x05,
    SAD_FIELD_TRUST_LEVEL    = 0x06,
    SAD_FIELD_REGION_PREFER  = 0x07,
    SAD_FIELD_REGION_EXCLUDE = 0x08,
    SAD_FIELD_PUBLISHER_ID   = 0x09,
    SAD_FIELD_MIN_BENCHMARK  = 0x0A,
    SAD_FIELD_CUSTOM         = 0x0B,
} sad_field_type_t;

typedef struct {
    sad_field_type_t type;
    uint8_t length;
    uint8_t value[64];
} sad_field_t;

typedef struct {
    uint8_t version;
    uint8_t num_fields;
    uint16_t total_length;
    sad_field_t fields[SAD_MAX_FIELDS];
} sad_t;

// routing_table.h

typedef struct {
    uint8_t node_id[16];           // StrandLink Node ID
    sad_t capabilities;             // What this node offers
    uint32_t latency_us;           // Current measured latency in microseconds
    float load_factor;             // 0.0 - 1.0 current load
    uint32_t cost_milli;           // Cost per request in millionths of dollar
    uint8_t trust_level;           // StrandTrust attestation level
    uint16_t region_code;          // ISO 3166-1 numeric
    uint64_t last_updated;         // Timestamp of last gossip update
    uint64_t ttl_ns;               // Time-to-live for this entry
} route_entry_t;

typedef struct routing_table routing_table_t;  // Opaque, RCU-protected

routing_table_t *routing_table_create(uint32_t initial_capacity);
void routing_table_destroy(routing_table_t *rt);

int routing_table_insert(routing_table_t *rt, const route_entry_t *entry);
int routing_table_remove(routing_table_t *rt, const uint8_t node_id[16]);
int routing_table_update_metrics(routing_table_t *rt, const uint8_t node_id[16],
                                  uint32_t latency_us, float load_factor);

// Resolve: find best matching node(s) for a SAD query
// Returns number of results written to out_results (up to max_results)
int routing_table_resolve(const routing_table_t *rt, const sad_t *query,
                          route_entry_t *out_results, int max_results);
```

### 4.3 Resolution Scoring Algorithm

The resolver scores each candidate route entry against a SAD query using a weighted multi-constraint scoring function:

```
Score(candidate, query) = Σ_i  w_i × match_score_i(candidate, query.field[i])

Where:
  - match_score for CAPABILITY:    popcount(candidate.caps & query.caps) / popcount(query.caps)
  - match_score for LATENCY:       max(0, 1.0 - (candidate.latency / query.max_latency))
  - match_score for COST:          max(0, 1.0 - (candidate.cost / query.max_cost))
  - match_score for CONTEXT_WINDOW: candidate.ctx >= query.ctx ? 1.0 : 0.0  (hard constraint)
  - match_score for TRUST_LEVEL:    candidate.trust >= query.trust ? 1.0 : 0.0  (hard constraint)
  - match_score for REGION_PREFER:  candidate.region in query.regions ? 1.0 : 0.5
  - match_score for REGION_EXCLUDE: candidate.region in query.excludes ? -∞ : 1.0  (hard constraint)

Default weights: CAPABILITY=0.3, LATENCY=0.25, COST=0.2, CONTEXT_WINDOW=0.15, TRUST=0.1
Weights are configurable per-deployment via strandroute.yaml.
```

---

## 5. Functional Requirements

### 5.1 SAD Operations

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-S-001 | Encode a `sad_t` struct to compact binary format (zero-copy compatible, parseable by P4 TCAM) | P0 |
| NR-S-002 | Decode binary SAD from StrandLink frame options into `sad_t` struct | P0 |
| NR-S-003 | Validate SAD fields: known types have correct lengths, version is supported, total_length matches | P0 |
| NR-S-004 | SAD matching engine: given a route_entry's capabilities and a SAD query, compute match score | P0 |
| NR-S-005 | Support wildcard SADs: a SAD with zero fields matches any endpoint (used for broadcast/discovery) | P1 |

### 5.2 Routing Table

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-RT-001 | Lock-free concurrent routing table using RCU (Read-Copy-Update) for read-heavy workloads | P0 |
| NR-RT-002 | Support 100,000+ route entries with < 10μs lookup time (benchmarked) | P0 |
| NR-RT-003 | TTL-based entry expiration with background garbage collection | P0 |
| NR-RT-004 | Incremental update: insert/remove/update individual entries without full table rebuild | P0 |
| NR-RT-005 | Export routing table state as JSON for observability and debugging | P1 |

### 5.3 Gossip Protocol (Capability Advertisement)

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-G-001 | Implement HyParView membership protocol for maintaining partial mesh of routing peers | P0 |
| NR-G-002 | Capability advertisements propagated via gossip with configurable fanout (default: 3) | P0 |
| NR-G-003 | Convergence time: new capability advertisement reaches 95% of nodes within O(log N) gossip rounds | P0 |
| NR-G-004 | Anti-entropy mechanism: periodic full-state exchange between random peers to repair missed updates | P1 |
| NR-G-005 | Gossip messages authenticated via StrandTrust node identity (prevent route injection attacks) | P1 |
| NR-G-006 | Configurable gossip interval (default: 1 second), TTL for advertisements (default: 30 seconds) | P0 |
| NR-G-007 | Support both push (proactive advertisement) and pull (on-demand query) modes | P1 |

### 5.4 Resolution & Forwarding

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-F-001 | Resolve SAD query to best-matching Node ID(s) using multi-constraint scoring | P0 |
| NR-F-002 | Support returning top-K results for multipath forwarding (configurable K, default 3) | P0 |
| NR-F-003 | Weighted multipath: distribute frames across top-K results using weighted consistent hashing | P0 |
| NR-F-004 | Software forwarding engine: receive StrandLink frame → extract SAD → resolve → rewrite dest_node_id → forward | P0 |
| NR-F-005 | Maintain per-path latency metrics via periodic probe frames (StrandLink heartbeat type) | P1 |
| NR-F-006 | Circuit-breaker: remove endpoints from resolution results when consecutive failures exceed threshold | P1 |

### 5.5 P4 Dataplane

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-P4-001 | P4_16 parser for StrandLink frame header (64-byte fixed header extraction) | P0 |
| NR-P4-002 | P4 TCAM table for SAD field matching on first 3 fields (MODEL_ARCH, CAPABILITY, CONTEXT_WINDOW) | P0 |
| NR-P4-003 | P4 exact-match table for Node ID forwarding (dest_node_id → egress port) | P0 |
| NR-P4-004 | P4 counter externs for per-stream packet/byte statistics | P1 |
| NR-P4-005 | Compile and test against BMv2 behavioral model for functional verification | P0 |
| NR-P4-006 | Compile against Tofino SDE for Intel Tofino/Tofino2 targets | P2 |
| NR-P4-007 | PSA (Portable Switch Architecture) compliance for multi-vendor portability | P1 |

### 5.6 SONiC Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| NR-SONIC-001 | Implement SAI (Switch Abstraction Interface) plugin for StrandRoute table programming | P1 |
| NR-SONIC-002 | Register StrandRoute as a SONiC Docker container with proper systemd service management | P1 |
| NR-SONIC-003 | Expose StrandRoute configuration via SONiC ConfigDB (JSON schema) | P1 |
| NR-SONIC-004 | Integrate with SONiC's syslog and telemetry infrastructure | P2 |

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NR-NF-001 | SAD encode/decode latency | < 500ns |
| NR-NF-002 | Routing table lookup (resolve) | < 10μs for 100K entries |
| NR-NF-003 | Gossip protocol memory overhead | < 50MB for 100K route entries |
| NR-NF-004 | Software forwarding throughput | > 1 Mpps single-core (C dataplane) |
| NR-NF-005 | P4 dataplane throughput | Line-rate on Tofino2 (6.4 Tbps) |
| NR-NF-006 | Gossip convergence (1000-node network) | < 5 seconds to 95% propagation |
| NR-NF-007 | Zero dynamic memory allocation in forwarding hot path | After initialization |

---

## 7. Build & Compilation

```bash
# Build C components
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release -DSONIC_INTEGRATION=ON
make -j$(nproc)

# Build P4 for BMv2 (behavioral model testing)
p4c --target bmv2 --arch v1model p4/strandroute.p4 -o build/strandroute.bmv2.json

# Build P4 for Tofino (requires Intel P4 Studio SDE)
# p4_build.sh p4/strandroute.p4 --target tofino2

# Run tests
cd build && ctest --output-on-failure

# Run P4 behavioral tests
python tests/test_p4_behavioral.py
```

### CMake Options

| Option | Default | Description |
|--------|---------|-------------|
| `SONIC_INTEGRATION` | OFF | Build SONiC SAI plugin and Docker container |
| `P4_BMV2` | ON | Build P4 BMv2 target for testing |
| `P4_TOFINO` | OFF | Build for Intel Tofino (requires SDE) |
| `ENABLE_ASAN` | OFF | Address sanitizer for development |
| `ENABLE_TSAN` | OFF | Thread sanitizer for concurrency testing |
| `STRANDLINK_INCLUDE_DIR` | `../strandlink/include` | Path to StrandLink C headers |

---

## 8. Testing Requirements

| Test Type | Description | Coverage |
|-----------|-------------|----------|
| Unit tests | SAD encode/decode, matching, routing table CRUD, multipath distribution | 90%+ |
| Concurrency tests | Routing table under concurrent read/write with thread sanitizer | All write paths |
| Gossip simulation | Simulate 100-1000 node network, verify convergence time and consistency | Statistical |
| P4 behavioral tests | BMv2 model: inject StrandLink frames, verify SAD matching and forwarding | All P4 tables |
| Resolver accuracy tests | Pre-defined SAD queries with known-best answers, verify scoring correctness | All field types |
| Integration tests | StrandLink frames from Zig → StrandRoute C forwarding → StrandLink output | Full datapath |

---

## 9. Dependencies

| Dependency | Version | Purpose | Required For |
|------------|---------|---------|--------------|
| C17 compiler (gcc/clang) | GCC 12+ / Clang 15+ | Build | All |
| CMake | 3.20+ | Build system | All |
| liburcu | 0.14+ | Userspace RCU for lock-free routing table | Core |
| FlatBuffers (flatcc) | 0.6+ | SAD binary encoding reference/tooling | SAD module |
| p4c | Latest | P4 compiler | P4 targets |
| BMv2 | Latest | P4 behavioral model for testing | Testing |
| StrandLink C FFI | — | Frame encode/decode from StrandLink module | Core |
| Intel Tofino SDE | 9.13+ | Tofino target compilation | Tofino builds only |
| SONiC build env | 202311+ | SONiC Docker container builds | SONiC integration only |
