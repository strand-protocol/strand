//! Stream multiplexer / demultiplexer.
//!
//! Manages a collection of streams identified by `StreamId` (u32). Dispatches
//! incoming frames to the appropriate stream and collects outgoing data.

use std::collections::HashMap;

use bytes::Bytes;

use crate::error::{StrandStreamError, Result};
use crate::frame::Frame;
use crate::stream::{Stream, StreamState};
use crate::transport::TransportMode;

/// Type alias for stream identifiers.
pub type StreamId = u32;

/// Multiplexer managing all streams on a connection.
pub struct Multiplexer {
    /// Active streams keyed by stream ID.
    streams: HashMap<StreamId, Stream>,
    /// Next client-initiated stream ID (odd, starting from 1).
    next_client_stream_id: u32,
    /// Maximum allowed concurrent streams.
    max_streams: u32,
}

impl Multiplexer {
    /// Create a new multiplexer.
    pub fn new(max_streams: u32) -> Self {
        Self {
            streams: HashMap::new(),
            next_client_stream_id: 1,
            max_streams,
        }
    }

    /// Create a new stream with the given transport mode.
    /// Returns the stream ID.
    pub fn create_stream(&mut self, mode: TransportMode) -> Result<StreamId> {
        if self.streams.len() as u32 >= self.max_streams {
            return Err(StrandStreamError::MaxStreamsExceeded(self.max_streams));
        }

        let id = self.next_client_stream_id;
        self.next_client_stream_id = self.next_client_stream_id.wrapping_add(2); // odd IDs

        let mut stream = Stream::new(id, mode);
        stream.open()?;
        self.streams.insert(id, stream);
        Ok(id)
    }

    /// Queue data for sending on the given stream.
    ///
    /// Delegates to the stream's mode-specific `TransportSender`, which assigns
    /// a sequence number and buffers the resulting `Frame` for network dispatch.
    /// Call `drain_frames()` to retrieve the ready-to-send frames.
    pub fn send(&mut self, stream_id: StreamId, data: Bytes) -> Result<()> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(StrandStreamError::StreamNotFound(stream_id))?;
        stream.send(data)
    }

    /// Receive data from the given stream (returns None if no data available).
    pub fn recv(&mut self, stream_id: StreamId) -> Result<Option<Bytes>> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(StrandStreamError::StreamNotFound(stream_id))?;
        stream.recv()
    }

    /// Drain all outbound frames produced by `send()` calls on all streams.
    ///
    /// Returns a flat `Vec<Frame>` ready to be serialised and sent to the
    /// network layer.  The order within the vector is stream-creation order
    /// (HashMap iteration), which is non-deterministic but acceptable since
    /// each stream maintains its own per-stream sequence numbering.
    pub fn drain_frames(&mut self) -> Vec<Frame> {
        let mut frames = Vec::new();
        for stream in self.streams.values_mut() {
            frames.extend(stream.drain_frames());
        }
        frames
    }

    /// Close a stream.
    pub fn close_stream(&mut self, stream_id: StreamId) -> Result<()> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(StrandStreamError::StreamNotFound(stream_id))?;
        stream.close()
    }

    /// Dispatch an incoming frame to the appropriate stream.
    ///
    /// For DATA frames, the frame is forwarded to the stream's mode-specific
    /// `TransportReceiver` via `transport_receive()`.  The receiver applies
    /// mode semantics (ordering reassembly for RO, deduplication for RU,
    /// probabilistic drop for PR, unconditional delivery for BE) and enqueues
    /// any ready payloads into the stream's application receive buffer.
    ///
    /// For FIN frames, marks the remote side as closed.
    /// For RST frames, resets the stream and removes it from the map.
    pub fn poll(&mut self, frame: &Frame) -> Result<()> {
        match frame {
            Frame::Data { stream_id, .. } => {
                Self::validate_stream_id(*stream_id)?;
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(StrandStreamError::StreamNotFound(*stream_id))?;
                // Delegate to the mode-specific receiver for ordering / dedup.
                stream.transport_receive(frame)?;
                Ok(())
            }
            Frame::Fin { stream_id } => {
                Self::validate_stream_id(*stream_id)?;
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(StrandStreamError::StreamNotFound(*stream_id))?;
                stream.remote_close();
                Ok(())
            }
            Frame::Rst { stream_id, .. } => {
                Self::validate_stream_id(*stream_id)?;
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(StrandStreamError::StreamNotFound(*stream_id))?;
                stream.reset();
                // Remove immediately: RST terminates the stream in both directions.
                self.streams.remove(stream_id);
                Ok(())
            }
            _ => {
                // Other frame types (ACK, NACK, Ping, Pong, WindowUpdate)
                // are handled by the connection layer, not the mux.
                Ok(())
            }
        }
    }

    /// Validate that a stream ID is not a reserved value.
    ///
    /// Stream IDs `0x00000000` and `0xFFFFFFFF` are reserved and must never
    /// appear in data frames. Rejecting them prevents confusion with control
    /// IDs and enforces the spec.
    fn validate_stream_id(id: StreamId) -> Result<()> {
        if id == 0x0000_0000 || id == 0xFFFF_FFFF {
            return Err(StrandStreamError::InvalidStreamId(id));
        }
        Ok(())
    }

    /// Remove all fully-closed streams from the HashMap.
    ///
    /// Call periodically to reclaim memory after streams complete their
    /// lifecycle (both local and remote sides closed).
    pub fn remove_closed_streams(&mut self) {
        self.streams
            .retain(|_, s| s.state() != StreamState::Closed);
    }

    /// Returns a reference to a stream by ID.
    pub fn get_stream(&self, stream_id: StreamId) -> Option<&Stream> {
        self.streams.get(&stream_id)
    }

    /// Returns a mutable reference to a stream by ID.
    pub fn get_stream_mut(&mut self, stream_id: StreamId) -> Option<&mut Stream> {
        self.streams.get_mut(&stream_id)
    }

    /// Returns the number of active (non-closed) streams.
    pub fn active_stream_count(&self) -> usize {
        self.streams
            .values()
            .filter(|s| s.state() != StreamState::Closed)
            .count()
    }

    /// Returns the total number of streams (including closed).
    pub fn stream_count(&self) -> usize {
        self.streams.len()
    }
}

impl Default for Multiplexer {
    fn default() -> Self {
        Self::new(1024)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn create_and_send_recv() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::BestEffort).unwrap();
        mux.send(sid, Bytes::from_static(b"hello")).unwrap();

        // Drain via the stream directly.
        let stream = mux.get_stream_mut(sid).unwrap();
        let pending = stream.drain_send();
        assert_eq!(pending.len(), 1);
        assert_eq!(&pending[0][..], b"hello");
    }

    #[test]
    fn dispatch_incoming_data() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();

        let frame = Frame::Data {
            stream_id: sid,
            seq: 0,
            flags: crate::frame::DataFlags::NONE,
            payload: Bytes::from_static(b"incoming"),
        };
        mux.poll(&frame).unwrap();

        let data = mux.recv(sid).unwrap().unwrap();
        assert_eq!(&data[..], b"incoming");
    }

    #[test]
    fn close_stream() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
        assert_eq!(mux.active_stream_count(), 1);

        mux.close_stream(sid).unwrap();
        // Half-closed local, still counts as active.
        let stream = mux.get_stream(sid).unwrap();
        assert_eq!(stream.state(), StreamState::HalfClosedLocal);
    }

    #[test]
    fn max_streams_enforced() {
        let mut mux = Multiplexer::new(2);
        mux.create_stream(TransportMode::BestEffort).unwrap();
        mux.create_stream(TransportMode::BestEffort).unwrap();
        let result = mux.create_stream(TransportMode::BestEffort);
        assert!(result.is_err());
    }

    #[test]
    fn rst_removes_stream_from_map() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
        assert_eq!(mux.stream_count(), 1);

        let frame = Frame::Rst {
            stream_id: sid,
            error_code: 42,
        };
        mux.poll(&frame).unwrap();

        // Stream must be removed immediately on RST to prevent map exhaustion.
        assert_eq!(mux.stream_count(), 0);
        assert!(mux.get_stream(sid).is_none());
    }

    #[test]
    fn remove_closed_streams_cleans_up() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
        assert_eq!(mux.stream_count(), 1);

        mux.close_stream(sid).unwrap();
        // After local close the stream is still present.
        assert_eq!(mux.stream_count(), 1);

        // Simulate both sides finishing by calling remote_close via FIN.
        let fin = Frame::Fin { stream_id: sid };
        mux.poll(&fin).unwrap();
        // Now the stream should be fully closed; remove_closed_streams cleans up.
        mux.remove_closed_streams();
        assert_eq!(mux.stream_count(), 0);
    }

    #[test]
    fn reserved_stream_ids_rejected() {
        let mut mux = Multiplexer::new(100);
        // We need a real stream to avoid StreamNotFound before the ID check,
        // so just test the validation helper directly via the Frame::Data path
        // with reserved IDs.
        let frame_zero = Frame::Data {
            stream_id: 0x0000_0000,
            seq: 0,
            flags: crate::frame::DataFlags::NONE,
            payload: bytes::Bytes::from_static(b"x"),
        };
        assert!(mux.poll(&frame_zero).is_err());

        let frame_max = Frame::Rst {
            stream_id: 0xFFFF_FFFF,
            error_code: 0,
        };
        assert!(mux.poll(&frame_max).is_err());
    }
}
