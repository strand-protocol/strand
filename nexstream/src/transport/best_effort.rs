//! Best-Effort transport mode -- fire and forget.
//!
//! No state, no retransmission, no ordering guarantees.

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Sending side for Best-Effort streams. Stateless -- just assign sequence
/// numbers for receiver-side reordering hints (optional per spec).
pub struct BestEffortSender {
    next_seq: u32,
}

impl BestEffortSender {
    pub fn new() -> Self {
        Self { next_seq: 0 }
    }
}

impl Default for BestEffortSender {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportSender for BestEffortSender {
    fn send(&mut self, stream_id: u32, data: Bytes) -> Result<Vec<Frame>> {
        let seq = self.next_seq;
        self.next_seq = self.next_seq.wrapping_add(1);
        Ok(vec![Frame::Data {
            stream_id,
            seq,
            flags: DataFlags::NONE,
            payload: data,
        }])
    }

    fn on_ack(&mut self, _seq: u32) {
        // No-op: best effort does not track acknowledgements.
    }

    fn retransmit(&mut self) -> Vec<Frame> {
        // No retransmission for best effort.
        Vec::new()
    }
}

/// Receiving side for Best-Effort streams. Delivers immediately, no
/// deduplication or ordering.
pub struct BestEffortReceiver;

impl BestEffortReceiver {
    pub fn new() -> Self {
        Self
    }
}

impl Default for BestEffortReceiver {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportReceiver for BestEffortReceiver {
    fn receive(&mut self, frame: &Frame) -> Result<Vec<Bytes>> {
        match frame {
            Frame::Data { payload, .. } => Ok(vec![payload.clone()]),
            _ => Err(NexStreamError::Internal(
                "BestEffortReceiver received non-data frame".into(),
            )),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fire_and_forget() {
        let mut sender = BestEffortSender::new();
        let mut receiver = BestEffortReceiver::new();

        let f = sender.send(1, Bytes::from_static(b"fire")).unwrap();
        let d = receiver.receive(&f[0]).unwrap();
        assert_eq!(d.len(), 1);
        assert_eq!(&d[0][..], b"fire");
    }

    #[test]
    fn no_retransmission() {
        let mut sender = BestEffortSender::new();
        sender.send(1, Bytes::from_static(b"gone")).unwrap();
        assert!(sender.retransmit().is_empty());
    }
}
