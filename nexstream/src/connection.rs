//! Connection state machine.
//!
//! Manages the lifecycle of a NexStream connection:
//! Idle -> Connecting -> Open -> Closing -> Closed.

use std::fmt;

use bytes::Bytes;

use crate::congestion::cubic::Cubic;
use crate::congestion::CongestionController;
use crate::error::{NexStreamError, Result};
use crate::flow_control::FlowController;
use crate::loss_detection::LossDetector;
use crate::mux::{Multiplexer, StreamId};
use crate::rtt::RttEstimator;
use crate::transport::TransportMode;

/// Connection state machine states.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConnectionState {
    /// Connection has not been initiated.
    Idle,
    /// Connection handshake in progress.
    Connecting,
    /// Connection is established and ready for streams.
    Open,
    /// Connection is shutting down.
    Closing,
    /// Connection is fully closed.
    Closed,
}

impl fmt::Display for ConnectionState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ConnectionState::Idle => write!(f, "Idle"),
            ConnectionState::Connecting => write!(f, "Connecting"),
            ConnectionState::Open => write!(f, "Open"),
            ConnectionState::Closing => write!(f, "Closing"),
            ConnectionState::Closed => write!(f, "Closed"),
        }
    }
}

/// Configuration for a connection.
#[derive(Debug, Clone)]
pub struct ConnectionConfig {
    /// Maximum number of concurrent streams.
    pub max_streams: u32,
    /// Connection-level flow control window.
    pub connection_window: usize,
    /// Per-stream flow control window.
    pub stream_window: usize,
}

impl Default for ConnectionConfig {
    fn default() -> Self {
        Self {
            max_streams: 1024,
            connection_window: 1024 * 1024,
            stream_window: 64 * 1024,
        }
    }
}

/// A NexStream connection.
pub struct Connection {
    /// Current connection state.
    state: ConnectionState,
    /// Stream multiplexer.
    mux: Multiplexer,
    /// Congestion controller.
    congestion: Box<dyn CongestionController>,
    /// RTT estimator.
    rtt: RttEstimator,
    /// Loss detector.
    loss_detector: LossDetector,
    /// Flow controller.
    flow_control: FlowController,
}

impl Connection {
    /// Create a new connection from the given config.
    pub fn new(config: ConnectionConfig) -> Self {
        Self {
            state: ConnectionState::Idle,
            mux: Multiplexer::new(config.max_streams),
            congestion: Box::new(Cubic::new()),
            rtt: RttEstimator::new(),
            loss_detector: LossDetector::new(),
            flow_control: FlowController::with_windows(
                config.connection_window,
                config.stream_window,
            ),
        }
    }

    /// Initiate a connection (client side).
    pub fn connect(&mut self) -> Result<()> {
        match self.state {
            ConnectionState::Idle => {
                self.state = ConnectionState::Connecting;
                Ok(())
            }
            _ => Err(NexStreamError::InvalidStateTransition {
                from: self.state.to_string(),
                to: "Connecting".into(),
            }),
        }
    }

    /// Accept a connection (server side) / complete handshake.
    pub fn accept(&mut self) -> Result<()> {
        match self.state {
            ConnectionState::Idle | ConnectionState::Connecting => {
                self.state = ConnectionState::Open;
                Ok(())
            }
            _ => Err(NexStreamError::InvalidStateTransition {
                from: self.state.to_string(),
                to: "Open".into(),
            }),
        }
    }

    /// Open a new stream on this connection.
    pub fn open_stream(&mut self, mode: TransportMode) -> Result<StreamId> {
        if self.state != ConnectionState::Open {
            return Err(NexStreamError::ConnectionClosed);
        }
        let sid = self.mux.create_stream(mode)?;
        self.flow_control.add_stream(sid);
        Ok(sid)
    }

    /// Send data on a stream.
    pub fn send(&mut self, stream_id: StreamId, data: Bytes) -> Result<()> {
        if self.state != ConnectionState::Open {
            return Err(NexStreamError::ConnectionClosed);
        }
        self.mux.send(stream_id, data)
    }

    /// Receive data from a stream.
    pub fn recv(&mut self, stream_id: StreamId) -> Result<Option<Bytes>> {
        if self.state != ConnectionState::Open {
            return Err(NexStreamError::ConnectionClosed);
        }
        self.mux.recv(stream_id)
    }

    /// Close the connection gracefully.
    pub fn close(&mut self) -> Result<()> {
        match self.state {
            ConnectionState::Open => {
                self.state = ConnectionState::Closing;
                // In a real implementation we would send CONN_CLOSE and wait.
                self.state = ConnectionState::Closed;
                Ok(())
            }
            ConnectionState::Closing | ConnectionState::Closed => Ok(()),
            _ => Err(NexStreamError::InvalidStateTransition {
                from: self.state.to_string(),
                to: "Closing".into(),
            }),
        }
    }

    /// Returns the current connection state.
    pub fn state(&self) -> ConnectionState {
        self.state
    }

    /// Returns a reference to the RTT estimator.
    pub fn rtt(&self) -> &RttEstimator {
        &self.rtt
    }

    /// Returns a mutable reference to the RTT estimator.
    pub fn rtt_mut(&mut self) -> &mut RttEstimator {
        &mut self.rtt
    }

    /// Returns a reference to the congestion controller.
    pub fn congestion(&self) -> &dyn CongestionController {
        self.congestion.as_ref()
    }

    /// Returns a mutable reference to the congestion controller.
    pub fn congestion_mut(&mut self) -> &mut dyn CongestionController {
        self.congestion.as_mut()
    }

    /// Returns a reference to the loss detector.
    pub fn loss_detector(&self) -> &LossDetector {
        &self.loss_detector
    }

    /// Returns a mutable reference to the loss detector.
    pub fn loss_detector_mut(&mut self) -> &mut LossDetector {
        &mut self.loss_detector
    }

    /// Returns a reference to the flow controller.
    pub fn flow_control(&self) -> &FlowController {
        &self.flow_control
    }

    /// Returns a mutable reference to the flow controller.
    pub fn flow_control_mut(&mut self) -> &mut FlowController {
        &mut self.flow_control
    }

    /// Returns a reference to the multiplexer.
    pub fn mux(&self) -> &Multiplexer {
        &self.mux
    }

    /// Returns a mutable reference to the multiplexer.
    pub fn mux_mut(&mut self) -> &mut Multiplexer {
        &mut self.mux
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn connection_lifecycle() {
        let mut conn = Connection::new(ConnectionConfig::default());
        assert_eq!(conn.state(), ConnectionState::Idle);

        conn.connect().unwrap();
        assert_eq!(conn.state(), ConnectionState::Connecting);

        conn.accept().unwrap();
        assert_eq!(conn.state(), ConnectionState::Open);

        conn.close().unwrap();
        assert_eq!(conn.state(), ConnectionState::Closed);
    }

    #[test]
    fn cannot_open_stream_when_not_open() {
        let mut conn = Connection::new(ConnectionConfig::default());
        let result = conn.open_stream(TransportMode::BestEffort);
        assert!(result.is_err());
    }

    #[test]
    fn open_stream_and_send_recv() {
        let mut conn = Connection::new(ConnectionConfig::default());
        conn.connect().unwrap();
        conn.accept().unwrap();

        let sid = conn.open_stream(TransportMode::BestEffort).unwrap();
        conn.send(sid, Bytes::from_static(b"test")).unwrap();

        // Push some receive data via the mux layer.
        conn.mux_mut().get_stream_mut(sid).unwrap().push_recv(Bytes::from_static(b"reply"));
        let data = conn.recv(sid).unwrap().unwrap();
        assert_eq!(&data[..], b"reply");
    }

    #[test]
    fn close_is_idempotent() {
        let mut conn = Connection::new(ConnectionConfig::default());
        conn.connect().unwrap();
        conn.accept().unwrap();
        conn.close().unwrap();
        conn.close().unwrap(); // should not error
    }
}
