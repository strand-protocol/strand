//! Per-stream and per-connection flow control.
//!
//! Each stream has an independent send window. There is also a connection-level
//! window that caps the total across all streams.

use std::collections::HashMap;

use crate::error::{StrandStreamError, Result};

/// Default per-stream window: 64 KB.
const DEFAULT_STREAM_WINDOW: usize = 64 * 1024;
/// Default connection window: 1 MB.
const DEFAULT_CONNECTION_WINDOW: usize = 1024 * 1024;

/// Manages flow control windows for a connection and its streams.
pub struct FlowController {
    /// Per-stream available window (bytes).
    stream_windows: HashMap<u32, usize>,
    /// Connection-level available window (bytes).
    connection_window: usize,
    /// Default initial window for new streams.
    default_stream_window: usize,
}

impl FlowController {
    /// Create a new flow controller with default window sizes.
    pub fn new() -> Self {
        Self {
            stream_windows: HashMap::new(),
            connection_window: DEFAULT_CONNECTION_WINDOW,
            default_stream_window: DEFAULT_STREAM_WINDOW,
        }
    }

    /// Create with custom connection and per-stream defaults.
    pub fn with_windows(connection_window: usize, default_stream_window: usize) -> Self {
        Self {
            stream_windows: HashMap::new(),
            connection_window,
            default_stream_window,
        }
    }

    /// Register a stream with its initial window.
    pub fn add_stream(&mut self, stream_id: u32) {
        self.stream_windows
            .entry(stream_id)
            .or_insert(self.default_stream_window);
    }

    /// Remove a stream from tracking.
    pub fn remove_stream(&mut self, stream_id: u32) {
        self.stream_windows.remove(&stream_id);
    }

    /// Update (increase or decrease) a stream's window by `delta` bytes.
    pub fn update_window(&mut self, stream_id: u32, delta: isize) -> Result<()> {
        let w = self
            .stream_windows
            .get_mut(&stream_id)
            .ok_or(StrandStreamError::StreamNotFound(stream_id))?;
        let new_val = (*w as isize + delta).max(0) as usize;
        *w = new_val;
        Ok(())
    }

    /// Update the connection-level window by `delta` bytes.
    pub fn update_connection_window(&mut self, delta: isize) {
        self.connection_window = (self.connection_window as isize + delta).max(0) as usize;
    }

    /// Returns the number of bytes available to send on the given stream,
    /// taking into account both the stream window and the connection window.
    pub fn available(&self, stream_id: u32) -> usize {
        let stream_avail = self
            .stream_windows
            .get(&stream_id)
            .copied()
            .unwrap_or(0);
        std::cmp::min(stream_avail, self.connection_window)
    }

    /// Consume `bytes` from both the stream and connection windows.
    ///
    /// Returns an error if there are not enough bytes available.
    pub fn consume(&mut self, stream_id: u32, bytes: usize) -> Result<()> {
        let avail = self.available(stream_id);
        if bytes > avail {
            return Err(StrandStreamError::FlowControlBlocked(stream_id));
        }

        if let Some(w) = self.stream_windows.get_mut(&stream_id) {
            *w = w.saturating_sub(bytes);
        }
        self.connection_window = self.connection_window.saturating_sub(bytes);
        Ok(())
    }

    /// Release `bytes` back to both the stream and connection windows
    /// (e.g., when the receiver acknowledges data).
    pub fn release(&mut self, stream_id: u32, bytes: usize) {
        if let Some(w) = self.stream_windows.get_mut(&stream_id) {
            *w += bytes;
        }
        self.connection_window += bytes;
    }

    /// Returns the connection-level available window.
    pub fn connection_available(&self) -> usize {
        self.connection_window
    }
}

impl Default for FlowController {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn stream_lifecycle() {
        let mut fc = FlowController::new();
        fc.add_stream(1);
        assert_eq!(fc.available(1), DEFAULT_STREAM_WINDOW);
        fc.remove_stream(1);
        assert_eq!(fc.available(1), 0);
    }

    #[test]
    fn consume_and_release() {
        let mut fc = FlowController::new();
        fc.add_stream(1);

        let initial = fc.available(1);
        fc.consume(1, 1000).unwrap();
        assert_eq!(fc.available(1), initial - 1000);

        fc.release(1, 500);
        assert_eq!(fc.available(1), initial - 500);
    }

    #[test]
    fn consume_exceeds_window() {
        let mut fc = FlowController::with_windows(100, 50);
        fc.add_stream(1);
        let result = fc.consume(1, 51);
        assert!(result.is_err());
    }

    #[test]
    fn connection_window_limits_stream() {
        let mut fc = FlowController::with_windows(100, 200);
        fc.add_stream(1);
        // Stream window is 200, but connection window is 100.
        assert_eq!(fc.available(1), 100);
    }

    #[test]
    fn update_window() {
        let mut fc = FlowController::new();
        fc.add_stream(1);
        let before = fc.available(1);
        fc.update_window(1, 1024).unwrap();
        // Stream window increased, but connection window still limits.
        let expected = std::cmp::min(
            DEFAULT_STREAM_WINDOW + 1024,
            DEFAULT_CONNECTION_WINDOW,
        );
        assert_eq!(fc.available(1), expected);
        assert!(fc.available(1) >= before);
    }
}
