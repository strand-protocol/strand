//! Transport mode definitions and per-mode sender/receiver traits.

pub mod best_effort;
pub mod probabilistic;
pub mod reliable_ordered;
pub mod reliable_unordered;

use bytes::Bytes;

use crate::error::Result;
use crate::frame::Frame;

/// The four delivery modes supported by StrandStream.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u8)]
pub enum TransportMode {
    ReliableOrdered = 0x01,
    ReliableUnordered = 0x02,
    BestEffort = 0x03,
    Probabilistic = 0x04,
}

impl TransportMode {
    /// Convert from a raw u8.
    pub fn from_u8(v: u8) -> Result<Self> {
        match v {
            0x01 => Ok(TransportMode::ReliableOrdered),
            0x02 => Ok(TransportMode::ReliableUnordered),
            0x03 => Ok(TransportMode::BestEffort),
            0x04 => Ok(TransportMode::Probabilistic),
            other => Err(crate::error::StrandStreamError::InvalidTransportMode(other)),
        }
    }
}

/// Trait for the sending side of a transport mode.
pub trait TransportSender: Send {
    /// Enqueue data for sending. Returns the frame(s) to transmit.
    fn send(&mut self, stream_id: u32, data: Bytes) -> Result<Vec<Frame>>;
    /// Handle an acknowledgement for the given sequence number.
    fn on_ack(&mut self, seq: u32);
    /// Retrieve any frames that need retransmission.
    fn retransmit(&mut self) -> Vec<Frame>;
}

/// Trait for the receiving side of a transport mode.
pub trait TransportReceiver: Send {
    /// Process an inbound data frame. Returns data ready for the application
    /// (may be empty if the frame is buffered waiting for ordering).
    fn receive(&mut self, frame: &Frame) -> Result<Vec<Bytes>>;
}
