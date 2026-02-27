// MIC Builder — fluent API for constructing and signing a Model Identity Certificate.

use std::time::Duration;

use crate::crypto::keys::IdentityKeyPair;
use crate::error::{StrandTrustError, Result};
use crate::mic::{Capability, Provenance, MIC};

/// Builder for constructing a [`MIC`] with a fluent API.
///
/// # Example
/// ```ignore
/// let mic = MICBuilder::new(&keypair)
///     .model_hash(hash)
///     .add_capability(Capability::TextGeneration)
///     .valid_for(Duration::from_secs(86400 * 30))
///     .build()?;
/// ```
pub struct MICBuilder<'a> {
    keypair: &'a IdentityKeyPair,
    model_hash: Option<[u8; 32]>,
    capabilities: Vec<Capability>,
    training_provenance: Option<Provenance>,
    valid_from: Option<u64>,
    valid_until: Option<u64>,
}

impl<'a> MICBuilder<'a> {
    /// Start building a MIC that will be signed by `keypair`.
    pub fn new(keypair: &'a IdentityKeyPair) -> Self {
        Self {
            keypair,
            model_hash: None,
            capabilities: Vec::new(),
            training_provenance: None,
            valid_from: None,
            valid_until: None,
        }
    }

    /// Set the model hash (SHA-256 of model weights / binary).
    pub fn model_hash(mut self, hash: [u8; 32]) -> Self {
        self.model_hash = Some(hash);
        self
    }

    /// Add a capability attestation.
    pub fn add_capability(mut self, cap: Capability) -> Self {
        self.capabilities.push(cap);
        self
    }

    /// Set training provenance.
    pub fn training_provenance(mut self, prov: Provenance) -> Self {
        self.training_provenance = Some(prov);
        self
    }

    /// Set explicit validity window (unix timestamps in seconds).
    pub fn validity(mut self, from: u64, until: u64) -> Self {
        self.valid_from = Some(from);
        self.valid_until = Some(until);
        self
    }

    /// Set validity as a duration from "now" (unix epoch seconds).
    /// Uses the provided `now` timestamp so the builder is deterministic in tests.
    pub fn valid_for_from(mut self, now: u64, duration: Duration) -> Self {
        self.valid_from = Some(now);
        self.valid_until = Some(now + duration.as_secs());
        self
    }

    /// Set validity as a duration from the current wall-clock time.
    pub fn valid_for(self, duration: Duration) -> Self {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .expect("system clock before epoch")
            .as_secs();
        self.valid_for_from(now, duration)
    }

    /// Consume the builder and produce a signed [`MIC`].
    pub fn build(self) -> Result<MIC> {
        let model_hash = self
            .model_hash
            .ok_or_else(|| StrandTrustError::MicBuild("model_hash is required".into()))?;

        let valid_from = self
            .valid_from
            .ok_or_else(|| StrandTrustError::MicBuild("validity window is required".into()))?;
        let valid_until = self
            .valid_until
            .ok_or_else(|| StrandTrustError::MicBuild("validity window is required".into()))?;

        if valid_until <= valid_from {
            return Err(StrandTrustError::MicBuild(
                "valid_until must be after valid_from".into(),
            ));
        }

        // Build the node_id field: we use the 32-byte public key as the node_id
        // in the MIC (distinct from the 16-byte truncated NodeId used in networking).
        let node_id = self.keypair.public_key_bytes();
        let issuer_public_key = self.keypair.public_key_bytes();

        // Build an unsigned MIC so we can compute signable_bytes.
        let mut mic = MIC {
            node_id,
            model_hash,
            capabilities: self.capabilities,
            training_provenance: self.training_provenance,
            valid_from,
            valid_until,
            signature: [0u8; 64],
            issuer_public_key,
        };

        // Sign the canonical signable bytes.
        let signable = mic.signable_bytes();
        mic.signature = self.keypair.sign(&signable);

        Ok(mic)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::mic::Capability;

    #[test]
    fn build_minimal_mic() {
        let kp = IdentityKeyPair::generate();
        let mic = MICBuilder::new(&kp)
            .model_hash([0xAA; 32])
            .validity(1000, 2000)
            .build()
            .unwrap();
        assert_eq!(mic.model_hash, [0xAA; 32]);
        assert_eq!(mic.valid_from, 1000);
        assert_eq!(mic.valid_until, 2000);
        assert!(mic.capabilities.is_empty());
        assert!(mic.training_provenance.is_none());
    }

    #[test]
    fn build_with_capabilities() {
        let kp = IdentityKeyPair::generate();
        let mic = MICBuilder::new(&kp)
            .model_hash([0xBB; 32])
            .add_capability(Capability::TextGeneration)
            .add_capability(Capability::CodeGeneration)
            .validity(1000, 2000)
            .build()
            .unwrap();
        assert_eq!(mic.capabilities.len(), 2);
    }

    #[test]
    fn missing_model_hash_fails() {
        let kp = IdentityKeyPair::generate();
        let result = MICBuilder::new(&kp).validity(1000, 2000).build();
        assert!(result.is_err());
    }

    #[test]
    fn missing_validity_fails() {
        let kp = IdentityKeyPair::generate();
        let result = MICBuilder::new(&kp).model_hash([0; 32]).build();
        assert!(result.is_err());
    }

    #[test]
    fn invalid_validity_window_fails() {
        let kp = IdentityKeyPair::generate();
        let result = MICBuilder::new(&kp)
            .model_hash([0; 32])
            .validity(2000, 1000) // until < from
            .build();
        assert!(result.is_err());
    }
}
