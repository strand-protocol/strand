// MIC Validator — verify signature, expiry, and (optional) issuer chain.

use crate::crypto::keys::verify_signature;
use crate::error::{NexTrustError, Result};
use crate::mic::MIC;

/// Outcome of a successful MIC validation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ValidationResult {
    /// Whether the signature is valid.
    pub signature_valid: bool,
    /// Whether the MIC is within its validity window.
    pub time_valid: bool,
}

/// Validate a MIC's Ed25519 signature and expiry.
///
/// `now` is the current unix timestamp in seconds (caller-provided for testability).
pub fn validate(mic: &MIC, now: u64) -> Result<ValidationResult> {
    // 1. Verify the signature over the signable content.
    let signable = mic.signable_bytes();
    verify_signature(&mic.issuer_public_key, &signable, &mic.signature)?;

    // 2. Check time validity.
    if now < mic.valid_from {
        return Err(NexTrustError::MicNotYetValid {
            not_before: mic.valid_from,
            now,
        });
    }
    if now > mic.valid_until {
        return Err(NexTrustError::MicExpired {
            not_after: mic.valid_until,
            now,
        });
    }

    Ok(ValidationResult {
        signature_valid: true,
        time_valid: true,
    })
}

/// Validate a chain of MICs where each MIC's `issuer_public_key` matches the
/// `node_id` (public key) of the next MIC in the chain. The last MIC in the
/// chain must be self-signed (root CA).
///
/// `chain` is ordered leaf-first: `[leaf, intermediate..., root]`.
pub fn validate_chain(chain: &[MIC], now: u64) -> Result<()> {
    if chain.is_empty() {
        return Err(NexTrustError::MicChainValidation("empty chain".into()));
    }

    for (i, mic) in chain.iter().enumerate() {
        // Validate each individual MIC
        validate(mic, now)?;

        // For non-root MICs, verify the issuer_public_key matches the next MIC's node_id
        if i + 1 < chain.len() {
            let parent = &chain[i + 1];
            if mic.issuer_public_key != parent.node_id {
                return Err(NexTrustError::MicChainValidation(format!(
                    "MIC at index {i} issuer does not match parent at index {}",
                    i + 1
                )));
            }
        } else {
            // Root: must be self-signed (issuer_public_key == node_id)
            if mic.issuer_public_key != mic.node_id {
                return Err(NexTrustError::MicChainValidation(
                    "root MIC is not self-signed".into(),
                ));
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::keys::IdentityKeyPair;
    use crate::mic::builder::MICBuilder;

    fn make_mic(kp: &IdentityKeyPair) -> MIC {
        MICBuilder::new(kp)
            .model_hash([0xAA; 32])
            .validity(1000, 5000)
            .build()
            .unwrap()
    }

    #[test]
    fn valid_mic() {
        let kp = IdentityKeyPair::generate();
        let mic = make_mic(&kp);
        let result = validate(&mic, 2000).unwrap();
        assert!(result.signature_valid);
        assert!(result.time_valid);
    }

    #[test]
    fn expired_mic() {
        let kp = IdentityKeyPair::generate();
        let mic = make_mic(&kp);
        let err = validate(&mic, 6000).unwrap_err();
        assert!(matches!(err, NexTrustError::MicExpired { .. }));
    }

    #[test]
    fn not_yet_valid() {
        let kp = IdentityKeyPair::generate();
        let mic = make_mic(&kp);
        let err = validate(&mic, 500).unwrap_err();
        assert!(matches!(err, NexTrustError::MicNotYetValid { .. }));
    }

    #[test]
    fn tampered_mic_fails() {
        let kp = IdentityKeyPair::generate();
        let mut mic = make_mic(&kp);
        mic.model_hash[0] ^= 0xFF; // tamper
        let err = validate(&mic, 2000);
        assert!(err.is_err());
    }

    #[test]
    fn self_signed_chain() {
        let kp = IdentityKeyPair::generate();
        let mic = make_mic(&kp);
        validate_chain(&[mic], 2000).unwrap();
    }
}
