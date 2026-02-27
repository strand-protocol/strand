// Handshake messages exchanged during the StrandTrust 1-RTT protocol.

use crate::mic::MIC;

/// Message 1: Initiator -> Responder (TRUST_HELLO).
#[derive(Debug, Clone)]
pub struct HandshakeInit {
    /// Initiator's ephemeral X25519 public key (32 bytes).
    pub ephemeral_pub: [u8; 32],
    /// Initiator's MIC (proving identity and capabilities).
    pub initiator_mic: MIC,
}

/// Message 2: Responder -> Initiator (TRUST_ACCEPT).
#[derive(Debug, Clone)]
pub struct HandshakeResponse {
    /// Responder's ephemeral X25519 public key (32 bytes).
    pub ephemeral_pub: [u8; 32],
    /// Responder's MIC.
    pub responder_mic: MIC,
    /// Encrypted payload (e.g., server_finished confirmation, encrypted with
    /// the server_write_key derived from the DH shared secret).
    pub encrypted_payload: Vec<u8>,
}

/// Message 3: Initiator -> Responder (TRUST_FINISH).
#[derive(Debug, Clone)]
pub struct HandshakeComplete {
    /// Encrypted payload (e.g., client_finished confirmation, encrypted with
    /// the client_write_key derived from the DH shared secret).
    pub encrypted_payload: Vec<u8>,
}
