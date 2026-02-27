//! Reliable-Ordered transport mode -- TCP-equivalent in-order delivery.
//!
//! Sender: sequence tracking with a send buffer for retransmission.
//! Receiver: in-order delivery buffer using a BTreeMap, delivers only
//! when the next expected contiguous sequence is present.

use std::collections::BTreeMap;

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Sending side for Reliable-Ordered streams.
pub struct ReliableOrderedSender {
    /// Next sequence number to assign.
    next_seq: u32,
    /// Send buffer: maps seq -> frame for potential retransmission.
    send_buffer: BTreeMap<u32, Frame>,
}

impl ReliableOrderedSender {
    pub fn new() -> Self {
        Self {
            next_seq: 0,
            send_buffer: BTreeMap::new(),
        }
    }

    /// Number of unacknowledged frames in the send buffer.
    pub fn in_flight(&self) -> usize {
        self.send_buffer.len()
    }
}

impl Default for ReliableOrderedSender {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportSender for ReliableOrderedSender {
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

/// Receiving side for Reliable-Ordered streams.
pub struct ReliableOrderedReceiver {
    /// The next sequence number we expect to deliver.
    expected_seq: u32,
    /// Buffer for out-of-order frames awaiting contiguous delivery.
    recv_buffer: BTreeMap<u32, Bytes>,
}

impl ReliableOrderedReceiver {
    pub fn new() -> Self {
        Self {
            expected_seq: 0,
            recv_buffer: BTreeMap::new(),
        }
    }

    /// Return the next expected sequence number.
    pub fn expected_seq(&self) -> u32 {
        self.expected_seq
    }
}

impl Default for ReliableOrderedReceiver {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportReceiver for ReliableOrderedReceiver {
    fn receive(&mut self, frame: &Frame) -> Result<Vec<Bytes>> {
        match frame {
            Frame::Data {
                seq, payload, ..
            } => {
                // Insert into the buffer (idempotent for duplicates).
                self.recv_buffer
                    .entry(*seq)
                    .or_insert_with(|| payload.clone());

                // Deliver as many contiguous frames as possible.
                let mut delivered = Vec::new();
                while let Some(data) = self.recv_buffer.remove(&self.expected_seq) {
                    delivered.push(data);
                    self.expected_seq = self.expected_seq.wrapping_add(1);
                }
                Ok(delivered)
            }
            _ => Err(NexStreamError::Internal(
                "ReliableOrderedReceiver received non-data frame".into(),
            )),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn in_order_delivery() {
        let mut sender = ReliableOrderedSender::new();
        let mut receiver = ReliableOrderedReceiver::new();

        let frames = sender
            .send(1, Bytes::from_static(b"hello"))
            .unwrap();
        let delivered = receiver.receive(&frames[0]).unwrap();
        assert_eq!(delivered.len(), 1);
        assert_eq!(&delivered[0][..], b"hello");
    }

    #[test]
    fn out_of_order_buffering() {
        let mut sender = ReliableOrderedSender::new();
        let mut receiver = ReliableOrderedReceiver::new();

        let f0 = sender.send(1, Bytes::from_static(b"A")).unwrap();
        let f1 = sender.send(1, Bytes::from_static(b"B")).unwrap();
        let f2 = sender.send(1, Bytes::from_static(b"C")).unwrap();

        // Deliver out of order: 1, 2, then 0.
        let d = receiver.receive(&f1[0]).unwrap();
        assert!(d.is_empty()); // seq 1 but expected 0
        let d = receiver.receive(&f2[0]).unwrap();
        assert!(d.is_empty()); // seq 2 still waiting

        // Now deliver 0 -> should flush 0, 1, 2.
        let d = receiver.receive(&f0[0]).unwrap();
        assert_eq!(d.len(), 3);
        assert_eq!(&d[0][..], b"A");
        assert_eq!(&d[1][..], b"B");
        assert_eq!(&d[2][..], b"C");
    }

    #[test]
    fn ack_removes_from_send_buffer() {
        let mut sender = ReliableOrderedSender::new();
        sender.send(1, Bytes::from_static(b"A")).unwrap();
        sender.send(1, Bytes::from_static(b"B")).unwrap();
        assert_eq!(sender.in_flight(), 2);
        sender.on_ack(0);
        assert_eq!(sender.in_flight(), 1);
        sender.on_ack(1);
        assert_eq!(sender.in_flight(), 0);
    }
}
