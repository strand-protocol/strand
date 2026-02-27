//! Individual stream state and operations.
//!
//! Each stream has an ID, a transport mode (immutable), and a state machine:
//! Idle -> Open -> HalfClosedLocal / HalfClosedRemote -> Closed.
//!
//! The send and receive paths are delegated to mode-specific `TransportSender`
//! and `TransportReceiver` objects (see `crate::transport`). This ensures that
//! RU, BE, and Probabilistic streams use their proper deduplication, congestion,
//! and ordering semantics rather than a generic `VecDeque`.

use std::collections::VecDeque;
use std::fmt;

use bytes::Bytes;

use crate::error::{StrandStreamError, Result};
use crate::frame::Frame;
use crate::transport::best_effort::{BestEffortReceiver, BestEffortSender};
use crate::transport::probabilistic::{ProbabilisticReceiver, ProbabilisticSender};
use crate::transport::reliable_ordered::{ReliableOrderedReceiver, ReliableOrderedSender};
use crate::transport::reliable_unordered::{ReliableUnorderedReceiver, ReliableUnorderedSender};
use crate::transport::{TransportMode, TransportReceiver, TransportSender};

/// Stream state machine states.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum StreamState {
    /// Stream has been allocated but not yet opened.
    Idle,
    /// Stream is fully open for bidirectional communication.
    Open,
    /// Local side has sent FIN; can still receive.
    HalfClosedLocal,
    /// Remote side has sent FIN; can still send.
    HalfClosedRemote,
    /// Stream is fully closed.
    Closed,
}

impl fmt::Display for StreamState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            StreamState::Idle => write!(f, "Idle"),
            StreamState::Open => write!(f, "Open"),
            StreamState::HalfClosedLocal => write!(f, "HalfClosedLocal"),
            StreamState::HalfClosedRemote => write!(f, "HalfClosedRemote"),
            StreamState::Closed => write!(f, "Closed"),
        }
    }
}

/// A single multiplexed stream.
///
/// The outbound path goes through a mode-specific `TransportSender` that
/// produces wire-ready `Frame` values (with sequence numbers assigned).
/// The inbound path goes through a mode-specific `TransportReceiver` that
/// applies mode semantics (ordering, deduplication, congestion gating) and
/// returns zero or more application-visible `Bytes` payloads.
///
/// A plain `VecDeque<Bytes>` is still kept as the **application-level receive
/// queue** — the receiver converts `Frame`s into `Bytes` and those are pushed
/// here so that `Stream::recv()` remains a simple pop operation.
pub struct Stream {
    /// Stream identifier.
    id: u32,
    /// The delivery mode for this stream (set at creation, immutable).
    mode: TransportMode,
    /// Current state.
    state: StreamState,
    /// Mode-specific sender: assigns sequence numbers, buffers for retransmit.
    sender: Box<dyn TransportSender>,
    /// Mode-specific receiver: applies ordering / dedup / probability filter.
    receiver: Box<dyn TransportReceiver>,
    /// Outbound frames ready to be handed to the network layer.
    pending_frames: Vec<Frame>,
    /// Application-level receive queue: payloads extracted by the receiver.
    recv_buf: VecDeque<Bytes>,
}

impl Stream {
    /// Create a new stream in the Idle state.
    ///
    /// The correct `TransportSender` / `TransportReceiver` pair is instantiated
    /// based on `mode`.  Probabilistic streams use a 50% delivery probability
    /// by default; callers that need a different probability should use
    /// `new_probabilistic()`.
    pub fn new(id: u32, mode: TransportMode) -> Self {
        let (sender, receiver): (Box<dyn TransportSender>, Box<dyn TransportReceiver>) =
            match mode {
                TransportMode::ReliableOrdered => (
                    Box::new(ReliableOrderedSender::new()),
                    Box::new(ReliableOrderedReceiver::new()),
                ),
                TransportMode::ReliableUnordered => (
                    Box::new(ReliableUnorderedSender::new()),
                    Box::new(ReliableUnorderedReceiver::new()),
                ),
                TransportMode::BestEffort => (
                    Box::new(BestEffortSender::new()),
                    Box::new(BestEffortReceiver::new()),
                ),
                TransportMode::Probabilistic => (
                    Box::new(ProbabilisticSender::new()),
                    // Default probability 0.5 — override with `new_probabilistic`.
                    Box::new(ProbabilisticReceiver::new(0.5)),
                ),
            };

        Self {
            id,
            mode,
            state: StreamState::Idle,
            sender,
            receiver,
            pending_frames: Vec::new(),
            recv_buf: VecDeque::new(),
        }
    }

    /// Create a new Probabilistic stream with a custom delivery probability.
    pub fn new_probabilistic(id: u32, probability: f64) -> Self {
        let sender: Box<dyn TransportSender> = Box::new(ProbabilisticSender::new());
        let receiver: Box<dyn TransportReceiver> =
            Box::new(ProbabilisticReceiver::new(probability));
        Self {
            id,
            mode: TransportMode::Probabilistic,
            state: StreamState::Idle,
            sender,
            receiver,
            pending_frames: Vec::new(),
            recv_buf: VecDeque::new(),
        }
    }

    /// Returns the stream ID.
    pub fn id(&self) -> u32 {
        self.id
    }

    /// Returns the transport mode.
    pub fn mode(&self) -> TransportMode {
        self.mode
    }

    /// Returns the current state.
    pub fn state(&self) -> StreamState {
        self.state
    }

    /// Transition the stream to the Open state.
    pub fn open(&mut self) -> Result<()> {
        match self.state {
            StreamState::Idle => {
                self.state = StreamState::Open;
                Ok(())
            }
            _ => Err(StrandStreamError::InvalidStateTransition {
                from: self.state.to_string(),
                to: "Open".into(),
            }),
        }
    }

    /// Queue data for sending on this stream.
    ///
    /// The data is passed to the mode-specific `TransportSender` which assigns
    /// a sequence number and returns the wire frame(s).  Those frames are
    /// accumulated in `pending_frames` for the mux layer to drain via
    /// `drain_frames()`.
    pub fn send(&mut self, data: Bytes) -> Result<()> {
        match self.state {
            StreamState::Open | StreamState::HalfClosedRemote => {
                let frames = self.sender.send(self.id, data)?;
                self.pending_frames.extend(frames);
                Ok(())
            }
            StreamState::HalfClosedLocal | StreamState::Closed => {
                Err(StrandStreamError::StreamClosed(self.id))
            }
            StreamState::Idle => Err(StrandStreamError::InvalidStateTransition {
                from: "Idle".into(),
                to: "send".into(),
            }),
        }
    }

    /// Receive data from this stream (returns buffered data, if any).
    pub fn recv(&mut self) -> Result<Option<Bytes>> {
        match self.state {
            StreamState::Open | StreamState::HalfClosedLocal => {
                Ok(self.recv_buf.pop_front())
            }
            StreamState::HalfClosedRemote | StreamState::Closed => {
                // Can still drain buffer even if remote closed.
                if let Some(data) = self.recv_buf.pop_front() {
                    Ok(Some(data))
                } else if self.state == StreamState::Closed {
                    Err(StrandStreamError::StreamClosed(self.id))
                } else {
                    Ok(None)
                }
            }
            StreamState::Idle => Err(StrandStreamError::InvalidStateTransition {
                from: "Idle".into(),
                to: "recv".into(),
            }),
        }
    }

    /// Process an inbound `Frame` through the mode-specific `TransportReceiver`.
    ///
    /// The receiver applies mode semantics (in-order reassembly for RO,
    /// deduplication for RU, probabilistic drop for PR, unconditional delivery
    /// for BE) and returns the payloads that are ready for the application.
    /// Those payloads are pushed onto the application-level receive queue so
    /// that subsequent `recv()` calls return them.
    ///
    /// This is the primary inbound path called by the mux layer; the legacy
    /// `push_recv()` helper is preserved for direct testing.
    pub fn transport_receive(&mut self, frame: &Frame) -> Result<()> {
        let payloads = self.receiver.receive(frame)?;
        for payload in payloads {
            self.recv_buf.push_back(payload);
        }
        Ok(())
    }

    /// Enqueue received data into the receive buffer directly (bypasses the
    /// transport receiver; useful for unit tests and the pure-Go overlay path).
    pub fn push_recv(&mut self, data: Bytes) {
        self.recv_buf.push_back(data);
    }

    /// Drain all outbound frames produced by `send()` calls (called by the
    /// mux layer before transmitting to the network).
    pub fn drain_frames(&mut self) -> Vec<Frame> {
        std::mem::take(&mut self.pending_frames)
    }

    /// Drain pending send data as raw `Bytes` (legacy helper; used by tests
    /// that do not inspect frame structure).
    ///
    /// Each `Bytes` value is the payload from one pending `Frame::Data`.
    /// Non-data frames (if any) are silently dropped here — callers that need
    /// full frame access should use `drain_frames()`.
    pub fn drain_send(&mut self) -> Vec<Bytes> {
        self.pending_frames
            .drain(..)
            .filter_map(|f| {
                if let Frame::Data { payload, .. } = f {
                    Some(payload)
                } else {
                    None
                }
            })
            .collect()
    }

    /// Notify the sender that a sequence number was acknowledged.
    ///
    /// For RO and RU streams this removes the frame from the retransmit buffer.
    /// For BE / Probabilistic streams this is a no-op.
    pub fn on_ack(&mut self, seq: u32) {
        self.sender.on_ack(seq);
    }

    /// Retrieve any frames that need retransmission (called by the loss-
    /// detection layer).
    pub fn retransmit(&mut self) -> Vec<Frame> {
        self.sender.retransmit()
    }

    /// Close the local side of the stream.
    pub fn close(&mut self) -> Result<()> {
        match self.state {
            StreamState::Open => {
                self.state = StreamState::HalfClosedLocal;
                Ok(())
            }
            StreamState::HalfClosedRemote => {
                self.state = StreamState::Closed;
                Ok(())
            }
            StreamState::Closed | StreamState::HalfClosedLocal => {
                Ok(()) // idempotent
            }
            StreamState::Idle => Err(StrandStreamError::InvalidStateTransition {
                from: "Idle".into(),
                to: "Closed".into(),
            }),
        }
    }

    /// Mark the remote side as closed.
    pub fn remote_close(&mut self) {
        match self.state {
            StreamState::Open => {
                self.state = StreamState::HalfClosedRemote;
            }
            StreamState::HalfClosedLocal => {
                self.state = StreamState::Closed;
            }
            _ => {} // ignore in other states
        }
    }

    /// Abruptly reset the stream.
    pub fn reset(&mut self) {
        self.state = StreamState::Closed;
        self.pending_frames.clear();
        self.recv_buf.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn state_transitions() {
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        assert_eq!(s.state(), StreamState::Idle);

        s.open().unwrap();
        assert_eq!(s.state(), StreamState::Open);

        s.close().unwrap();
        assert_eq!(s.state(), StreamState::HalfClosedLocal);

        s.remote_close();
        assert_eq!(s.state(), StreamState::Closed);
    }

    #[test]
    fn send_recv_buffers() {
        let mut s = Stream::new(1, TransportMode::BestEffort);
        s.open().unwrap();

        s.send(Bytes::from_static(b"hello")).unwrap();
        // drain_send() extracts raw payloads from pending frames.
        let drained = s.drain_send();
        assert_eq!(drained.len(), 1);
        assert_eq!(&drained[0][..], b"hello");

        s.push_recv(Bytes::from_static(b"world"));
        let data = s.recv().unwrap().unwrap();
        assert_eq!(&data[..], b"world");
    }

    #[test]
    fn send_produces_frames() {
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        s.open().unwrap();
        s.send(Bytes::from_static(b"data")).unwrap();

        let frames = s.drain_frames();
        assert_eq!(frames.len(), 1);
        match &frames[0] {
            Frame::Data { stream_id, seq, payload, .. } => {
                assert_eq!(*stream_id, 1);
                assert_eq!(*seq, 0);
                assert_eq!(&payload[..], b"data");
            }
            _ => panic!("expected Data frame"),
        }
    }

    #[test]
    fn transport_receive_ru_dedup() {
        use crate::frame::DataFlags;
        let mut s = Stream::new(1, TransportMode::ReliableUnordered);
        s.open().unwrap();

        let frame = Frame::Data {
            stream_id: 1,
            seq: 42,
            flags: DataFlags::NONE,
            payload: Bytes::from_static(b"msg"),
        };

        // First delivery: payload should appear in recv buffer.
        s.transport_receive(&frame).unwrap();
        assert_eq!(s.recv().unwrap().unwrap().as_ref(), b"msg");

        // Second delivery (duplicate): no new data.
        s.transport_receive(&frame).unwrap();
        assert!(s.recv().unwrap().is_none());
    }

    #[test]
    fn transport_receive_ro_ordered() {
        use crate::frame::DataFlags;
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        s.open().unwrap();

        // Deliver seq=1 before seq=0 -- should be buffered.
        let f1 = Frame::Data {
            stream_id: 1,
            seq: 1,
            flags: DataFlags::NONE,
            payload: Bytes::from_static(b"B"),
        };
        let f0 = Frame::Data {
            stream_id: 1,
            seq: 0,
            flags: DataFlags::NONE,
            payload: Bytes::from_static(b"A"),
        };

        s.transport_receive(&f1).unwrap();
        assert!(s.recv().unwrap().is_none()); // not yet -- waiting for seq 0

        s.transport_receive(&f0).unwrap();
        // Now both 0 and 1 should flush.
        assert_eq!(s.recv().unwrap().unwrap().as_ref(), b"A");
        assert_eq!(s.recv().unwrap().unwrap().as_ref(), b"B");
    }

    #[test]
    fn cannot_send_when_half_closed_local() {
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        s.open().unwrap();
        s.close().unwrap();
        assert!(s.send(Bytes::from_static(b"fail")).is_err());
    }

    #[test]
    fn reset_clears_buffers() {
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        s.open().unwrap();
        s.send(Bytes::from_static(b"data")).unwrap();
        s.push_recv(Bytes::from_static(b"data"));
        s.reset();
        assert_eq!(s.state(), StreamState::Closed);
        assert!(s.drain_send().is_empty());
    }

    #[test]
    fn on_ack_clears_retransmit_buffer() {
        let mut s = Stream::new(1, TransportMode::ReliableOrdered);
        s.open().unwrap();
        s.send(Bytes::from_static(b"A")).unwrap();
        s.send(Bytes::from_static(b"B")).unwrap();

        assert_eq!(s.retransmit().len(), 2);
        s.on_ack(0);
        assert_eq!(s.retransmit().len(), 1);
        s.on_ack(1);
        assert!(s.retransmit().is_empty());
    }
}
