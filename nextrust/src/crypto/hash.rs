// Cryptographic hashing: SHA-256 and BLAKE3.

use sha2::{Digest, Sha256};

/// SHA-256 hash of `data`, returning a 32-byte digest.
pub fn hash_sha256(data: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(data);
    let result = hasher.finalize();
    let mut out = [0u8; 32];
    out.copy_from_slice(&result);
    out
}

/// BLAKE3 hash of `data`, returning a 32-byte digest.
pub fn hash_blake3(data: &[u8]) -> [u8; 32] {
    *blake3::hash(data).as_bytes()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sha256_known_vector() {
        // SHA-256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
        let hash = hash_sha256(b"");
        assert_eq!(
            hex(&hash),
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        );
    }

    #[test]
    fn blake3_deterministic() {
        let h1 = hash_blake3(b"hello");
        let h2 = hash_blake3(b"hello");
        assert_eq!(h1, h2);
    }

    #[test]
    fn blake3_different_input() {
        let h1 = hash_blake3(b"hello");
        let h2 = hash_blake3(b"world");
        assert_ne!(h1, h2);
    }

    fn hex(bytes: &[u8]) -> String {
        bytes.iter().map(|b| format!("{b:02x}")).collect()
    }
}
