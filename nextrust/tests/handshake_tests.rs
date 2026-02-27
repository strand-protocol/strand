// Integration tests for the NexTrust 3-way handshake protocol.

use nextrust::crypto::keys::IdentityKeyPair;
use nextrust::handshake::protocol::{Initiator, Responder};
use nextrust::mic::builder::MICBuilder;
use nextrust::mic::Capability;

/// Helper: create an identity keypair and a self-signed MIC for testing.
fn make_identity_and_mic(caps: Vec<Capability>) -> (IdentityKeyPair, nextrust::mic::MIC) {
    let kp = IdentityKeyPair::generate();
    let mut builder = MICBuilder::new(&kp)
        .model_hash([0xEE; 32])
        .validity(1000, 9999999);
    for cap in caps {
        builder = builder.add_capability(cap);
    }
    let mic = builder.build().unwrap();
    (kp, mic)
}

// ── Full 3-way handshake ─────────────────────────────────────────────────

#[test]
fn full_handshake_succeeds() {
    let (client_kp, client_mic) =
        make_identity_and_mic(vec![Capability::TextGeneration, Capability::CodeGeneration]);
    let (server_kp, server_mic) =
        make_identity_and_mic(vec![Capability::TextGeneration, Capability::ToolUse]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let now = 5000u64;

    // Step 1: Client creates TRUST_HELLO
    let init_msg = initiator.create_init().unwrap();

    // Step 2: Server processes TRUST_HELLO, returns TRUST_ACCEPT
    let response_msg = responder.process_init(init_msg, now).unwrap();

    // Step 3: Client processes TRUST_ACCEPT, returns TRUST_FINISH
    let complete_msg = initiator.process_response(response_msg, now).unwrap();

    // Step 4: Server processes TRUST_FINISH
    responder.process_complete(complete_msg).unwrap();

    // Both sides should now be in the Complete state.
    let (client_session_key, peer_mic_from_client) = initiator.completed_state().unwrap();
    let (server_session_key, peer_mic_from_server) = responder.completed_state().unwrap();

    // The client's "peer MIC" should be the server's MIC and vice versa.
    assert_eq!(peer_mic_from_client.model_hash, [0xEE; 32]);
    assert_eq!(peer_mic_from_server.model_hash, [0xEE; 32]);

    // Session keys should be non-zero.
    assert_ne!(client_session_key, &[0u8; 32]);
    assert_ne!(server_session_key, &[0u8; 32]);
}

#[test]
fn handshake_with_different_capabilities() {
    let (client_kp, client_mic) =
        make_identity_and_mic(vec![Capability::Embedding]);
    let (server_kp, server_mic) =
        make_identity_and_mic(vec![Capability::Rag, Capability::Custom("search".into())]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let now = 5000u64;

    let init_msg = initiator.create_init().unwrap();
    let response_msg = responder.process_init(init_msg, now).unwrap();
    let complete_msg = initiator.process_response(response_msg, now).unwrap();
    responder.process_complete(complete_msg).unwrap();

    assert!(initiator.completed_state().is_some());
    assert!(responder.completed_state().is_some());
}

// ── Error cases ──────────────────────────────────────────────────────────

#[test]
fn create_init_twice_fails() {
    let (kp, mic) = make_identity_and_mic(vec![]);
    let mut initiator = Initiator::new(kp, mic);
    initiator.create_init().unwrap();
    assert!(initiator.create_init().is_err());
}

#[test]
fn responder_process_init_twice_fails() {
    let (client_kp, client_mic) = make_identity_and_mic(vec![]);
    let (server_kp, server_mic) = make_identity_and_mic(vec![]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let init_msg = initiator.create_init().unwrap();
    let _response_msg = responder.process_init(init_msg.clone(), 5000).unwrap();

    // Trying to process_init again should fail (responder is no longer Idle)
    assert!(responder.process_init(init_msg, 5000).is_err());
}

#[test]
fn expired_mic_rejected_during_handshake() {
    let (client_kp, client_mic) = make_identity_and_mic(vec![]);
    let (server_kp, server_mic) = make_identity_and_mic(vec![]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let init_msg = initiator.create_init().unwrap();

    // Use a timestamp far in the future so the MIC (valid_until=9999999) is expired
    let far_future = 99999999u64;
    assert!(responder.process_init(init_msg, far_future).is_err());
}

#[test]
fn tampered_response_payload_rejected() {
    let (client_kp, client_mic) = make_identity_and_mic(vec![]);
    let (server_kp, server_mic) = make_identity_and_mic(vec![]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let now = 5000u64;

    let init_msg = initiator.create_init().unwrap();
    let mut response_msg = responder.process_init(init_msg, now).unwrap();

    // Tamper with the encrypted payload
    if !response_msg.encrypted_payload.is_empty() {
        response_msg.encrypted_payload[0] ^= 0xFF;
    }

    // Client should reject the tampered response
    assert!(initiator.process_response(response_msg, now).is_err());
}

#[test]
fn tampered_complete_payload_rejected() {
    let (client_kp, client_mic) = make_identity_and_mic(vec![]);
    let (server_kp, server_mic) = make_identity_and_mic(vec![]);

    let mut initiator = Initiator::new(client_kp, client_mic);
    let mut responder = Responder::new(server_kp, server_mic);

    let now = 5000u64;

    let init_msg = initiator.create_init().unwrap();
    let response_msg = responder.process_init(init_msg, now).unwrap();
    let mut complete_msg = initiator.process_response(response_msg, now).unwrap();

    // Tamper with the client's finished payload
    if !complete_msg.encrypted_payload.is_empty() {
        complete_msg.encrypted_payload[0] ^= 0xFF;
    }

    // Server should reject the tampered complete
    assert!(responder.process_complete(complete_msg).is_err());
}
