// ChaCha20-Poly1305 AEAD encrypt / decrypt (RFC 8439).

use chacha20poly1305::aead::{Aead, KeyInit, Payload};
use chacha20poly1305::{ChaCha20Poly1305, Nonce};

use crate::error::{NexTrustError, Result};

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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn roundtrip_no_aad() {
        let key = [0x42u8; 32];
        let nonce = [0u8; 12];
        let cipher = AeadCipher::new(key);
        let plaintext = b"hello nextrust aead";
        let ct = cipher.encrypt(&nonce, plaintext, b"").unwrap();
        let pt = cipher.decrypt(&nonce, &ct, b"").unwrap();
        assert_eq!(&pt, plaintext);
    }

    #[test]
    fn roundtrip_with_aad() {
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
    fn wrong_aad_fails() {
        let key = [0xAAu8; 32];
        let nonce = [2u8; 12];
        let cipher = AeadCipher::new(key);
        let ct = cipher.encrypt(&nonce, b"data", b"good aad").unwrap();
        let result = cipher.decrypt(&nonce, &ct, b"bad aad");
        assert!(result.is_err());
    }

    #[test]
    fn tampered_ciphertext_fails() {
        let key = [0xBBu8; 32];
        let nonce = [3u8; 12];
        let cipher = AeadCipher::new(key);
        let mut ct = cipher.encrypt(&nonce, b"data", b"").unwrap();
        ct[0] ^= 0xFF; // flip a byte
        let result = cipher.decrypt(&nonce, &ct, b"");
        assert!(result.is_err());
    }
}
