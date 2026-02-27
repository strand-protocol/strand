// AEAD cipher suites: ChaCha20-Poly1305 (RFC 8439) and AES-256-GCM.
//
// Suite IDs match the NexTrust cipher suite negotiation spec:
//   0x0001 NEXUS_X25519_ED25519_AES256GCM_SHA256
//   0x0002 NEXUS_X25519_ED25519_CHACHA20POLY1305_SHA256

// Both aes-gcm and chacha20poly1305 re-export the same `aead` traits.
// Import once from aes_gcm to avoid redundant imports.
use aes_gcm::aead::{Aead, KeyInit, Payload};
use aes_gcm::{Aes256Gcm, Nonce as AesNonce};
use chacha20poly1305::{ChaCha20Poly1305, Nonce};

use crate::error::{NexTrustError, Result};

/// Cipher suite identifier (wire value).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CipherSuite {
    /// AES-256-GCM — suite ID 0x0001.
    Aes256Gcm,
    /// ChaCha20-Poly1305 — suite ID 0x0002.
    ChaCha20Poly1305,
}

impl CipherSuite {
    /// Wire ID used during handshake negotiation.
    pub fn wire_id(self) -> u16 {
        match self {
            CipherSuite::Aes256Gcm => 0x0001,
            CipherSuite::ChaCha20Poly1305 => 0x0002,
        }
    }

    /// Resolve from a wire ID.
    pub fn from_wire_id(id: u16) -> Option<Self> {
        match id {
            0x0001 => Some(CipherSuite::Aes256Gcm),
            0x0002 => Some(CipherSuite::ChaCha20Poly1305),
            _ => None,
        }
    }
}

/// ChaCha20-Poly1305 authenticated encryption with associated data.
pub struct AeadCipher {
    key: [u8; 32],
}

impl AeadCipher {
    /// Create a new AEAD cipher from a 32-byte key.
    pub fn new(key: [u8; 32]) -> Self {
        Self { key }
    }

    /// Encrypt `plaintext` with the given 12-byte `nonce` and optional associated data `aad`.
    ///
    /// Returns ciphertext || 16-byte Poly1305 tag.
    pub fn encrypt(&self, nonce: &[u8; 12], plaintext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        let cipher = ChaCha20Poly1305::new_from_slice(&self.key)
            .map_err(|e| NexTrustError::Encryption(format!("cipher init: {e}")))?;
        let nonce = Nonce::from_slice(nonce);
        let payload = Payload { msg: plaintext, aad };
        cipher
            .encrypt(nonce, payload)
            .map_err(|e| NexTrustError::Encryption(format!("{e}")))
    }

    /// Decrypt `ciphertext` (which includes the appended 16-byte tag) with the given
    /// 12-byte `nonce` and the same `aad` used during encryption.
    pub fn decrypt(&self, nonce: &[u8; 12], ciphertext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        let cipher = ChaCha20Poly1305::new_from_slice(&self.key)
            .map_err(|e| NexTrustError::Decryption(format!("cipher init: {e}")))?;
        let nonce = Nonce::from_slice(nonce);
        let payload = Payload {
            msg: ciphertext,
            aad,
        };
        cipher
            .decrypt(nonce, payload)
            .map_err(|e| NexTrustError::Decryption(format!("{e}")))
    }

    /// Return the key bytes (useful for session key export).
    pub fn key(&self) -> &[u8; 32] {
        &self.key
    }
}

/// AES-256-GCM authenticated encryption with associated data.
pub struct Aes256GcmCipher {
    key: [u8; 32],
}

impl Aes256GcmCipher {
    /// Create a new AES-256-GCM cipher from a 32-byte key.
    pub fn new(key: [u8; 32]) -> Self {
        Self { key }
    }

    /// Encrypt `plaintext` with the given 12-byte `nonce` and optional associated data `aad`.
    ///
    /// Returns ciphertext || 16-byte GCM tag.
    pub fn encrypt(&self, nonce: &[u8; 12], plaintext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        let cipher = Aes256Gcm::new_from_slice(&self.key)
            .map_err(|e| NexTrustError::Encryption(format!("aes-gcm init: {e}")))?;
        let nonce = AesNonce::from_slice(nonce);
        let payload = Payload { msg: plaintext, aad };
        cipher
            .encrypt(nonce, payload)
            .map_err(|e| NexTrustError::Encryption(format!("{e}")))
    }

    /// Decrypt `ciphertext` (which includes the appended 16-byte tag) with the given
    /// 12-byte `nonce` and the same `aad` used during encryption.
    pub fn decrypt(&self, nonce: &[u8; 12], ciphertext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        let cipher = Aes256Gcm::new_from_slice(&self.key)
            .map_err(|e| NexTrustError::Decryption(format!("aes-gcm init: {e}")))?;
        let nonce = AesNonce::from_slice(nonce);
        let payload = Payload {
            msg: ciphertext,
            aad,
        };
        cipher
            .decrypt(nonce, payload)
            .map_err(|e| NexTrustError::Decryption(format!("{e}")))
    }

    /// Return the key bytes.
    pub fn key(&self) -> &[u8; 32] {
        &self.key
    }
}

/// Unified AEAD key that dispatches between the two supported cipher suites.
pub enum AeadKey {
    ChaCha20Poly1305(AeadCipher),
    Aes256Gcm(Aes256GcmCipher),
}

impl AeadKey {
    /// Construct from a 32-byte key and the desired cipher suite.
    pub fn new(suite: CipherSuite, key: [u8; 32]) -> Self {
        match suite {
            CipherSuite::ChaCha20Poly1305 => AeadKey::ChaCha20Poly1305(AeadCipher::new(key)),
            CipherSuite::Aes256Gcm => AeadKey::Aes256Gcm(Aes256GcmCipher::new(key)),
        }
    }

    pub fn encrypt(&self, nonce: &[u8; 12], plaintext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        match self {
            AeadKey::ChaCha20Poly1305(c) => c.encrypt(nonce, plaintext, aad),
            AeadKey::Aes256Gcm(c) => c.encrypt(nonce, plaintext, aad),
        }
    }

    pub fn decrypt(&self, nonce: &[u8; 12], ciphertext: &[u8], aad: &[u8]) -> Result<Vec<u8>> {
        match self {
            AeadKey::ChaCha20Poly1305(c) => c.decrypt(nonce, ciphertext, aad),
            AeadKey::Aes256Gcm(c) => c.decrypt(nonce, ciphertext, aad),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- ChaCha20-Poly1305 tests ---

    #[test]
    fn chacha_roundtrip_no_aad() {
        let key = [0x42u8; 32];
        let nonce = [0u8; 12];
        let cipher = AeadCipher::new(key);
        let plaintext = b"hello nextrust aead";
        let ct = cipher.encrypt(&nonce, plaintext, b"").unwrap();
        let pt = cipher.decrypt(&nonce, &ct, b"").unwrap();
        assert_eq!(&pt, plaintext);
    }

    #[test]
    fn chacha_roundtrip_with_aad() {
        let key = [0x99u8; 32];
        let nonce = [1u8; 12];
        let cipher = AeadCipher::new(key);
        let plaintext = b"secret payload";
        let aad = b"additional data";
        let ct = cipher.encrypt(&nonce, plaintext, aad).unwrap();
        let pt = cipher.decrypt(&nonce, &ct, aad).unwrap();
        assert_eq!(&pt, plaintext);
    }

    #[test]
    fn chacha_wrong_aad_fails() {
        let key = [0xAAu8; 32];
        let nonce = [2u8; 12];
        let cipher = AeadCipher::new(key);
        let ct = cipher.encrypt(&nonce, b"data", b"good aad").unwrap();
        let result = cipher.decrypt(&nonce, &ct, b"bad aad");
        assert!(result.is_err());
    }

    #[test]
    fn chacha_tampered_ciphertext_fails() {
        let key = [0xBBu8; 32];
        let nonce = [3u8; 12];
        let cipher = AeadCipher::new(key);
        let mut ct = cipher.encrypt(&nonce, b"data", b"").unwrap();
        ct[0] ^= 0xFF; // flip a byte
        let result = cipher.decrypt(&nonce, &ct, b"");
        assert!(result.is_err());
    }

    // --- AES-256-GCM tests ---

    #[test]
    fn aes_gcm_roundtrip_no_aad() {
        let key = [0x42u8; 32];
        let nonce = [0u8; 12];
        let cipher = Aes256GcmCipher::new(key);
        let plaintext = b"hello aes-256-gcm";
        let ct = cipher.encrypt(&nonce, plaintext, b"").unwrap();
        let pt = cipher.decrypt(&nonce, &ct, b"").unwrap();
        assert_eq!(&pt, plaintext);
    }

    #[test]
    fn aes_gcm_roundtrip_with_aad() {
        let key = [0x99u8; 32];
        let nonce = [1u8; 12];
        let cipher = Aes256GcmCipher::new(key);
        let plaintext = b"aes gcm secret";
        let aad = b"auth data";
        let ct = cipher.encrypt(&nonce, plaintext, aad).unwrap();
        let pt = cipher.decrypt(&nonce, &ct, aad).unwrap();
        assert_eq!(&pt, plaintext);
    }

    #[test]
    fn aes_gcm_wrong_aad_fails() {
        let key = [0xAAu8; 32];
        let nonce = [2u8; 12];
        let cipher = Aes256GcmCipher::new(key);
        let ct = cipher.encrypt(&nonce, b"data", b"good aad").unwrap();
        let result = cipher.decrypt(&nonce, &ct, b"bad aad");
        assert!(result.is_err());
    }

    #[test]
    fn aes_gcm_wrong_key_fails() {
        let key1 = [0x11u8; 32];
        let key2 = [0x22u8; 32];
        let nonce = [0u8; 12];
        let enc = Aes256GcmCipher::new(key1);
        let dec = Aes256GcmCipher::new(key2);
        let ct = enc.encrypt(&nonce, b"secret", b"").unwrap();
        assert!(dec.decrypt(&nonce, &ct, b"").is_err());
    }

    #[test]
    fn aes_gcm_tampered_ciphertext_fails() {
        let key = [0xBBu8; 32];
        let nonce = [3u8; 12];
        let cipher = Aes256GcmCipher::new(key);
        let mut ct = cipher.encrypt(&nonce, b"data", b"").unwrap();
        ct[0] ^= 0xFF;
        assert!(cipher.decrypt(&nonce, &ct, b"").is_err());
    }

    // --- AeadKey dispatch tests ---

    #[test]
    fn aead_key_dispatches_chacha() {
        let key = [0x55u8; 32];
        let nonce = [0u8; 12];
        let ak = AeadKey::new(CipherSuite::ChaCha20Poly1305, key);
        let ct = ak.encrypt(&nonce, b"msg", b"").unwrap();
        let pt = ak.decrypt(&nonce, &ct, b"").unwrap();
        assert_eq!(pt, b"msg");
    }

    #[test]
    fn aead_key_dispatches_aes_gcm() {
        let key = [0x66u8; 32];
        let nonce = [0u8; 12];
        let ak = AeadKey::new(CipherSuite::Aes256Gcm, key);
        let ct = ak.encrypt(&nonce, b"msg", b"").unwrap();
        let pt = ak.decrypt(&nonce, &ct, b"").unwrap();
        assert_eq!(pt, b"msg");
    }

    #[test]
    fn cipher_suite_wire_ids() {
        assert_eq!(CipherSuite::Aes256Gcm.wire_id(), 0x0001);
        assert_eq!(CipherSuite::ChaCha20Poly1305.wire_id(), 0x0002);
        assert_eq!(CipherSuite::from_wire_id(0x0001), Some(CipherSuite::Aes256Gcm));
        assert_eq!(CipherSuite::from_wire_id(0x0002), Some(CipherSuite::ChaCha20Poly1305));
        assert_eq!(CipherSuite::from_wire_id(0x9999), None);
    }
}
