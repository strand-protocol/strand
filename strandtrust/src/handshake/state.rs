// Handshake state machine.

use crate::mic::MIC;

/// The current state of a StrandTrust handshake.
#[derive(Debug)]
pub enum HandshakeState {
    /// No handshake in progress.
    Idle,

    /// Initiator has sent TRUST_HELLO, waiting for TRUST_ACCEPT.
    InitSent {
        /// The initiator's ephemeral X25519 secret bytes (kept for DH).
        ephemeral_secret: [u8; 32],
        /// The initiator's ephemeral X25519 public bytes (sent in TRUST_HELLO).
        ephemeral_public: [u8; 32],
    },

    /// Initiator has received TRUST_ACCEPT with session key and peer MIC.
    ResponseReceived {
        /// Derived session key (client_write_key).
        session_key: [u8; 32],
        /// The responder's MIC.
        peer_mic: MIC,
        /// Server's write key (for decrypting server messages).
        server_write_key: [u8; 32],
    },

    /// Handshake is complete — both sides authenticated.
    Complete {
        /// Symmetric session key for encrypting outbound data.
        session_key: [u8; 32],
        /// The peer's MIC.
        peer_mic: MIC,
        /// Our own MIC (sent to the peer).
        my_mic: MIC,
        /// Server's write key.
        server_write_key: [u8; 32],
    },
}

impl HandshakeState {
    /// Human-readable label for the current state (used in error messages).
    pub fn label(&self) -> &'static str {
        match self {
            HandshakeState::Idle => "Idle",
            HandshakeState::InitSent { .. } => "InitSent",
            HandshakeState::ResponseReceived { .. } => "ResponseReceived",
            HandshakeState::Complete { .. } => "Complete",
        }
    }
}
