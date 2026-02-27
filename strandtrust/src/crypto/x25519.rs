// X25519 Diffie-Hellman key exchange for session key derivation.

use hkdf::Hkdf;
use rand::rngs::OsRng;
use sha2::Sha256;
use x25519_dalek::{PublicKey, StaticSecret};

use crate::error::{StrandTrustError, Result};

/// An X25519 ephemeral keypair for one handshake.
pub struct X25519KeyPair {
    secret: StaticSecret,
    public: PublicKey,
}

impl X25519KeyPair {
    /// Generate a new random ephemeral keypair.
    pub fn generate() -> Self {
        let secret = StaticSecret::random_from_rng(OsRng);
        let public = PublicKey::from(&secret);
        Self { secret, public }
    }

    /// Create from existing secret bytes (used in deterministic tests).
    pub fn from_secret_bytes(bytes: [u8; 32]) -> Self {
        let secret = StaticSecret::from(bytes);
        let public = PublicKey::from(&secret);
        Self { secret, public }
    }

    /// The 32-byte public key.
    pub fn public_key_bytes(&self) -> [u8; 32] {
        *self.public.as_bytes()
    }

    /// The raw public key.
    pub fn public_key(&self) -> &PublicKey {
        &self.public
    }

    /// Perform Diffie-Hellman with a peer's public key, returning the 32-byte shared secret.
    pub fn diffie_hellman(&self, peer_public: &[u8; 32]) -> [u8; 32] {
        let peer_pk = PublicKey::from(*peer_public);
        let shared = self.secret.diffie_hellman(&peer_pk);
        *shared.as_bytes()
    }
}

/// Session keys derived from the X25519 shared secret via HKDF.
pub struct SessionKeys {
    pub client_write_key: [u8; 32],
    pub server_write_key: [u8; 32],
    pub client_write_iv: [u8; 12],
    pub server_write_iv: [u8; 12],
}

/// Derive session keys from a shared secret following the StrandTrust spec (section 4.2).
///
/// ```text
/// early_secret      = HKDF-Extract(salt=0, ikm=shared_secret)
/// handshake_secret  = HKDF-Expand(early_secret, "strand handshake", 32)
/// client_write_key  = HKDF-Expand(handshake_secret, "client write key" || client_id || server_id, 32)
/// server_write_key  = HKDF-Expand(handshake_secret, "server write key" || client_id || server_id, 32)
/// client_write_iv   = HKDF-Expand(handshake_secret, "client write iv"  || client_id || server_id, 12)
/// server_write_iv   = HKDF-Expand(handshake_secret, "server write iv"  || client_id || server_id, 12)
/// ```
pub fn derive_session_keys(
    shared_secret: &[u8; 32],
    client_node_id: &[u8; 16],
    server_node_id: &[u8; 16],
) -> Result<SessionKeys> {
    // Step 1: HKDF-Extract with zero salt
    let salt = [0u8; 32];
    let hk = Hkdf::<Sha256>::new(Some(&salt), shared_secret);

    // Step 2: Derive handshake_secret
    let mut handshake_secret = [0u8; 32];
    hk.expand(b"strand handshake", &mut handshake_secret)
        .map_err(|e| StrandTrustError::Encryption(format!("HKDF expand error: {e}")))?;

    // Prepare HKDF from handshake_secret (extract with no salt)
    let hk2 = Hkdf::<Sha256>::new(None, &handshake_secret);

    // Helper to build info = label || client_node_id || server_node_id
    let make_info = |label: &[u8]| -> Vec<u8> {
        let mut info = Vec::with_capacity(label.len() + 32);
        info.extend_from_slice(label);
        info.extend_from_slice(client_node_id);
        info.extend_from_slice(server_node_id);
        info
    };

    let mut client_write_key = [0u8; 32];
    let info = make_info(b"client write key");
    hk2.expand(&info, &mut client_write_key)
        .map_err(|e| StrandTrustError::Encryption(format!("HKDF expand error: {e}")))?;

    let mut server_write_key = [0u8; 32];
    let info = make_info(b"server write key");
    hk2.expand(&info, &mut server_write_key)
        .map_err(|e| StrandTrustError::Encryption(format!("HKDF expand error: {e}")))?;

    let mut client_write_iv = [0u8; 12];
    let info = make_info(b"client write iv");
    hk2.expand(&info, &mut client_write_iv)
        .map_err(|e| StrandTrustError::Encryption(format!("HKDF expand error: {e}")))?;

    let mut server_write_iv = [0u8; 12];
    let info = make_info(b"server write iv");
    hk2.expand(&info, &mut server_write_iv)
        .map_err(|e| StrandTrustError::Encryption(format!("HKDF expand error: {e}")))?;

    Ok(SessionKeys {
        client_write_key,
        server_write_key,
        client_write_iv,
        server_write_iv,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_dh_shared_secret_matches() {
        let alice = X25519KeyPair::generate();
        let bob = X25519KeyPair::generate();

        let alice_shared = alice.diffie_hellman(&bob.public_key_bytes());
        let bob_shared = bob.diffie_hellman(&alice.public_key_bytes());
        assert_eq!(alice_shared, bob_shared);
    }

    #[test]
    fn test_session_key_derivation() {
        let alice = X25519KeyPair::generate();
        let bob = X25519KeyPair::generate();

        let shared = alice.diffie_hellman(&bob.public_key_bytes());
        let client_id = [1u8; 16];
        let server_id = [2u8; 16];

        let keys1 = derive_session_keys(&shared, &client_id, &server_id).unwrap();
        let keys2 = derive_session_keys(&shared, &client_id, &server_id).unwrap();

        // Deterministic
        assert_eq!(keys1.client_write_key, keys2.client_write_key);
        assert_eq!(keys1.server_write_key, keys2.server_write_key);
        assert_eq!(keys1.client_write_iv, keys2.client_write_iv);
        assert_eq!(keys1.server_write_iv, keys2.server_write_iv);

        // Different keys for client and server
        assert_ne!(keys1.client_write_key, keys1.server_write_key);
        assert_ne!(keys1.client_write_iv, keys1.server_write_iv);
    }
}
