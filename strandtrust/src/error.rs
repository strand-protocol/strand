// NexTrust error types

use thiserror::Error;

/// Top-level error type for the NexTrust crate.
#[derive(Debug, Error)]
pub enum NexTrustError {
    // ── Crypto errors ───────────────────────────────────────────────────
    #[error("key generation failed: {0}")]
    KeyGeneration(String),

    #[error("invalid key material: {0}")]
    InvalidKey(String),

    #[error("signature verification failed")]
    SignatureVerification,

    #[error("AEAD encryption failed: {0}")]
    Encryption(String),

    #[error("AEAD decryption failed: {0}")]
    Decryption(String),

    // ── MIC errors ──────────────────────────────────────────────────────
    #[error("MIC build error: {0}")]
    MicBuild(String),

    #[error("MIC serialization error: {0}")]
    MicSerialization(String),

    #[error("MIC deserialization error: {0}")]
    MicDeserialization(String),

    #[error("MIC expired: not_after={not_after}, now={now}")]
    MicExpired { not_after: u64, now: u64 },

    #[error("MIC not yet valid: not_before={not_before}, now={now}")]
    MicNotYetValid { not_before: u64, now: u64 },

    #[error("MIC chain validation failed: {0}")]
    MicChainValidation(String),

    #[error("MIC version unsupported: {0}")]
    MicVersionUnsupported(u16),

    #[error("invalid capability: {0}")]
    InvalidCapability(String),

    // ── Handshake errors ────────────────────────────────────────────────
    #[error("handshake error: {0}")]
    Handshake(String),

    #[error("invalid handshake state transition: {from} -> {to}")]
    InvalidStateTransition { from: String, to: String },

    #[error("handshake timeout")]
    HandshakeTimeout,

    // ── FFI errors ──────────────────────────────────────────────────────
    #[error("FFI error: {0}")]
    Ffi(String),

    // ── Generic ─────────────────────────────────────────────────────────
    #[error("buffer too small: need {need}, have {have}")]
    BufferTooSmall { need: usize, have: usize },
}

/// Crate-level result alias.
pub type Result<T> = std::result::Result<T, NexTrustError>;
