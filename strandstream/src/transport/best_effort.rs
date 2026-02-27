//! Best-Effort transport mode -- fire and forget.
//!
//! No retransmission, no ordering guarantees, no duplicate suppression.
//!
//! ## Congestion window gating
//!
//! The sender carries an optional congestion window (`cwnd`).  When `cwnd` is
//! set to `Some(0)` the sender drops outgoing frames silently (logging them at
//! `DEBUG` level via `tracing`) rather than returning an error.  This matches
//! the Best-Effort contract: the application never blocks, but packets are
//! discarded when the network cannot absorb them.
//!
//! The congestion window is updated externally by the connection's CUBIC
//! controller via `set_cwnd()`.  Passing `None` disables window gating
//! entirely (the default for newly created streams).

use bytes::Bytes;

use crate::error::{StrandStreamError, Result};
use crate::frame::{DataFlags, Frame};
use crate::transport::{TransportReceiver, TransportSender};

/// Sending side for Best-Effort streams.
///
/// Assigns monotonically increasing sequence numbers (wrapping at u32::MAX)
/// so that the receiver can detect reordering if desired, though reordering
/// does not affect delivery for BE streams.
pub struct BestEffortSender {
    /// Next sequence number to assign.
    next_seq: u32,
    /// Optional congestion window in bytes.  `Some(0)` means the window is
    /// exhausted and frames should be dropped.  `None` disables gating.
    cwnd: Option<u32>,
}

impl BestEffortSender {
    pub fn new() -> Self {
        Self {
            next_seq: 0,
            cwnd: None,
        }
    }

    /// Update the congestion window.
    ///
    /// Set to `Some(0)` to signal that no new frames should be sent.
    /// Set to `None` to disable window gating (e.g. for uncongested paths).
    pub fn set_cwnd(&mut self, cwnd: Option<u32>) {
        self.cwnd = cwnd;
    }

    /// Returns `true` if the congestion window permits sending.
    ///
    /// A window of `None` or any non-zero value allows sending.
    fn window_open(&self) -> bool {
        match self.cwnd {
            None => true,
            Some(0) => false,
            Some(_) => true,
        }
    }
}

impl Default for BestEffortSender {
    fn default() -> Self {
        Self::new()
    }
}

impl TransportSender for BestEffortSender {
    /// Transmit `data` on `stream_id`.
    ///
    /// If the congestion window is exhausted (`cwnd == Some(0)`) the frame is
    /// silently dropped and an empty `Vec` is returned, preserving the
    /// fire-and-forget contract.  The drop is logged at `DEBUG` level so that
    /// diagnostic tooling can observe it without imposing overhead on the hot
    /// path.
    fn send(&mut self, stream_id: u32, data: Bytes) -> Result<Vec<Frame>> {
        if !self.window_open() {
            tracing::debug!(
                stream_id,
                payload_len = data.len(),
                "best-effort frame dropped: congestion window exhausted (cwnd=0)"
            );
            // Return empty vec -- caller gets Ok(()), frame is silently gone.
            return Ok(vec![]);
        }

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
            _ => Err(StrandStreamError::Internal(
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

    #[test]
    fn cwnd_zero_drops_frame() {
        let mut sender = BestEffortSender::new();
        // Exhaust the congestion window.
        sender.set_cwnd(Some(0));

        let frames = sender.send(1, Bytes::from_static(b"dropped")).unwrap();
        // With window exhausted, the send returns Ok but produces no frames.
        assert!(
            frames.is_empty(),
            "expected frame to be dropped when cwnd=0"
        );
    }

    #[test]
    fn cwnd_nonzero_sends_normally() {
        let mut sender = BestEffortSender::new();
        sender.set_cwnd(Some(65535));

        let frames = sender.send(1, Bytes::from_static(b"ok")).unwrap();
        assert_eq!(frames.len(), 1);
    }

    #[test]
    fn cwnd_none_disables_gating() {
        let mut sender = BestEffortSender::new();
        // Default: no window set.
        let frames = sender.send(1, Bytes::from_static(b"ok")).unwrap();
        assert_eq!(frames.len(), 1);

        // Explicitly disable gating.
        sender.set_cwnd(None);
        let frames = sender.send(1, Bytes::from_static(b"also ok")).unwrap();
        assert_eq!(frames.len(), 1);
    }

    #[test]
    fn cwnd_transition_drop_then_send() {
        let mut sender = BestEffortSender::new();

        // Closed window: drop.
        sender.set_cwnd(Some(0));
        let f = sender.send(1, Bytes::from_static(b"x")).unwrap();
        assert!(f.is_empty());

        // Reopen window: frame goes through.
        sender.set_cwnd(Some(1024));
        let f = sender.send(1, Bytes::from_static(b"y")).unwrap();
        assert_eq!(f.len(), 1);
    }
}
