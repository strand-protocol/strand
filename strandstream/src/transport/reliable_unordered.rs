//! Reliable-Unordered transport mode -- exactly-once delivery, no ordering.
//!
//! Sender: sequence tracking + send buffer for retransmission (same as RO).
//! Receiver: tracks delivered seq numbers for exactly-once delivery using a
//! `BTreeSet`.  Old entries are garbage-collected once the set exceeds
//! `DELIVERED_GC_THRESHOLD` — the lowest `DELIVERED_GC_KEEP` entries are
//! removed, preserving the ability to detect near-term duplicates without
//! growing without bound.

use std::collections::{BTreeMap, BTreeSet};

use bytes::Bytes;

use crate::error::{StrandStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Maximum number of sequence numbers in the delivered set before GC runs.
const DELIVERED_GC_THRESHOLD: usize = 1024;

/// Number of oldest entries to discard when GC runs.
///
/// We keep the most recent `DELIVERED_GC_THRESHOLD - DELIVERED_GC_DISCARD`
/// entries so that late duplicates of recently-seen packets are still caught.
const DELIVERED_GC_DISCARD: usize = 512;

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
/// Delivers frames immediately and uses a `BTreeSet` to ensure exactly-once
/// delivery (duplicates are silently dropped).
///
/// ## Garbage collection
///
/// The delivered-set is bounded to prevent unbounded memory growth.  Once it
/// reaches `DELIVERED_GC_THRESHOLD` entries the oldest `DELIVERED_GC_DISCARD`
/// sequence numbers are removed.  This means that a very delayed retransmit
/// whose sequence number has been GC'd will be re-delivered, but only after
/// at least `DELIVERED_GC_THRESHOLD - DELIVERED_GC_DISCARD` newer packets
/// have been received — an acceptable trade-off for long-running streams.
pub struct ReliableUnorderedReceiver {
    /// Set of sequence numbers already delivered (ordered for efficient GC).
    delivered: BTreeSet<u32>,
}

impl ReliableUnorderedReceiver {
    pub fn new() -> Self {
        Self {
            delivered: BTreeSet::new(),
        }
    }

    /// Remove the oldest `DELIVERED_GC_DISCARD` entries from the delivered set.
    ///
    /// Called automatically when the set size hits `DELIVERED_GC_THRESHOLD`.
    fn gc(&mut self) {
        // Collect the lowest DELIVERED_GC_DISCARD keys.
        let to_remove: Vec<u32> = self
            .delivered
            .iter()
            .copied()
            .take(DELIVERED_GC_DISCARD)
            .collect();
        for seq in to_remove {
            self.delivered.remove(&seq);
        }
    }

    /// Returns the number of sequence numbers currently tracked.
    #[cfg(test)]
    pub fn delivered_count(&self) -> usize {
        self.delivered.len()
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
            Frame::Data { seq, payload, .. } => {
                // Deduplicate: only deliver if not already seen.
                if self.delivered.insert(*seq) {
                    // Run GC if the set has grown too large.
                    if self.delivered.len() >= DELIVERED_GC_THRESHOLD {
                        self.gc();
                    }
                    Ok(vec![payload.clone()])
                } else {
                    Ok(vec![]) // duplicate, drop silently
                }
            }
            _ => Err(StrandStreamError::Internal(
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

    #[test]
    fn gc_bounds_delivered_set() {
        let mut receiver = ReliableUnorderedReceiver::new();

        // Deliver DELIVERED_GC_THRESHOLD + 1 unique frames.
        // After the (THRESHOLD)th insertion the GC should run, removing
        // DELIVERED_GC_DISCARD entries.
        let limit = DELIVERED_GC_THRESHOLD + 1;
        for seq in 0..limit as u32 {
            let frame = Frame::Data {
                stream_id: 1,
                seq,
                flags: crate::frame::DataFlags::NONE,
                payload: Bytes::from_static(b"x"),
            };
            let d = receiver.receive(&frame).unwrap();
            assert_eq!(d.len(), 1, "frame {seq} should be delivered once");
        }

        // After GC the set must be smaller than DELIVERED_GC_THRESHOLD.
        assert!(
            receiver.delivered_count() < DELIVERED_GC_THRESHOLD,
            "delivered set should be bounded after GC, got {}",
            receiver.delivered_count()
        );
    }

    #[test]
    fn gc_does_not_drop_recent_duplicates() {
        let mut receiver = ReliableUnorderedReceiver::new();

        // Fill past the GC threshold so that GC runs.
        for seq in 0..DELIVERED_GC_THRESHOLD as u32 {
            let frame = Frame::Data {
                stream_id: 1,
                seq,
                flags: crate::frame::DataFlags::NONE,
                payload: Bytes::from_static(b"x"),
            };
            receiver.receive(&frame).unwrap();
        }

        // The highest sequence numbers (most recent) should still be tracked.
        let high_seq = (DELIVERED_GC_THRESHOLD - 1) as u32;
        let dup_frame = Frame::Data {
            stream_id: 1,
            seq: high_seq,
            flags: crate::frame::DataFlags::NONE,
            payload: Bytes::from_static(b"dup"),
        };
        let d = receiver.receive(&dup_frame).unwrap();
        assert!(d.is_empty(), "recent duplicate must still be deduplicated");
    }
}
