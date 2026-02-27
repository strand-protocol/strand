// Integration tests for the MIC (Model Identity Certificate) subsystem.

use std::time::Duration;

use strandtrust::crypto::keys::IdentityKeyPair;
use strandtrust::mic::builder::MICBuilder;
use strandtrust::mic::serializer::{deserialize, serialize};
use strandtrust::mic::validator::{validate, validate_chain};
use strandtrust::mic::{Capability, Provenance, MIC};

// ── Builder ──────────────────────────────────────────────────────────────

#[test]
fn build_minimal_mic() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0xAA; 32])
        .validity(1000, 2000)
        .build()
        .unwrap();
    assert_eq!(mic.model_hash, [0xAA; 32]);
    assert_eq!(mic.valid_from, 1000);
    assert_eq!(mic.valid_until, 2000);
    assert!(mic.capabilities.is_empty());
    assert!(mic.training_provenance.is_none());
    assert_eq!(mic.node_id, kp.public_key_bytes());
    assert_eq!(mic.issuer_public_key, kp.public_key_bytes());
}

#[test]
fn build_with_capabilities_and_provenance() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0xBB; 32])
        .add_capability(Capability::TextGeneration)
        .add_capability(Capability::CodeGeneration)
        .add_capability(Capability::Custom("multi-modal".into()))
        .training_provenance(Provenance {
            description: "SFT on code dataset".into(),
            dataset_hash: [0xCC; 32],
            timestamp: 1700000000,
        })
        .validity(1000, 9000)
        .build()
        .unwrap();

    assert_eq!(mic.capabilities.len(), 3);
    assert!(mic.training_provenance.is_some());
}

#[test]
fn build_with_valid_for_from() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0x00; 32])
        .valid_for_from(5000, Duration::from_secs(3600))
        .build()
        .unwrap();
    assert_eq!(mic.valid_from, 5000);
    assert_eq!(mic.valid_until, 5000 + 3600);
}

#[test]
fn build_missing_model_hash() {
    let kp = IdentityKeyPair::generate();
    assert!(MICBuilder::new(&kp).validity(1, 2).build().is_err());
}

#[test]
fn build_missing_validity() {
    let kp = IdentityKeyPair::generate();
    assert!(MICBuilder::new(&kp).model_hash([0; 32]).build().is_err());
}

#[test]
fn build_invalid_validity_window() {
    let kp = IdentityKeyPair::generate();
    assert!(MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(2000, 1000)
        .build()
        .is_err());
}

// ── Serializer ───────────────────────────────────────────────────────────

#[test]
fn serialize_deserialize_roundtrip_minimal() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0x11; 32])
        .validity(100, 200)
        .build()
        .unwrap();
    let bytes = serialize(&mic);
    let mic2 = deserialize(&bytes).unwrap();
    assert_eq!(mic, mic2);
}

#[test]
fn serialize_deserialize_roundtrip_full() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0x22; 32])
        .add_capability(Capability::TextGeneration)
        .add_capability(Capability::ToolUse)
        .add_capability(Capability::Custom("reasoning-v2".into()))
        .training_provenance(Provenance {
            description: "RLHF fine-tune round 3".into(),
            dataset_hash: [0x44; 32],
            timestamp: 1700000000,
        })
        .validity(1000, 99999)
        .build()
        .unwrap();
    let bytes = serialize(&mic);
    let mic2 = deserialize(&bytes).unwrap();
    assert_eq!(mic, mic2);
}

#[test]
fn deserialize_bad_version() {
    let mut data = vec![0xFF]; // bad version
    data.extend_from_slice(&[0u8; 256]);
    assert!(deserialize(&data).is_err());
}

#[test]
fn deserialize_truncated() {
    assert!(deserialize(&[MIC::VERSION, 0, 0]).is_err());
}

#[test]
fn serialize_version_byte() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(1, 2)
        .build()
        .unwrap();
    let bytes = serialize(&mic);
    assert_eq!(bytes[0], MIC::VERSION);
}

// ── Validator ────────────────────────────────────────────────────────────

#[test]
fn validate_valid_mic() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0xAA; 32])
        .validity(1000, 5000)
        .build()
        .unwrap();
    let result = validate(&mic, 2000).unwrap();
    assert!(result.signature_valid);
    assert!(result.time_valid);
}

#[test]
fn validate_expired() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(1000, 2000)
        .build()
        .unwrap();
    assert!(validate(&mic, 3000).is_err());
}

#[test]
fn validate_not_yet_valid() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(5000, 9000)
        .build()
        .unwrap();
    assert!(validate(&mic, 1000).is_err());
}

#[test]
fn validate_tampered_signature() {
    let kp = IdentityKeyPair::generate();
    let mut mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(1000, 5000)
        .build()
        .unwrap();
    mic.model_hash[0] ^= 0xFF; // tamper with the content
    assert!(validate(&mic, 2000).is_err());
}

#[test]
fn validate_tampered_signature_bytes() {
    let kp = IdentityKeyPair::generate();
    let mut mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(1000, 5000)
        .build()
        .unwrap();
    mic.signature[0] ^= 0xFF; // tamper with signature itself
    assert!(validate(&mic, 2000).is_err());
}

#[test]
fn validate_self_signed_chain() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0; 32])
        .validity(1000, 5000)
        .build()
        .unwrap();
    validate_chain(&[mic], 2000).unwrap();
}

#[test]
fn validate_chain_empty_fails() {
    assert!(validate_chain(&[], 2000).is_err());
}

// ── Roundtrip: build -> serialize -> deserialize -> validate ─────────────

#[test]
fn full_roundtrip() {
    let kp = IdentityKeyPair::generate();
    let mic = MICBuilder::new(&kp)
        .model_hash([0xDD; 32])
        .add_capability(Capability::CodeGeneration)
        .validity(1000, 999999)
        .build()
        .unwrap();

    // Serialize
    let bytes = serialize(&mic);

    // Deserialize
    let mic2 = deserialize(&bytes).unwrap();
    assert_eq!(mic, mic2);

    // Validate
    let result = validate(&mic2, 5000).unwrap();
    assert!(result.signature_valid);
    assert!(result.time_valid);
}
