//! Probabilistic transport mode -- accept frames with configurable probability.
//!
//! The receiver accepts each incoming frame independently with probability `p`
//! (configured in the range 0.0..=1.0). No retransmission, no ordering.

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Sending side for Probabilistic streams. Identical to best-effort sender.
pub struct ProbabilisticSender {
    next_seq: u32,
}

impl ProbabilisticSender {
    pub fn new() -> Self {
        Self { next_seq: 0 }
    }
}

impl Default for ProbabilisticSender {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportSender for ProbabilisticSender {
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
        // No acknowledgement tracking.
    }

    fn retransmit(&mut self) -> Vec<Frame> {
        // No retransmission.
        Vec::new()
    }
}

/// Receiving side for Probabilistic streams.
///
/// Each frame is accepted with probability `p`. Frames that fail the
/// probability check are silently dropped.
pub struct ProbabilisticReceiver {
    /// Delivery probability, clamped to [0.0, 1.0].
    probability: f64,
}

impl ProbabilisticReceiver {
    /// Create a new probabilistic receiver with the given delivery probability.
    ///
    /// `probability` is clamped to the range [0.0, 1.0].
    pub fn new(probability: f64) -> Self {
        Self {
            probability: probability.clamp(0.0, 1.0),
        }
    }

    /// Returns the configured delivery probability.
    pub fn probability(&self) -> f64 {
        self.probability
    }
}

impl TransportReceiver for ProbabilisticReceiver {
    fn receive(&mut self, frame: &Frame) -> Result<Vec<Bytes>> {
        match frame {
            Frame::Data { payload, .. } => {
                if rand::random::<f64>() < self.probability {
                    Ok(vec![payload.clone()])
                } else {
                    Ok(vec![]) // dropped by probability
                }
            }
            _ => Err(NexStreamError::Internal(
                "ProbabilisticReceiver received non-data frame".into(),
            )),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn probability_one_always_delivers() {
        let mut sender = ProbabilisticSender::new();
        let mut receiver = ProbabilisticReceiver::new(1.0);

        for _ in 0..100 {
            let f = sender.send(1, Bytes::from_static(b"data")).unwrap();
            let d = receiver.receive(&f[0]).unwrap();
            assert_eq!(d.len(), 1);
        }
    }

    #[test]
    fn probability_zero_never_delivers() {
        let mut sender = ProbabilisticSender::new();
        let mut receiver = ProbabilisticReceiver::new(0.0);

        for _ in 0..100 {
            let f = sender.send(1, Bytes::from_static(b"data")).unwrap();
            let d = receiver.receive(&f[0]).unwrap();
            assert!(d.is_empty());
        }
    }

    #[test]
    fn probability_delivers_roughly_expected_ratio() {
        let mut sender = ProbabilisticSender::new();
        let mut receiver = ProbabilisticReceiver::new(0.5);

        let trials = 10_000;
        let mut delivered = 0usize;
        for _ in 0..trials {
            let f = sender.send(1, Bytes::from_static(b"d")).unwrap();
            let d = receiver.receive(&f[0]).unwrap();
            delivered += d.len();
        }

        // Expect roughly 50% +/- 5% (very generous tolerance).
        let ratio = delivered as f64 / trials as f64;
        assert!(
            (0.40..=0.60).contains(&ratio),
            "delivery ratio {ratio} outside expected range"
        );
    }
}
