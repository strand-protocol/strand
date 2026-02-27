# StrandTrust — Layer 4: Model Identity, Cryptographic Trust & Attestation

## Module: `strandtrust/`

## Language: Rust (Edition 2021, MSRV 1.75+)

## Crate Type: Library (`lib`)

---

## 1. Overview

StrandTrust replaces the web PKI (X.509 certificates, TLS handshake, Certificate Authorities) with a purpose-built identity and trust system for AI systems. Instead of proving "you own this domain name," StrandTrust proves "this model has these properties, was trained on this data, achieves these benchmarks, and is endorsed by these organizations" — all cryptographically verifiable, with optional zero-knowledge proofs that attest to properties without revealing proprietary information.

StrandTrust issues **Model Identity Certificates (MICs)** — the AI equivalent of X.509 certificates — and provides a handshake protocol that establishes authenticated, encrypted channels between AI agents with mutual capability attestation.

---

## 2. Standards & RFCs Being Replaced / Extended

| Standard | Title | Relevance |
|----------|-------|-----------|
| **RFC 8446** | The Transport Layer Security (TLS) Protocol Version 1.3 | StrandTrust replaces TLS for AI-to-AI communication. The StrandTrust handshake protocol is architecturally inspired by TLS 1.3's 1-RTT design but uses model identity instead of domain certificates |
| **RFC 5280** | Internet X.509 Public Key Infrastructure Certificate and CRL Profile | StrandTrust's Model Identity Certificate (MIC) replaces X.509 certificates. MIC schema designed for AI properties instead of organizational identity |
| **RFC 6960** | X.509 Internet PKI Online Certificate Status Protocol (OCSP) | Reference for certificate revocation — StrandTrust implements a revocation mechanism for compromised or deprecated model identities |
| **RFC 6962** | Certificate Transparency | Reference for auditability — StrandTrust implements a Model Identity Transparency Log for public verifiability of all issued MICs |
| **RFC 5958** | Asymmetric Key Packages | Reference for key serialization formats |
| **RFC 7519** | JSON Web Token (JWT) | Reference for token-based attestation claims. StrandTrust uses a binary equivalent (not JSON) for efficiency |
| **RFC 9180** | Hybrid Public Key Encryption (HPKE) | Reference for StrandTrust's channel encryption. StrandTrust uses HPKE-style KEM+AEAD for session key establishment |
| **RFC 7748** | Elliptic Curves for Security (X25519, X448) | Key agreement curves used in StrandTrust handshake |
| **RFC 8032** | Edwards-Curve Digital Signature Algorithm (EdDSA, Ed25519) | Signature algorithm for MIC signing and handshake authentication |
| **RFC 5869** | HMAC-based Extract-and-Expand Key Derivation Function (HKDF) | Key derivation for session keys from handshake material |

### Cryptographic Primitive References

| Primitive | Specification | Purpose in StrandTrust |
|-----------|--------------|---------------------|
| **Ed25519** | RFC 8032 | Node identity key pair, MIC signatures |
| **X25519** | RFC 7748 | Ephemeral key exchange in handshake |
| **HKDF-SHA256** | RFC 5869 | Session key derivation |
| **AES-256-GCM** | NIST SP 800-38D | Symmetric encryption for StrandStream data (post-handshake) |
| **ChaCha20-Poly1305** | RFC 8439 | Alternative AEAD cipher (for platforms without AES-NI) |
| **SHA-256 / SHA-3** | FIPS 180-4 / FIPS 202 | Hashing for MIC fingerprints, Merkle trees |
| **Groth16 zk-SNARK** | Groth, 2016 (via arkworks) | Zero-knowledge proofs of model properties |
| **BLS12-381** | — | Pairing-friendly curve for zk-SNARK proofs |
| **Merkle Tree** | RFC 6962 (CT logs) | Training data provenance, MIC transparency log |

---

## 3. Model Identity Certificate (MIC) Specification

### 3.1 MIC Structure

A MIC is a signed, structured document attesting to a model's identity and properties. Unlike X.509 which uses ASN.1/DER encoding, MICs use a compact binary format with explicit versioning.

```
MIC {
  // === Identity ===
  version:            u16,          // MIC format version (current: 1)
  serial_number:      [u8; 32],     // Unique certificate serial (SHA-256 of content)
  node_id:            [u8; 16],     // StrandLink Node ID derived from public key
  public_key:         [u8; 32],     // Ed25519 public key
  
  // === Issuer ===
  issuer_node_id:     [u8; 16],     // CA's Node ID (self-signed for root CAs)
  issuer_signature:   [u8; 64],     // Ed25519 signature over all preceding fields
  
  // === Validity ===
  not_before:         u64,          // Unix timestamp (seconds)
  not_after:          u64,          // Unix timestamp (seconds)
  
  // === Model Properties (Attestation Claims) ===
  claims:             Vec<Claim>,   // Variable-length list of attestation claims
  
  // === Extensions ===
  extensions:         Vec<Extension>, // Optional extensions
  
  // === Provenance Chain ===
  provenance:         Option<ProvenanceChain>, // Training data / pipeline provenance
}
```

### 3.2 Attestation Claims

Each claim in a MIC attests to a specific model property:

| Claim Type | ID | Value | Description |
|------------|-----|-------|-------------|
| `ARCHITECTURE_HASH` | `0x01` | `[u8; 32]` | SHA-256 hash of model architecture definition |
| `PARAMETER_COUNT` | `0x02` | `u64` | Total parameter count |
| `TRAINING_DATA_HASH` | `0x03` | `[u8; 32]` | Merkle root of training dataset hashes |
| `CAPABILITY_SET` | `0x04` | `u32` (bitfield) | Same bitfield as SAD CAPABILITY field |
| `CONTEXT_WINDOW` | `0x05` | `u32` | Maximum context window in tokens |
| `BENCHMARK_SCORE` | `0x06` | `BenchmarkClaim` | Named benchmark + score (e.g., "humaneval" → 92.1) |
| `SAFETY_EVAL` | `0x07` | `SafetyClaim` | Safety evaluation result + evaluator identity |
| `PUBLISHER_IDENTITY` | `0x08` | `PublisherInfo` | Organization name, contact, website |
| `ENDORSEMENT` | `0x09` | `Endorsement` | Third-party endorsement (signed by endorser's key) |
| `CUSTOM` | `0xFF` | `bytes` | Application-defined claim |

### 3.3 Zero-Knowledge Attestation

For claims that involve proprietary information (model weights, training data, internal benchmarks), StrandTrust supports **ZK attestation**: a Groth16 zk-SNARK proof that a property holds without revealing the underlying data.

ZK-provable claims:
- "This model achieves HumanEval pass@1 > 0.90" (without revealing exact score)
- "This model was trained on dataset with Merkle root X" (without revealing dataset contents)
- "This model's architecture hash matches H" (without revealing architecture)
- "This model has > 70B parameters" (range proof, without revealing exact count)

The ZK proof is attached as an extension to the relevant claim:

```rust
pub struct ZkAttestation {
    pub claim_type: ClaimType,
    pub circuit_id: [u8; 32],        // Identifier for the ZK circuit used
    pub proof: Vec<u8>,               // Groth16 proof bytes (serialized via arkworks)
    pub public_inputs: Vec<[u8; 32]>, // Public inputs to the circuit
    pub verifying_key_hash: [u8; 32], // Hash of the verifying key (for key lookup)
}
```

### 3.4 Provenance Chain

A provenance chain is a Merkle tree recording the lineage of a model:

```
ProvenanceChain {
  root_hash:          [u8; 32],       // Merkle root
  entries: [
    ProvenanceEntry {
      entry_type:     ProvenanceType,  // dataset, fine_tune, rlhf, merge, distill, quantize
      timestamp:      u64,
      parent_hashes:  Vec<[u8; 32]>,   // Previous entries this builds on
      description:    String,           // Human-readable description
      actor_node_id:  [u8; 16],        // Who performed this step
      signature:      [u8; 64],        // Actor's signature over this entry
    }
  ]
}
```

---

## 4. Handshake Protocol

### 4.1 StrandTrust Handshake (1-RTT Mutual Authentication)

```
Client                                        Server
  |                                              |
  |--- TRUST_HELLO --------------------------->|
  |     client_node_id                          |
  |     client_ephemeral_pubkey (X25519)        |
  |     supported_ciphers                       |
  |     client_MIC (or MIC fingerprint)         |
  |     requested_attestation_level             |
  |                                              |
  |<-- TRUST_ACCEPT ----------------------------|
  |     server_node_id                          |
  |     server_ephemeral_pubkey (X25519)        |
  |     selected_cipher                         |
  |     server_MIC                              |
  |     server_attestation_proofs[]             |
  |     encrypted{server_finished}              |
  |                                              |
  |--- TRUST_FINISH --------------------------->|
  |     client_attestation_proofs[]             |
  |     encrypted{client_finished}              |
  |                                              |
  |========= ENCRYPTED CHANNEL ESTABLISHED =====|
```

### 4.2 Session Key Derivation

```
shared_secret = X25519(client_ephemeral_privkey, server_ephemeral_pubkey)
early_secret = HKDF-Extract(salt=0, ikm=shared_secret)
handshake_secret = HKDF-Expand(early_secret, "strand handshake", 32)
client_write_key = HKDF-Expand(handshake_secret, "client write key" || client_node_id || server_node_id, 32)
server_write_key = HKDF-Expand(handshake_secret, "server write key" || client_node_id || server_node_id, 32)
client_write_iv = HKDF-Expand(handshake_secret, "client write iv" || client_node_id || server_node_id, 12)
server_write_iv = HKDF-Expand(handshake_secret, "server write iv" || client_node_id || server_node_id, 12)
```

### 4.3 Cipher Suites

| ID | Name | Key Exchange | Signature | AEAD | Hash |
|----|------|-------------|-----------|------|------|
| `0x0001` | `STRAND_X25519_ED25519_AES256GCM_SHA256` | X25519 | Ed25519 | AES-256-GCM | SHA-256 |
| `0x0002` | `STRAND_X25519_ED25519_CHACHA20POLY1305_SHA256` | X25519 | Ed25519 | ChaCha20-Poly1305 | SHA-256 |

---

## 5. Architecture & Components

### 5.1 Source Tree Structure

```
strandtrust/
├── Cargo.toml
├── src/
│   ├── lib.rs                       # Crate root, public API
│   ├── mic/
│   │   ├── mod.rs                   # MIC type definitions and API
│   │   ├── builder.rs               # MIC builder pattern for construction
│   │   ├── parser.rs                # MIC binary encoding/decoding
│   │   ├── validator.rs             # MIC validation (signature, expiry, chain)
│   │   ├── claims.rs                # Attestation claim types and serialization
│   │   ├── extensions.rs            # MIC extension types
│   │   └── provenance.rs            # Provenance chain construction and verification
│   ├── ca/
│   │   ├── mod.rs                   # Certificate Authority interface
│   │   ├── issuer.rs                # MIC issuance logic
│   │   ├── revocation.rs            # Revocation list management (CRL equivalent)
│   │   ├── transparency_log.rs      # Append-only MIC transparency log (RFC 6962 inspired)
│   │   └── store.rs                 # MIC storage backend trait + in-memory/file implementations
│   ├── handshake/
│   │   ├── mod.rs                   # Handshake protocol state machine
│   │   ├── client.rs                # Client-side handshake implementation
│   │   ├── server.rs                # Server-side handshake implementation
│   │   ├── frames.rs               # TRUST_HELLO / TRUST_ACCEPT / TRUST_FINISH frame encoding
│   │   └── session.rs              # Post-handshake session state (keys, cipher)
│   ├── crypto/
│   │   ├── mod.rs                   # Crypto abstraction layer
│   │   ├── keys.rs                  # Ed25519 key generation, serialization, Node ID derivation
│   │   ├── x25519.rs               # X25519 key exchange
│   │   ├── aead.rs                  # AES-256-GCM and ChaCha20-Poly1305 encrypt/decrypt
│   │   ├── hkdf.rs                  # HKDF key derivation
│   │   ├── hash.rs                  # SHA-256, SHA-3 hashing
│   │   └── merkle.rs               # Merkle tree construction and verification
│   ├── zk/
│   │   ├── mod.rs                   # Zero-knowledge proof API
│   │   ├── circuits/
│   │   │   ├── benchmark_range.rs   # Circuit: benchmark score in range [min, max]
│   │   │   ├── param_count_range.rs # Circuit: parameter count range proof
│   │   │   ├── dataset_membership.rs # Circuit: training dataset Merkle membership
│   │   │   └── architecture_match.rs # Circuit: architecture hash matches
│   │   ├── prover.rs               # Groth16 proof generation (arkworks)
│   │   ├── verifier.rs             # Groth16 proof verification (arkworks)
│   │   └── setup.rs                # Trusted setup / ceremony for circuit-specific keys
│   ├── encrypt.rs                   # Post-handshake encryption/decryption of StrandStream frames
│   ├── identity.rs                  # Node identity management (keypair, Node ID)
│   ├── config.rs                    # Configuration
│   └── error.rs                     # Error types
├── tests/
│   ├── mic_test.rs                  # MIC creation, serialization, validation roundtrip
│   ├── handshake_test.rs            # Full handshake between client and server
│   ├── revocation_test.rs           # MIC revocation and CRL checking
│   ├── zk_test.rs                   # ZK proof generation and verification
│   ├── provenance_test.rs           # Provenance chain construction and verification
│   ├── encrypt_test.rs             # Encryption/decryption correctness
│   └── integration_test.rs          # Full: generate MIC → handshake → encrypted stream
├── benches/
│   ├── handshake_bench.rs           # Handshake latency
│   ├── encrypt_bench.rs             # Encryption throughput
│   ├── zk_proof_bench.rs            # ZK proof generation time
│   └── mic_validate_bench.rs        # MIC validation latency
├── fuzz/
│   ├── fuzz_mic_parse.rs            # Fuzz MIC decoder
│   └── fuzz_handshake_input.rs      # Fuzz handshake state machine
└── examples/
    ├── generate_mic.rs              # CLI tool to generate a self-signed MIC
    ├── verify_mic.rs                # CLI tool to verify a MIC
    └── handshake_demo.rs            # Demo of full handshake between two nodes
```

---

## 6. Functional Requirements

### 6.1 Model Identity Certificates

| ID | Requirement | Priority |
|----|-------------|----------|
| NT-MIC-001 | Generate Ed25519 keypair and derive 128-bit Node ID (truncated SHA-256 of public key) | P0 |
| NT-MIC-002 | Build MIC with builder pattern: add claims, extensions, provenance, sign with issuer key | P0 |
| NT-MIC-003 | Serialize MIC to compact binary format (not ASN.1, not JSON — custom binary with version prefix) | P0 |
| NT-MIC-004 | Deserialize MIC from binary with full validation: version, signature, expiry, issuer chain | P0 |
| NT-MIC-005 | Support MIC chain validation: verify chain from leaf MIC to trusted root CA | P0 |
| NT-MIC-006 | MIC fingerprint: SHA-256 hash of the serialized MIC for compact reference | P0 |
| NT-MIC-007 | MIC pinning: allow nodes to pin specific MIC fingerprints for known peers (TOFU model) | P1 |

### 6.2 Certificate Authority

| ID | Requirement | Priority |
|----|-------------|----------|
| NT-CA-001 | Issue MICs: accept CSR (Certificate Signing Request), validate, sign, return MIC | P0 |
| NT-CA-002 | Maintain revocation list: revoke MICs by serial number, serve CRL to requesters | P0 |
| NT-CA-003 | Transparency log: append-only log of all issued MICs (Merkle tree, RFC 6962 inspired) | P1 |
| NT-CA-004 | MIC storage: pluggable backend (in-memory for testing, file-based for single node, database for production) | P0 |
| NT-CA-005 | Root CA self-signing: generate self-signed root MIC for bootstrapping trust | P0 |
| NT-CA-006 | Intermediate CA support: issue intermediate CA MICs that can sign leaf MICs | P1 |

### 6.3 Handshake Protocol

| ID | Requirement | Priority |
|----|-------------|----------|
| NT-HS-001 | Implement full 1-RTT mutual handshake: TRUST_HELLO → TRUST_ACCEPT → TRUST_FINISH | P0 |
| NT-HS-002 | Handshake state machine with proper error handling for every invalid transition | P0 |
| NT-HS-003 | Session key derivation via X25519 + HKDF-SHA256 as specified in §4.2 | P0 |
| NT-HS-004 | Mutual MIC exchange and validation during handshake | P0 |
| NT-HS-005 | Attestation level negotiation: client requests minimum level, server proves it meets requirements | P0 |
| NT-HS-006 | ZK proof exchange during handshake when attestation requires it (e.g., proving benchmark without revealing score) | P1 |
| NT-HS-007 | Session resumption: 0-RTT reconnect using cached session tickets (reference TLS 1.3 0-RTT, RFC 8446 §2.3) | P2 |
| NT-HS-008 | Handshake timeout: configurable, default 5 seconds | P0 |

### 6.4 Encryption

| ID | Requirement | Priority |
|----|-------------|----------|
| NT-ENC-001 | Encrypt StrandStream frames with AES-256-GCM using session keys from handshake | P0 |
| NT-ENC-002 | Support ChaCha20-Poly1305 as alternative AEAD (selected during handshake) | P0 |
| NT-ENC-003 | Per-frame nonce: 96-bit nonce derived from frame sequence number + IV (no nonce reuse) | P0 |
| NT-ENC-004 | Key rotation: derive new traffic keys after configurable number of frames (default: 2^20) or bytes (default: 2^30) | P1 |
| NT-ENC-005 | Encrypt/decrypt must be zero-copy compatible: operate on StrandLink ring buffer slots in-place | P0 |
| NT-ENC-006 | Support optional encryption bypass for trusted network segments (configurable per-connection) | P1 |

### 6.5 Zero-Knowledge Proofs

| ID | Requirement | Priority |
|----|-------------|----------|
| NT-ZK-001 | Implement Groth16 prover for benchmark range proofs using arkworks/groth16 on BLS12-381 | P1 |
| NT-ZK-002 | Implement Groth16 verifier (must be fast enough for inline handshake verification: < 10ms) | P1 |
| NT-ZK-003 | Trusted setup ceremony: generate per-circuit proving/verifying key pairs | P1 |
| NT-ZK-004 | Pre-built circuits: benchmark_range, param_count_range, dataset_membership, architecture_match | P1 |
| NT-ZK-005 | Circuit ID registry: map circuit IDs to verifying keys for verification | P1 |
| NT-ZK-006 | Proof serialization: compact binary format for embedding in MICs and handshake frames | P1 |

---

## 7. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NT-NF-001 | Handshake latency (1-RTT, no ZK proofs) | < 2ms on localhost, < 10ms cross-datacenter |
| NT-NF-002 | MIC validation latency (single MIC, no chain) | < 100μs |
| NT-NF-003 | MIC chain validation (depth 3) | < 500μs |
| NT-NF-004 | AES-256-GCM encryption throughput | > 10 GB/s with AES-NI |
| NT-NF-005 | ChaCha20-Poly1305 encryption throughput | > 5 GB/s |
| NT-NF-006 | ZK proof generation (Groth16, benchmark range) | < 5 seconds |
| NT-NF-007 | ZK proof verification (Groth16) | < 10ms |
| NT-NF-008 | MIC serialized size (typical, no ZK proofs) | < 2KB |
| NT-NF-009 | MIC serialized size (with ZK attestation) | < 4KB |

---

## 8. Build & Compilation

```bash
# Build (default features)
cargo build --release

# Build without ZK proofs (smaller binary, faster compile)
cargo build --release --no-default-features --features std,crypto

# Run tests
cargo test

# Run ZK-specific tests (slow — proof generation)
cargo test --features zk -- --test-threads=1

# Benchmarks
cargo bench

# Fuzz
cargo fuzz run fuzz_mic_parse -- -max_total_time=300
```

### Cargo Features

| Feature | Default | Description |
|---------|---------|-------------|
| `std` | Yes | Standard library support |
| `crypto` | Yes | Core cryptographic operations (Ed25519, X25519, AEAD, HKDF) |
| `zk` | Yes | Zero-knowledge proof support (adds arkworks dependency, increases compile time) |
| `ca` | Yes | Certificate Authority functionality |
| `transparency` | No | MIC transparency log (adds database dependency) |

---

## 9. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `ed25519-dalek` | 2.1+ | Ed25519 signatures |
| `x25519-dalek` | 2.0+ | X25519 key exchange |
| `aes-gcm` | 0.10+ | AES-256-GCM AEAD encryption |
| `chacha20poly1305` | 0.10+ | ChaCha20-Poly1305 AEAD encryption |
| `hkdf` | 0.12+ | HKDF key derivation |
| `sha2` | 0.10+ | SHA-256 hashing |
| `sha3` | 0.10+ | SHA-3 hashing |
| `ark-groth16` | 0.4+ | Groth16 zk-SNARK prover/verifier |
| `ark-bls12-381` | 0.4+ | BLS12-381 pairing curve for Groth16 |
| `ark-relations` | 0.4+ | R1CS constraint system for ZK circuits |
| `ark-serialize` | 0.4+ | Serialization for arkworks types |
| `rand` | 0.8+ | Cryptographic random number generation |
| `zeroize` | 1.7+ | Secure memory zeroing for key material |
| `tracing` | 0.1+ | Structured logging |
| `criterion` | 0.5+ | Benchmarking (dev) |
| `proptest` | 1.4+ | Property testing (dev) |

---

## 10. Security Considerations

| Concern | Mitigation |
|---------|------------|
| Key compromise | Immediate MIC revocation via CA. Short-lived MICs (default: 30 days). Key rotation supported |
| Replay attack | Handshake includes ephemeral keys and timestamps. Session tickets have expiry |
| Nonce reuse | Nonces derived from monotonic sequence numbers. Key rotation enforced before nonce space exhaustion |
| Side-channel attacks | Use constant-time operations from `*-dalek` crates. Zeroize key material on drop |
| ZK trusted setup compromise | Per-circuit setup with multi-party ceremony. Verifying keys published in transparency log |
| Man-in-the-middle | Mutual authentication via MIC exchange. Node ID derived from public key (self-certifying) |
| Quantum resistance | Future extension: post-quantum KEM (ML-KEM/Kyber) as additional cipher suite. Current design is modular to support swap |
