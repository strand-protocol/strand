// Model Identity Certificate (MIC) — the AI equivalent of X.509 certificates.

pub mod builder;
pub mod serializer;
pub mod validator;

use serde::{Deserialize, Serialize};

// ── Capability ───────────────────────────────────────────────────────────

/// A capability that the model can attest to.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum Capability {
    /// Text generation
    TextGeneration,
    /// Code generation
    CodeGeneration,
    /// Image understanding
    ImageUnderstanding,
    /// Tool use / function calling
    ToolUse,
    /// Retrieval-augmented generation
    Rag,
    /// Embedding generation
    Embedding,
    /// Custom capability with a string tag
    Custom(String),
}

impl Capability {
    /// Wire tag for serialization.
    pub fn tag(&self) -> u8 {
        match self {
            Capability::TextGeneration => 0x01,
            Capability::CodeGeneration => 0x02,
            Capability::ImageUnderstanding => 0x03,
            Capability::ToolUse => 0x04,
            Capability::Rag => 0x05,
            Capability::Embedding => 0x06,
            Capability::Custom(_) => 0xFF,
        }
    }

    /// Reconstruct from tag + optional payload bytes.
    pub fn from_tag_and_payload(tag: u8, payload: &[u8]) -> Option<Self> {
        match tag {
            0x01 => Some(Capability::TextGeneration),
            0x02 => Some(Capability::CodeGeneration),
            0x03 => Some(Capability::ImageUnderstanding),
            0x04 => Some(Capability::ToolUse),
            0x05 => Some(Capability::Rag),
            0x06 => Some(Capability::Embedding),
            0xFF => {
                let s = String::from_utf8(payload.to_vec()).ok()?;
                Some(Capability::Custom(s))
            }
            _ => None,
        }
    }
}

// ── Provenance ───────────────────────────────────────────────────────────

/// Optional provenance information describing the model's training lineage.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Provenance {
    /// Human-readable description of the training pipeline.
    pub description: String,
    /// SHA-256 hash of the training dataset Merkle root.
    pub dataset_hash: [u8; 32],
    /// Unix timestamp when training completed.
    pub timestamp: u64,
}

// ── MIC ──────────────────────────────────────────────────────────────────

/// A Model Identity Certificate.
///
/// This is the core signed document proving a model's identity, capabilities,
/// and provenance.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MIC {
    /// The 32-byte Node ID (Ed25519 public key hash, padded to 32 bytes for alignment).
    pub node_id: [u8; 32],
    /// SHA-256 hash of the model binary / weights.
    pub model_hash: [u8; 32],
    /// Capabilities this model attests to.
    pub capabilities: Vec<Capability>,
    /// Optional training provenance.
    pub training_provenance: Option<Provenance>,
    /// Unix timestamp (seconds) — start of validity.
    pub valid_from: u64,
    /// Unix timestamp (seconds) — end of validity.
    pub valid_until: u64,
    /// Ed25519 signature over the signable content (everything except signature itself).
    pub signature: [u8; 64],
    /// The issuer's Ed25519 public key (32 bytes).
    pub issuer_public_key: [u8; 32],
}

impl MIC {
    /// The canonical MIC format version.
    pub const VERSION: u8 = 1;

    /// Produce the byte content that is signed (everything except the signature field).
    /// This MUST match the order used by the serializer for the signed portion.
    pub fn signable_bytes(&self) -> Vec<u8> {
        let mut buf = Vec::with_capacity(256);
        buf.push(Self::VERSION);
        buf.extend_from_slice(&self.node_id);
        buf.extend_from_slice(&self.model_hash);
        // capabilities
        let num_caps = self.capabilities.len() as u16;
        buf.extend_from_slice(&num_caps.to_be_bytes());
        for cap in &self.capabilities {
            let tag = cap.tag();
            buf.push(tag);
            if let Capability::Custom(ref s) = cap {
                let bytes = s.as_bytes();
                let len = bytes.len() as u16;
                buf.extend_from_slice(&len.to_be_bytes());
                buf.extend_from_slice(bytes);
            }
        }
        // provenance
        match &self.training_provenance {
            Some(prov) => {
                buf.push(1); // provenance_flag = present
                let desc_bytes = prov.description.as_bytes();
                let desc_len = desc_bytes.len() as u16;
                buf.extend_from_slice(&desc_len.to_be_bytes());
                buf.extend_from_slice(desc_bytes);
                buf.extend_from_slice(&prov.dataset_hash);
                buf.extend_from_slice(&prov.timestamp.to_be_bytes());
            }
            None => {
                buf.push(0); // provenance_flag = absent
            }
        }
        // validity window
        buf.extend_from_slice(&self.valid_from.to_be_bytes());
        buf.extend_from_slice(&self.valid_until.to_be_bytes());
        // issuer public key
        buf.extend_from_slice(&self.issuer_public_key);
        buf
    }
}
