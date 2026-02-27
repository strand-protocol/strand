// MIC binary serializer / deserializer.
//
// Wire format (big-endian):
// [version:1B][node_id:32B][model_hash:32B]
// [num_caps:2B][caps...]
// [provenance_flag:1B][provenance...]
// [valid_from:8B][valid_until:8B]
// [issuer_pk:32B][signature:64B]
//
// Capability encoding:
//   [tag:1B]                            — built-in capability
//   [0xFF:1B][len:2B][utf8_bytes:lenB]  — custom capability
//
// Provenance encoding (when flag == 1):
//   [desc_len:2B][desc_bytes:desc_lenB][dataset_hash:32B][timestamp:8B]

use crate::error::{NexTrustError, Result};
use crate::mic::{Capability, Provenance, MIC};

/// Serialize a [`MIC`] into its compact binary form.
pub fn serialize(mic: &MIC) -> Vec<u8> {
    let mut buf = Vec::with_capacity(256);

    buf.push(MIC::VERSION);
    buf.extend_from_slice(&mic.node_id);
    buf.extend_from_slice(&mic.model_hash);

    // Capabilities
    let num_caps = mic.capabilities.len() as u16;
    buf.extend_from_slice(&num_caps.to_be_bytes());
    for cap in &mic.capabilities {
        buf.push(cap.tag());
        if let Capability::Custom(ref s) = cap {
            let bytes = s.as_bytes();
            let len = bytes.len() as u16;
            buf.extend_from_slice(&len.to_be_bytes());
            buf.extend_from_slice(bytes);
        }
    }

    // Provenance
    match &mic.training_provenance {
        Some(prov) => {
            buf.push(1);
            let desc_bytes = prov.description.as_bytes();
            let desc_len = desc_bytes.len() as u16;
            buf.extend_from_slice(&desc_len.to_be_bytes());
            buf.extend_from_slice(desc_bytes);
            buf.extend_from_slice(&prov.dataset_hash);
            buf.extend_from_slice(&prov.timestamp.to_be_bytes());
        }
        None => {
            buf.push(0);
        }
    }

    // Validity
    buf.extend_from_slice(&mic.valid_from.to_be_bytes());
    buf.extend_from_slice(&mic.valid_until.to_be_bytes());

    // Issuer public key
    buf.extend_from_slice(&mic.issuer_public_key);

    // Signature (last field)
    buf.extend_from_slice(&mic.signature);

    buf
}

/// Deserialize a [`MIC`] from its compact binary form.
pub fn deserialize(data: &[u8]) -> Result<MIC> {
    let mut pos: usize = 0;

    let read_u8 = |pos: &mut usize, data: &[u8]| -> Result<u8> {
        if *pos >= data.len() {
            return Err(NexTrustError::MicDeserialization("unexpected end of data".into()));
        }
        let v = data[*pos];
        *pos += 1;
        Ok(v)
    };

    let read_u16 = |pos: &mut usize, data: &[u8]| -> Result<u16> {
        if *pos + 2 > data.len() {
            return Err(NexTrustError::MicDeserialization("unexpected end of data".into()));
        }
        let v = u16::from_be_bytes([data[*pos], data[*pos + 1]]);
        *pos += 2;
        Ok(v)
    };

    let read_u64 = |pos: &mut usize, data: &[u8]| -> Result<u64> {
        if *pos + 8 > data.len() {
            return Err(NexTrustError::MicDeserialization("unexpected end of data".into()));
        }
        let mut arr = [0u8; 8];
        arr.copy_from_slice(&data[*pos..*pos + 8]);
        *pos += 8;
        Ok(u64::from_be_bytes(arr))
    };

    let read_bytes_fixed = |pos: &mut usize, data: &[u8], n: usize| -> Result<Vec<u8>> {
        if *pos + n > data.len() {
            return Err(NexTrustError::MicDeserialization("unexpected end of data".into()));
        }
        let v = data[*pos..*pos + n].to_vec();
        *pos += n;
        Ok(v)
    };

    // Version
    let version = read_u8(&mut pos, data)?;
    if version != MIC::VERSION {
        return Err(NexTrustError::MicVersionUnsupported(version as u16));
    }

    // node_id
    let node_id_bytes = read_bytes_fixed(&mut pos, data, 32)?;
    let mut node_id = [0u8; 32];
    node_id.copy_from_slice(&node_id_bytes);

    // model_hash
    let mh_bytes = read_bytes_fixed(&mut pos, data, 32)?;
    let mut model_hash = [0u8; 32];
    model_hash.copy_from_slice(&mh_bytes);

    // Capabilities
    let num_caps = read_u16(&mut pos, data)?;
    let mut capabilities = Vec::with_capacity(num_caps as usize);
    for _ in 0..num_caps {
        let tag = read_u8(&mut pos, data)?;
        if tag == 0xFF {
            // Custom capability
            let len = read_u16(&mut pos, data)? as usize;
            let payload = read_bytes_fixed(&mut pos, data, len)?;
            let cap = Capability::from_tag_and_payload(tag, &payload).ok_or_else(|| {
                NexTrustError::MicDeserialization("invalid custom capability".into())
            })?;
            capabilities.push(cap);
        } else {
            let cap = Capability::from_tag_and_payload(tag, &[]).ok_or_else(|| {
                NexTrustError::MicDeserialization(format!("unknown capability tag: 0x{tag:02x}"))
            })?;
            capabilities.push(cap);
        }
    }

    // Provenance
    let prov_flag = read_u8(&mut pos, data)?;
    let training_provenance = if prov_flag == 1 {
        let desc_len = read_u16(&mut pos, data)? as usize;
        let desc_bytes = read_bytes_fixed(&mut pos, data, desc_len)?;
        let description = String::from_utf8(desc_bytes)
            .map_err(|e| NexTrustError::MicDeserialization(format!("invalid utf8: {e}")))?;
        let dh_bytes = read_bytes_fixed(&mut pos, data, 32)?;
        let mut dataset_hash = [0u8; 32];
        dataset_hash.copy_from_slice(&dh_bytes);
        let timestamp = read_u64(&mut pos, data)?;
        Some(Provenance {
            description,
            dataset_hash,
            timestamp,
        })
    } else {
        None
    };

    // Validity
    let valid_from = read_u64(&mut pos, data)?;
    let valid_until = read_u64(&mut pos, data)?;

    // Issuer public key
    let ipk_bytes = read_bytes_fixed(&mut pos, data, 32)?;
    let mut issuer_public_key = [0u8; 32];
    issuer_public_key.copy_from_slice(&ipk_bytes);

    // Signature
    let sig_bytes = read_bytes_fixed(&mut pos, data, 64)?;
    let mut signature = [0u8; 64];
    signature.copy_from_slice(&sig_bytes);

    Ok(MIC {
        node_id,
        model_hash,
        capabilities,
        training_provenance,
        valid_from,
        valid_until,
        signature,
        issuer_public_key,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::keys::IdentityKeyPair;
    use crate::mic::builder::MICBuilder;

    #[test]
    fn roundtrip_minimal() {
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
    fn roundtrip_with_caps_and_provenance() {
        let kp = IdentityKeyPair::generate();
        let mic = MICBuilder::new(&kp)
            .model_hash([0x22; 32])
            .add_capability(Capability::TextGeneration)
            .add_capability(Capability::CodeGeneration)
            .add_capability(Capability::Custom("reasoning".into()))
            .training_provenance(Provenance {
                description: "RLHF fine-tune on dataset v3".into(),
                dataset_hash: [0x33; 32],
                timestamp: 1700000000,
            })
            .validity(1000, 2000)
            .build()
            .unwrap();
        let bytes = serialize(&mic);
        let mic2 = deserialize(&bytes).unwrap();
        assert_eq!(mic, mic2);
    }

    #[test]
    fn bad_version_fails() {
        let mut bytes = vec![0xFF]; // bad version
        bytes.extend_from_slice(&[0u8; 200]); // padding
        let result = deserialize(&bytes);
        assert!(result.is_err());
    }

    #[test]
    fn truncated_data_fails() {
        let result = deserialize(&[MIC::VERSION, 0, 0, 0]);
        assert!(result.is_err());
    }
}
