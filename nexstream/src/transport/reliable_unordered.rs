//! Reliable-Unordered transport mode -- exactly-once delivery, no ordering.
//!
//! Sender: sequence tracking + send buffer for retransmission (same as RO).
//! Receiver: tracks delivered seq via HashSet, delivers immediately (no buffering).

use std::collections::{BTreeMap, HashSet};

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Sending side for Reliable-Unordered streams.
pub struct ReliableUnorderedSender {
    next_seq: u32,
    send_buffer: BTreeMap<u32, Frame>,
}

impl ReliableUnorderedSender {
    pub fn new() -> Self {
        Self {
            next_seq: 0,
            send_buffer: BTreeMap::new(),
        }
    }

    pub fn in_flight(&self) -> usize {
        self.send_buffer.len()
    }
}

impl Default for ReliableUnorderedSender {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportSender for ReliableUnorderedSender {
    fn send(&mut self, stream_id: u32, data: Bytes) -> Result<Vec<Frame>> {
        let seq = self.next_seq;
        self.next_seq = self.next_seq.wrapping_add(1);
        let frame = Frame::Data {
            stream_id,
            seq,
            flags: DataFlags::NONE,
            payload: data,
        };
        self.send_buffer.insert(seq, frame.clone());
        Ok(vec![frame])
    }

    fn on_ack(&mut self, seq: u32) {
        self.send_buffer.remove(&seq);
    }

    fn retransmit(&mut self) -> Vec<Frame> {
        self.send_buffer.values().cloned().collect()
    }
}

/// Receiving side for Reliable-Unordered streams.
///
/// Delivers frames immediately and uses a `HashSet` to ensure exactly-once
/// delivery (duplicates are silently dropped).
pub struct ReliableUnorderedReceiver {
    /// Set of sequence numbers already delivered.
    delivered: HashSet<u32>,
}

impl ReliableUnorderedReceiver {
    pub fn new() -> Self {
        Self {
            delivered: HashSet::new(),
        }
    }
}

impl Default for ReliableUnorderedReceiver {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportReceiver for ReliableUnorderedReceiver {
    fn receive(&mut self, frame: &Frame) -> Result<Vec<Bytes>> {
        match frame {
            Frame::Data {
                seq, payload, ..
            } => {
                // Deduplicate: only deliver if not already seen.
                if self.delivered.insert(*seq) {
                    Ok(vec![payload.clone()])
                } else {
                    Ok(vec![]) // duplicate, drop
                }
            }
            _ => Err(NexStreamError::Internal(
                "ReliableUnorderedReceiver received non-data frame".into(),
            )),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn immediate_delivery() {
        let mut sender = ReliableUnorderedSender::new();
        let mut receiver = ReliableUnorderedReceiver::new();

        let f1 = sender.send(1, Bytes::from_static(b"B")).unwrap();
        let f0 = sender.send(1, Bytes::from_static(b"A")).unwrap();

        // Deliver f1 first -- should arrive immediately.
        let d = receiver.receive(&f1[0]).unwrap();
        assert_eq!(d.len(), 1);
        assert_eq!(&d[0][..], b"B");

        let d = receiver.receive(&f0[0]).unwrap();
        assert_eq!(d.len(), 1);
        assert_eq!(&d[0][..], b"A");
    }

    #[test]
    fn dedup_duplicates() {
        let mut sender = ReliableUnorderedSender::new();
        let mut receiver = ReliableUnorderedReceiver::new();

        let f = sender.send(1, Bytes::from_static(b"X")).unwrap();
        let d = receiver.receive(&f[0]).unwrap();
        assert_eq!(d.len(), 1);

        // Duplicate delivery.
        let d = receiver.receive(&f[0]).unwrap();
        assert!(d.is_empty());
    }
}
