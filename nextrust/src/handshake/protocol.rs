// NexTrust handshake protocol: 3-message exchange with X25519 DH + HKDF session keys.
//
//   Initiator                         Responder
//     |--- HandshakeInit ------->|
//     |<-- HandshakeResponse ----|
//     |--- HandshakeComplete --->|
//     |==== encrypted channel ===|

use crate::crypto::aead::AeadCipher;
use crate::crypto::keys::{derive_node_id, IdentityKeyPair};
use crate::crypto::x25519::{derive_session_keys, X25519KeyPair};
use crate::error::{NexTrustError, Result};
use crate::handshake::messages::{HandshakeComplete, HandshakeInit, HandshakeResponse};
use crate::handshake::state::HandshakeState;
use crate::mic::validator::validate;
use crate::mic::MIC;

/// Derive the 16-byte node ID from a MIC's node_id field (which holds the 32-byte public key).
/// This matches the IdentityKeyPair::node_id() derivation: first 16 bytes of SHA-256(pubkey).
fn node_id_from_mic(mic: &MIC) -> [u8; 16] {
    use ed25519_dalek::VerifyingKey;
    // mic.node_id stores the raw 32-byte Ed25519 public key
    if let Ok(vk) = VerifyingKey::from_bytes(&mic.node_id) {
        derive_node_id(&vk)
    } else {
        // Fallback: just truncate (shouldn't happen with valid MICs)
        let mut id = [0u8; 16];
        id.copy_from_slice(&mic.node_id[..16]);
        id
    }
}

/// A fixed nonce used for the handshake confirmation messages.
/// In production, these would be derived uniquely; here we use a simple scheme:
/// message 2 uses nonce [0,0,...,1], message 3 uses nonce [0,0,...,2].
fn handshake_nonce(msg_num: u8) -> [u8; 12] {
    let mut n = [0u8; 12];
    n[11] = msg_num;
    n
}

/// The confirmation message encrypted inside TRUST_ACCEPT and TRUST_FINISH.
const FINISHED_MSG: &[u8] = b"nexus handshake finished";

// ── Initiator ────────────────────────────────────────────────────────────

/// Client-side (initiator) of the NexTrust handshake.
pub struct Initiator {
    #[allow(dead_code)]
    identity: IdentityKeyPair,
    mic: MIC,
    state: HandshakeState,
}

impl Initiator {
    /// Create a new initiator with the given identity and MIC.
    pub fn new(identity: IdentityKeyPair, mic: MIC) -> Self {
        Self {
            identity,
            mic,
            state: HandshakeState::Idle,
        }
    }

    /// Step 1: Generate TRUST_HELLO (HandshakeInit).
    pub fn create_init(&mut self) -> Result<HandshakeInit> {
        if !matches!(self.state, HandshakeState::Idle) {
            return Err(NexTrustError::InvalidStateTransition {
                from: self.state.label().into(),
                to: "InitSent".into(),
            });
        }

        // We need to keep the secret bytes so we can perform DH when we get the response.
        // X25519KeyPair uses StaticSecret under the hood; extract secret via from_secret_bytes
        // round-trip is not possible with EphemeralSecret, so we use StaticSecret which allows it.
        // We store the full keypair's public bytes and reconstruct from secret in process_response.
        // Actually, let's store the raw secret bytes by generating from known bytes.
        let secret_bytes = {
            // Generate 32 random bytes, build keypair from them so we can store the secret
            use rand::RngCore;
            let mut secret = [0u8; 32];
            rand::rngs::OsRng.fill_bytes(&mut secret);
            secret
        };
        let ephemeral = X25519KeyPair::from_secret_bytes(secret_bytes);
        let ephemeral_pub = ephemeral.public_key_bytes();

        self.state = HandshakeState::InitSent {
            ephemeral_secret: secret_bytes,
            ephemeral_public: ephemeral_pub,
        };

        Ok(HandshakeInit {
            ephemeral_pub,
            initiator_mic: self.mic.clone(),
        })
    }

    /// Step 2 (initiator side): Process TRUST_ACCEPT, produce TRUST_FINISH.
    pub fn process_response(
        &mut self,
        response: HandshakeResponse,
        now: u64,
    ) -> Result<HandshakeComplete> {
        let (ephemeral_secret, _ephemeral_public) = match &self.state {
            HandshakeState::InitSent {
                ephemeral_secret,
                ephemeral_public,
            } => (*ephemeral_secret, *ephemeral_public),
            _ => {
                return Err(NexTrustError::InvalidStateTransition {
                    from: self.state.label().into(),
                    to: "ResponseReceived".into(),
                });
            }
        };

        // Validate responder's MIC
        validate(&response.responder_mic, now)?;

        // Perform DH
        let our_ephemeral = X25519KeyPair::from_secret_bytes(ephemeral_secret);
        let shared_secret = our_ephemeral.diffie_hellman(&response.ephemeral_pub);

        // Derive session keys (we are the client)
        let client_node_id = node_id_from_mic(&self.mic);
        let server_node_id = node_id_from_mic(&response.responder_mic);
        let session_keys = derive_session_keys(&shared_secret, &client_node_id, &server_node_id)?;

        // Verify the responder's encrypted payload (decrypt with server_write_key)
        let server_cipher = AeadCipher::new(session_keys.server_write_key);
        let decrypted = server_cipher.decrypt(
            &handshake_nonce(2),
            &response.encrypted_payload,
            b"",
        )?;
        if decrypted != FINISHED_MSG {
            return Err(NexTrustError::Handshake(
                "invalid server finished message".into(),
            ));
        }

        // Encrypt our own finished message with client_write_key
        let client_cipher = AeadCipher::new(session_keys.client_write_key);
        let encrypted_payload = client_cipher.encrypt(
            &handshake_nonce(3),
            FINISHED_MSG,
            b"",
        )?;

        self.state = HandshakeState::Complete {
            session_key: session_keys.client_write_key,
            peer_mic: response.responder_mic,
            my_mic: self.mic.clone(),
            server_write_key: session_keys.server_write_key,
        };

        Ok(HandshakeComplete { encrypted_payload })
    }

    /// Get the completed handshake state (session key and peer MIC).
    pub fn completed_state(&self) -> Option<(&[u8; 32], &MIC)> {
        match &self.state {
            HandshakeState::Complete {
                session_key,
                peer_mic,
                ..
            } => Some((session_key, peer_mic)),
            _ => None,
        }
    }
}

// ── Responder ────────────────────────────────────────────────────────────

/// Server-side (responder) of the NexTrust handshake.
pub struct Responder {
    #[allow(dead_code)]
    identity: IdentityKeyPair,
    mic: MIC,
    state: HandshakeState,
    /// Stored session keys after processing init
    session_keys_cache: Option<(/* client_write_key */ [u8; 32], /* server_write_key */ [u8; 32])>,
}

impl Responder {
    /// Create a new responder with the given identity and MIC.
    pub fn new(identity: IdentityKeyPair, mic: MIC) -> Self {
        Self {
            identity,
            mic,
            state: HandshakeState::Idle,
            session_keys_cache: None,
        }
    }

    /// Step 1 (responder side): Process TRUST_HELLO, produce TRUST_ACCEPT.
    pub fn process_init(
        &mut self,
        init: HandshakeInit,
        now: u64,
    ) -> Result<HandshakeResponse> {
        if !matches!(self.state, HandshakeState::Idle) {
            return Err(NexTrustError::InvalidStateTransition {
                from: self.state.label().into(),
                to: "ResponseSent".into(),
            });
        }

        // Validate initiator's MIC
        validate(&init.initiator_mic, now)?;

        // Generate ephemeral keypair
        let secret_bytes = {
            use rand::RngCore;
            let mut secret = [0u8; 32];
            rand::rngs::OsRng.fill_bytes(&mut secret);
            secret
        };
        let ephemeral = X25519KeyPair::from_secret_bytes(secret_bytes);
        let ephemeral_pub = ephemeral.public_key_bytes();

        // Perform DH
        let shared_secret = ephemeral.diffie_hellman(&init.ephemeral_pub);

        // Derive session keys (the initiator is the client, we are the server)
        let client_node_id = node_id_from_mic(&init.initiator_mic);
        let server_node_id = node_id_from_mic(&self.mic);
        let session_keys = derive_session_keys(&shared_secret, &client_node_id, &server_node_id)?;

        // Encrypt server finished message
        let server_cipher = AeadCipher::new(session_keys.server_write_key);
        let encrypted_payload = server_cipher.encrypt(
            &handshake_nonce(2),
            FINISHED_MSG,
            b"",
        )?;

        self.session_keys_cache = Some((session_keys.client_write_key, session_keys.server_write_key));

        self.state = HandshakeState::ResponseReceived {
            session_key: session_keys.server_write_key,
            peer_mic: init.initiator_mic,
            server_write_key: session_keys.server_write_key,
        };

        Ok(HandshakeResponse {
            ephemeral_pub,
            responder_mic: self.mic.clone(),
            encrypted_payload,
        })
    }

    /// Step 3 (responder side): Process TRUST_FINISH to complete the handshake.
    pub fn process_complete(&mut self, complete: HandshakeComplete) -> Result<()> {
        let peer_mic = match &self.state {
            HandshakeState::ResponseReceived { peer_mic, .. } => peer_mic.clone(),
            _ => {
                return Err(NexTrustError::InvalidStateTransition {
                    from: self.state.label().into(),
                    to: "Complete".into(),
                });
            }
        };

        let (client_write_key, server_write_key) = self
            .session_keys_cache
            .ok_or_else(|| NexTrustError::Handshake("no cached session keys".into()))?;

        // Decrypt and verify client finished message
        let client_cipher = AeadCipher::new(client_write_key);
        let decrypted = client_cipher.decrypt(
            &handshake_nonce(3),
            &complete.encrypted_payload,
            b"",
        )?;
        if decrypted != FINISHED_MSG {
            return Err(NexTrustError::Handshake(
                "invalid client finished message".into(),
            ));
        }

        self.state = HandshakeState::Complete {
            session_key: server_write_key,
            peer_mic,
            my_mic: self.mic.clone(),
            server_write_key,
        };

        Ok(())
    }

    /// Get the completed handshake state (session key and peer MIC).
    pub fn completed_state(&self) -> Option<(&[u8; 32], &MIC)> {
        match &self.state {
            HandshakeState::Complete {
                session_key,
                peer_mic,
                ..
            } => Some((session_key, peer_mic)),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::mic::builder::MICBuilder;
    use crate::mic::Capability;

    fn make_identity_and_mic() -> (IdentityKeyPair, MIC) {
        let kp = IdentityKeyPair::generate();
        let mic = MICBuilder::new(&kp)
            .model_hash([0xDD; 32])
            .add_capability(Capability::TextGeneration)
            .validity(1000, 9999999)
            .build()
            .unwrap();
        (kp, mic)
    }

    #[test]
    fn full_handshake() {
        let (client_kp, client_mic) = make_identity_and_mic();
        let (server_kp, server_mic) = make_identity_and_mic();

        let mut initiator = Initiator::new(client_kp, client_mic);
        let mut responder = Responder::new(server_kp, server_mic);

        let now = 5000u64;

        // Step 1: client -> server
        let init_msg = initiator.create_init().unwrap();

        // Step 2: server processes init, returns response
        let response_msg = responder.process_init(init_msg, now).unwrap();

        // Step 3: client processes response, returns complete
        let complete_msg = initiator.process_response(response_msg, now).unwrap();

        // Step 4: server processes complete
        responder.process_complete(complete_msg).unwrap();

        // Both sides should be in Complete state
        assert!(initiator.completed_state().is_some());
        assert!(responder.completed_state().is_some());
    }
}
