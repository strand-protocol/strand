//! Congestion control module.
//!
//! Defines the `CongestionController` trait and provides CUBIC as the default
//! algorithm.

pub mod cubic;

/// Trait for pluggable congestion control algorithms.
///
/// Implementations are responsible for tracking the congestion window and
/// bytes in flight, and for adjusting in response to ACKs and loss events.
pub trait CongestionController: Send {
    /// Notify the controller that `bytes` were sent.
    fn on_packet_sent(&mut self, bytes: usize);

    /// Notify the controller that `bytes` were acknowledged.
    fn on_ack(&mut self, bytes: usize);

    /// Notify the controller that `bytes` were declared lost.
    fn on_loss(&mut self, bytes: usize);

    /// Returns the current congestion window in bytes.
    fn window(&self) -> usize;

    /// Returns the number of bytes currently in flight.
    fn bytes_in_flight(&self) -> usize;

    /// Whether the controller allows sending `bytes` more data.
    fn can_send(&self, bytes: usize) -> bool {
        self.bytes_in_flight() + bytes <= self.window()
    }
}
