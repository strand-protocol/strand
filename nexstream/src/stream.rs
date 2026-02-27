//! Individual stream state and operations.
//!
//! Each stream has an ID, a transport mode (immutable), and a state machine:
//! Idle -> Open -> HalfClosedLocal / HalfClosedRemote -> Closed.

use std::collections::VecDeque;
use std::fmt;

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
use crate::transport::TransportMode;

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
pub struct Stream {
    /// Stream identifier.
    id: u32,
    /// The delivery mode for this stream (set at creation, immutable).
    mode: TransportMode,
    /// Current state.
    state: StreamState,
    /// Outbound data buffer.
    send_buf: VecDeque<Bytes>,
    /// Inbound data buffer.
    recv_buf: VecDeque<Bytes>,
}

impl Stream {
    /// Create a new stream in the Idle state.
    pub fn new(id: u32, mode: TransportMode) -> Self {
        Self {
            id,
            mode,
            state: StreamState::Idle,
            send_buf: VecDeque::new(),
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
            _ => Err(NexStreamError::InvalidStateTransition {
                from: self.state.to_string(),
                to: "Open".into(),
            }),
        }
    }

    /// Queue data for sending on this stream.
    pub fn send(&mut self, data: Bytes) -> Result<()> {
        match self.state {
            StreamState::Open | StreamState::HalfClosedRemote => {
                self.send_buf.push_back(data);
                Ok(())
            }
            StreamState::HalfClosedLocal | StreamState::Closed => {
                Err(NexStreamError::StreamClosed(self.id))
            }
            StreamState::Idle => Err(NexStreamError::InvalidStateTransition {
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
                } else {
                    if self.state == StreamState::Closed {
                        Err(NexStreamError::StreamClosed(self.id))
                    } else {
                        Ok(None)
                    }
                }
            }
            StreamState::Idle => Err(NexStreamError::InvalidStateTransition {
                from: "Idle".into(),
                to: "recv".into(),
            }),
        }
    }

    /// Enqueue received data into the receive buffer (called by the mux layer).
    pub fn push_recv(&mut self, data: Bytes) {
        self.recv_buf.push_back(data);
    }

    /// Drain pending send data (called by the mux layer).
    pub fn drain_send(&mut self) -> Vec<Bytes> {
        self.send_buf.drain(..).collect()
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
            StreamState::Idle => Err(NexStreamError::InvalidStateTransition {
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
        self.send_buf.clear();
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
        let drained = s.drain_send();
        assert_eq!(drained.len(), 1);

        s.push_recv(Bytes::from_static(b"world"));
        let data = s.recv().unwrap().unwrap();
        assert_eq!(&data[..], b"world");
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
}
