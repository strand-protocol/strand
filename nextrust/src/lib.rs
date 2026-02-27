// NexTrust L4 — Model Identity, Cryptographic Trust & Attestation
//
// Crate root: module declarations and public re-exports.

pub mod error;
pub mod crypto;
pub mod mic;
pub mod handshake;
pub mod ffi;

// Re-export key types at crate root for convenience.
pub use error::{NexTrustError, Result};
pub use crypto::keys::IdentityKeyPair;
pub use mic::MIC;
