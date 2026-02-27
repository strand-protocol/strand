//! NexStream -- Layer 3 Hybrid Transport Protocol for the Nexus Protocol stack.
//!
//! Provides four delivery modes multiplexed over a single connection:
//! - **Reliable-Ordered**: TCP-equivalent in-order, exactly-once delivery
//! - **Reliable-Unordered**: exactly-once delivery without ordering
//! - **Best-Effort**: fire-and-forget, no guarantees
//! - **Probabilistic**: configurable delivery probability

pub mod congestion;
pub mod connection;
pub mod error;
pub mod flow_control;
pub mod frame;
pub mod loss_detection;
pub mod mux;
pub mod retransmission;
pub mod rtt;
pub mod stream;
pub mod transport;

// Re-export key public types at crate root.
pub use connection::{Connection, ConnectionConfig, ConnectionState};
pub use error::{NexStreamError, Result};
pub use flow_control::FlowController;
pub use frame::Frame;
pub use mux::Multiplexer;
pub use rtt::RttEstimator;
pub use stream::{Stream, StreamState};
pub use transport::TransportMode;
