// Ed25519 keypair generation, Node ID derivation, serialization

use ed25519_dalek::{Signature, Signer, SigningKey, Verifier, VerifyingKey};
use rand::rngs::OsRng;
use sha2::{Digest, Sha256};

use crate::error::{NexTrustError, Result};

/// 16-byte Node ID derived from truncated SHA-256 of the public key.
pub type NodeId = [u8; 16];

/// An Ed25519 identity keypair with its derived Node ID.
#[derive(Debug)]
pub struct IdentityKeyPair {
    signing_key: SigningKey,
    verifying_key: VerifyingKey,
    node_id: NodeId,
}

impl IdentityKeyPair {
    /// Generate a fresh random Ed25519 keypair.
    pub fn generate() -> Self {
        let signing_key = SigningKey::generate(&mut OsRng);
        let verifying_key = signing_key.verifying_key();
        let node_id = derive_node_id(&verifying_key);
        Self {
            signing_key,
            verifying_key,
            node_id,
        }
    }

    /// Reconstruct from a 32-byte secret seed.
    pub fn from_seed(seed: &[u8; 32]) -> Self {
        let signing_key = SigningKey::from_bytes(seed);
        let verifying_key = signing_key.verifying_key();
        let node_id = derive_node_id(&verifying_key);
        Self {
            signing_key,
            verifying_key,
            node_id,
        }
    }

    /// The 16-byte Node ID.
    pub fn node_id(&self) -> &NodeId {
        &self.node_id
    }

    /// The 32-byte Ed25519 public key.
    pub fn public_key_bytes(&self) -> [u8; 32] {
        self.verifying_key.to_bytes()
    }

    /// The 32-byte secret key seed.
    pub fn secret_key_bytes(&self) -> [u8; 32] {
        self.signing_key.to_bytes()
    }

    /// Access the raw verifying (public) key.
    pub fn verifying_key(&self) -> &VerifyingKey {
        &self.verifying_key
    }

    /// Access the raw signing (private) key.
    pub fn signing_key(&self) -> &SigningKey {
        &self.signing_key
    }

    /// Sign arbitrary data.
    pub fn sign(&self, data: &[u8]) -> [u8; 64] {
        let sig: Signature = self.signing_key.sign(data);
        sig.to_bytes()
    }

    /// Verify a signature against the public key.
    pub fn verify(&self, data: &[u8], signature: &[u8; 64]) -> Result<()> {
        let sig = Signature::from_bytes(signature);
        self.verifying_key
            .verify(data, &sig)
            .map_err(|_| NexTrustError::SignatureVerification)
    }
}

/// Derive a 128-bit Node ID from an Ed25519 public key:
/// Node ID = first 16 bytes of SHA-256(public_key).
pub fn derive_node_id(pubkey: &VerifyingKey) -> NodeId {
    let hash = Sha256::digest(pubkey.as_bytes());
    let mut id = [0u8; 16];
    id.copy_from_slice(&hash[..16]);
    id
}

/// Verify a signature given raw public key bytes, message, and signature bytes.
pub fn verify_signature(
    pubkey_bytes: &[u8; 32],
    message: &[u8],
    signature: &[u8; 64],
) -> Result<()> {
    let vk = VerifyingKey::from_bytes(pubkey_bytes)
        .map_err(|e| NexTrustError::InvalidKey(format!("{e}")))?;
    let sig = Signature::from_bytes(signature);
    vk.verify(message, &sig)
        .map_err(|_| NexTrustError::SignatureVerification)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_keypair_roundtrip() {
        let kp = IdentityKeyPair::generate();
        let seed = kp.secret_key_bytes();
        let kp2 = IdentityKeyPair::from_seed(&seed);
        assert_eq!(kp.public_key_bytes(), kp2.public_key_bytes());
        assert_eq!(kp.node_id(), kp2.node_id());
    }

    #[test]
    fn test_sign_verify() {
        let kp = IdentityKeyPair::generate();
        let msg = b"hello nextrust";
        let sig = kp.sign(msg);
        kp.verify(msg, &sig).expect("signature should be valid");
    }

    #[test]
    fn test_verify_wrong_message() {
        let kp = IdentityKeyPair::generate();
        let sig = kp.sign(b"correct message");
        let result = kp.verify(b"wrong message", &sig);
        assert!(result.is_err());
    }

    #[test]
    fn test_node_id_deterministic() {
        let kp = IdentityKeyPair::generate();
        let id1 = derive_node_id(kp.verifying_key());
        let id2 = derive_node_id(kp.verifying_key());
        assert_eq!(id1, id2);
    }
}
