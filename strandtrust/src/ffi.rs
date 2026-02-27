// C FFI bindings for NexTrust core operations.
//
// All functions return 0 on success, -1 on error.
// Buffers are caller-allocated; lengths are checked.

use std::slice;

use crate::crypto::aead::AeadCipher;
use crate::crypto::keys::IdentityKeyPair;
use crate::mic::builder::MICBuilder;
use crate::mic::serializer;
use crate::mic::validator;

// ── Keypair generation ───────────────────────────────────────────────────

/// Generate a new Ed25519 keypair.
///
/// `public_key_out`: pointer to 32-byte buffer for the public key.
/// `secret_key_out`: pointer to 32-byte buffer for the secret key seed.
///
/// Returns 0 on success.
#[no_mangle]
pub unsafe extern "C" fn nextrust_keypair_generate(
    public_key_out: *mut u8,
    secret_key_out: *mut u8,
) -> i32 {
    if public_key_out.is_null() || secret_key_out.is_null() {
        return -1;
    }
    let kp = IdentityKeyPair::generate();
    let pk = kp.public_key_bytes();
    let sk = kp.secret_key_bytes();
    unsafe {
        std::ptr::copy_nonoverlapping(pk.as_ptr(), public_key_out, 32);
        std::ptr::copy_nonoverlapping(sk.as_ptr(), secret_key_out, 32);
    }
    0
}

// ── MIC creation ─────────────────────────────────────────────────────────

/// Create a self-signed MIC and write the serialized bytes to `mic_out`.
///
/// `secret_key`: pointer to 32-byte Ed25519 secret key seed.
/// `model_hash`: pointer to 32-byte model hash.
/// `valid_from`: start of validity (unix seconds).
/// `valid_until`: end of validity (unix seconds).
/// `mic_out`: pointer to buffer for serialized MIC.
/// `mic_out_len`: pointer to usize — on input the buffer capacity, on output the actual length.
///
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn nextrust_mic_create(
    secret_key: *const u8,
    model_hash: *const u8,
    valid_from: u64,
    valid_until: u64,
    mic_out: *mut u8,
    mic_out_len: *mut usize,
) -> i32 {
    if secret_key.is_null() || model_hash.is_null() || mic_out.is_null() || mic_out_len.is_null() {
        return -1;
    }

    let sk = unsafe { slice::from_raw_parts(secret_key, 32) };
    let mh = unsafe { slice::from_raw_parts(model_hash, 32) };

    let mut sk_arr = [0u8; 32];
    sk_arr.copy_from_slice(sk);
    let mut mh_arr = [0u8; 32];
    mh_arr.copy_from_slice(mh);

    let kp = IdentityKeyPair::from_seed(&sk_arr);
    let mic = match MICBuilder::new(&kp)
        .model_hash(mh_arr)
        .validity(valid_from, valid_until)
        .build()
    {
        Ok(m) => m,
        Err(_) => return -1,
    };

    let bytes = serializer::serialize(&mic);
    let cap = unsafe { *mic_out_len };
    if bytes.len() > cap {
        return -1;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr(), mic_out, bytes.len());
        *mic_out_len = bytes.len();
    }
    0
}

// ── MIC verification ─────────────────────────────────────────────────────

/// Verify a serialized MIC.
///
/// `mic_data`: pointer to serialized MIC bytes.
/// `mic_len`: length of MIC data.
/// `now`: current unix timestamp for expiry check.
///
/// Returns 0 if valid, -1 if invalid.
#[no_mangle]
pub unsafe extern "C" fn nextrust_mic_verify(
    mic_data: *const u8,
    mic_len: usize,
    now: u64,
) -> i32 {
    if mic_data.is_null() {
        return -1;
    }
    let data = unsafe { slice::from_raw_parts(mic_data, mic_len) };
    let mic = match serializer::deserialize(data) {
        Ok(m) => m,
        Err(_) => return -1,
    };
    match validator::validate(&mic, now) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

// ── Handshake init (simplified — returns ephemeral public key) ───────────

/// Generate an X25519 ephemeral keypair for handshake initiation.
///
/// `ephemeral_pub_out`: pointer to 32-byte buffer for ephemeral public key.
/// `ephemeral_secret_out`: pointer to 32-byte buffer for ephemeral secret.
///
/// Returns 0 on success.
#[no_mangle]
pub unsafe extern "C" fn nextrust_handshake_init(
    ephemeral_pub_out: *mut u8,
    ephemeral_secret_out: *mut u8,
) -> i32 {
    if ephemeral_pub_out.is_null() || ephemeral_secret_out.is_null() {
        return -1;
    }
    use crate::crypto::x25519::X25519KeyPair;
    use rand::RngCore;

    let mut secret_bytes = [0u8; 32];
    rand::rngs::OsRng.fill_bytes(&mut secret_bytes);
    let kp = X25519KeyPair::from_secret_bytes(secret_bytes);
    let pub_bytes = kp.public_key_bytes();

    unsafe {
        std::ptr::copy_nonoverlapping(pub_bytes.as_ptr(), ephemeral_pub_out, 32);
        std::ptr::copy_nonoverlapping(secret_bytes.as_ptr(), ephemeral_secret_out, 32);
    }
    0
}

// ── AEAD encrypt / decrypt ───────────────────────────────────────────────

/// Encrypt plaintext using ChaCha20-Poly1305.
///
/// `key`: pointer to 32-byte key.
/// `nonce`: pointer to 12-byte nonce.
/// `plaintext`: pointer to plaintext bytes.
/// `plaintext_len`: length of plaintext.
/// `aad`: pointer to AAD bytes (may be null if `aad_len` is 0).
/// `aad_len`: length of AAD.
/// `ciphertext_out`: pointer to output buffer (must hold plaintext_len + 16).
/// `ciphertext_out_len`: pointer to usize — on input the buffer capacity, on output actual length.
///
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn nextrust_encrypt(
    key: *const u8,
    nonce: *const u8,
    plaintext: *const u8,
    plaintext_len: usize,
    aad: *const u8,
    aad_len: usize,
    ciphertext_out: *mut u8,
    ciphertext_out_len: *mut usize,
) -> i32 {
    if key.is_null() || nonce.is_null() || plaintext.is_null() || ciphertext_out.is_null() || ciphertext_out_len.is_null() {
        return -1;
    }

    let key_slice = unsafe { slice::from_raw_parts(key, 32) };
    let nonce_slice = unsafe { slice::from_raw_parts(nonce, 12) };
    let pt = unsafe { slice::from_raw_parts(plaintext, plaintext_len) };
    let aad_slice = if aad.is_null() || aad_len == 0 {
        &[]
    } else {
        unsafe { slice::from_raw_parts(aad, aad_len) }
    };

    let mut key_arr = [0u8; 32];
    key_arr.copy_from_slice(key_slice);
    let mut nonce_arr = [0u8; 12];
    nonce_arr.copy_from_slice(nonce_slice);

    let cipher = AeadCipher::new(key_arr);
    let ct = match cipher.encrypt(&nonce_arr, pt, aad_slice) {
        Ok(c) => c,
        Err(_) => return -1,
    };

    let cap = unsafe { *ciphertext_out_len };
    if ct.len() > cap {
        return -1;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(ct.as_ptr(), ciphertext_out, ct.len());
        *ciphertext_out_len = ct.len();
    }
    0
}

/// Decrypt ciphertext using ChaCha20-Poly1305.
///
/// `key`: pointer to 32-byte key.
/// `nonce`: pointer to 12-byte nonce.
/// `ciphertext`: pointer to ciphertext bytes (includes 16-byte tag).
/// `ciphertext_len`: length of ciphertext.
/// `aad`: pointer to AAD bytes (may be null if `aad_len` is 0).
/// `aad_len`: length of AAD.
/// `plaintext_out`: pointer to output buffer (must hold ciphertext_len - 16).
/// `plaintext_out_len`: pointer to usize — on input the buffer capacity, on output actual length.
///
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn nextrust_decrypt(
    key: *const u8,
    nonce: *const u8,
    ciphertext: *const u8,
    ciphertext_len: usize,
    aad: *const u8,
    aad_len: usize,
    plaintext_out: *mut u8,
    plaintext_out_len: *mut usize,
) -> i32 {
    if key.is_null() || nonce.is_null() || ciphertext.is_null() || plaintext_out.is_null() || plaintext_out_len.is_null() {
        return -1;
    }

    let key_slice = unsafe { slice::from_raw_parts(key, 32) };
    let nonce_slice = unsafe { slice::from_raw_parts(nonce, 12) };
    let ct = unsafe { slice::from_raw_parts(ciphertext, ciphertext_len) };
    let aad_slice = if aad.is_null() || aad_len == 0 {
        &[]
    } else {
        unsafe { slice::from_raw_parts(aad, aad_len) }
    };

    let mut key_arr = [0u8; 32];
    key_arr.copy_from_slice(key_slice);
    let mut nonce_arr = [0u8; 12];
    nonce_arr.copy_from_slice(nonce_slice);

    let cipher = AeadCipher::new(key_arr);
    let pt = match cipher.decrypt(&nonce_arr, ct, aad_slice) {
        Ok(p) => p,
        Err(_) => return -1,
    };

    let cap = unsafe { *plaintext_out_len };
    if pt.len() > cap {
        return -1;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(pt.as_ptr(), plaintext_out, pt.len());
        *plaintext_out_len = pt.len();
    }
    0
}
