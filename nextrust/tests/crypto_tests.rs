// Integration tests for the NexTrust crypto layer.

use nextrust::crypto::aead::AeadCipher;
use nextrust::crypto::hash::{hash_blake3, hash_sha256};
use nextrust::crypto::keys::{verify_signature, IdentityKeyPair};
use nextrust::crypto::x25519::{derive_session_keys, X25519KeyPair};

// ── Ed25519 key generation and signing ───────────────────────────────────

#[test]
fn keypair_generate_unique() {
    let kp1 = IdentityKeyPair::generate();
    let kp2 = IdentityKeyPair::generate();
    assert_ne!(kp1.public_key_bytes(), kp2.public_key_bytes());
}

#[test]
fn keypair_seed_roundtrip() {
    let kp = IdentityKeyPair::generate();
    let seed = kp.secret_key_bytes();
    let kp2 = IdentityKeyPair::from_seed(&seed);
    assert_eq!(kp.public_key_bytes(), kp2.public_key_bytes());
    assert_eq!(kp.node_id(), kp2.node_id());
}

#[test]
fn sign_and_verify() {
    let kp = IdentityKeyPair::generate();
    let msg = b"NexTrust integration test message";
    let sig = kp.sign(msg);
    kp.verify(msg, &sig).expect("valid signature");
}

#[test]
fn verify_with_raw_pubkey() {
    let kp = IdentityKeyPair::generate();
    let msg = b"verify_signature standalone";
    let sig = kp.sign(msg);
    let pk = kp.public_key_bytes();
    verify_signature(&pk, msg, &sig).expect("valid signature via standalone fn");
}

#[test]
fn wrong_message_rejects() {
    let kp = IdentityKeyPair::generate();
    let sig = kp.sign(b"correct");
    assert!(kp.verify(b"wrong", &sig).is_err());
}

#[test]
fn wrong_key_rejects() {
    let kp1 = IdentityKeyPair::generate();
    let kp2 = IdentityKeyPair::generate();
    let msg = b"cross-key test";
    let sig = kp1.sign(msg);
    assert!(kp2.verify(msg, &sig).is_err());
}

// ── X25519 Diffie-Hellman ────────────────────────────────────────────────

#[test]
fn dh_shared_secret_symmetric() {
    let alice = X25519KeyPair::generate();
    let bob = X25519KeyPair::generate();
    let alice_shared = alice.diffie_hellman(&bob.public_key_bytes());
    let bob_shared = bob.diffie_hellman(&alice.public_key_bytes());
    assert_eq!(alice_shared, bob_shared);
}

#[test]
fn dh_different_peers_differ() {
    let alice = X25519KeyPair::generate();
    let bob = X25519KeyPair::generate();
    let carol = X25519KeyPair::generate();
    let ab = alice.diffie_hellman(&bob.public_key_bytes());
    let ac = alice.diffie_hellman(&carol.public_key_bytes());
    assert_ne!(ab, ac);
}

#[test]
fn session_key_derivation_deterministic() {
    let alice = X25519KeyPair::generate();
    let bob = X25519KeyPair::generate();
    let shared = alice.diffie_hellman(&bob.public_key_bytes());
    let cid = [1u8; 16];
    let sid = [2u8; 16];
    let k1 = derive_session_keys(&shared, &cid, &sid).unwrap();
    let k2 = derive_session_keys(&shared, &cid, &sid).unwrap();
    assert_eq!(k1.client_write_key, k2.client_write_key);
    assert_eq!(k1.server_write_key, k2.server_write_key);
}

#[test]
fn session_keys_differ_by_role() {
    let alice = X25519KeyPair::generate();
    let bob = X25519KeyPair::generate();
    let shared = alice.diffie_hellman(&bob.public_key_bytes());
    let keys = derive_session_keys(&shared, &[1u8; 16], &[2u8; 16]).unwrap();
    assert_ne!(keys.client_write_key, keys.server_write_key);
    assert_ne!(keys.client_write_iv, keys.server_write_iv);
}

// ── AEAD roundtrip ───────────────────────────────────────────────────────

#[test]
fn aead_roundtrip_empty_aad() {
    let key = [0x42u8; 32];
    let nonce = [0u8; 12];
    let cipher = AeadCipher::new(key);
    let pt = b"The quick brown fox";
    let ct = cipher.encrypt(&nonce, pt, b"").unwrap();
    let recovered = cipher.decrypt(&nonce, &ct, b"").unwrap();
    assert_eq!(&recovered, pt);
}

#[test]
fn aead_roundtrip_with_aad() {
    let key = [0x77u8; 32];
    let nonce = [0xAA; 12];
    let cipher = AeadCipher::new(key);
    let pt = b"authenticated payload";
    let aad = b"channel metadata";
    let ct = cipher.encrypt(&nonce, pt, aad).unwrap();
    let recovered = cipher.decrypt(&nonce, &ct, aad).unwrap();
    assert_eq!(&recovered, pt);
}

#[test]
fn aead_wrong_key_fails() {
    let cipher1 = AeadCipher::new([0x11u8; 32]);
    let cipher2 = AeadCipher::new([0x22u8; 32]);
    let nonce = [0u8; 12];
    let ct = cipher1.encrypt(&nonce, b"secret", b"").unwrap();
    assert!(cipher2.decrypt(&nonce, &ct, b"").is_err());
}

#[test]
fn aead_wrong_nonce_fails() {
    let cipher = AeadCipher::new([0x33u8; 32]);
    let n1 = [0u8; 12];
    let n2 = [1u8; 12];
    let ct = cipher.encrypt(&n1, b"data", b"").unwrap();
    assert!(cipher.decrypt(&n2, &ct, b"").is_err());
}

#[test]
fn aead_tampered_ciphertext_fails() {
    let cipher = AeadCipher::new([0x44u8; 32]);
    let nonce = [0u8; 12];
    let mut ct = cipher.encrypt(&nonce, b"data", b"").unwrap();
    ct[0] ^= 0xFF;
    assert!(cipher.decrypt(&nonce, &ct, b"").is_err());
}

// ── Hashing ──────────────────────────────────────────────────────────────

#[test]
fn sha256_known_empty() {
    let h = hash_sha256(b"");
    let hex: String = h.iter().map(|b| format!("{b:02x}")).collect();
    assert_eq!(hex, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
}

#[test]
fn blake3_deterministic() {
    assert_eq!(hash_blake3(b"test"), hash_blake3(b"test"));
}

#[test]
fn blake3_different_inputs() {
    assert_ne!(hash_blake3(b"a"), hash_blake3(b"b"));
}
