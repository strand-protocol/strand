//! Stream multiplexer / demultiplexer.
//!
//! Manages a collection of streams identified by `StreamId` (u32). Dispatches
//! incoming frames to the appropriate stream and collects outgoing data.

use std::collections::HashMap;

use bytes::Bytes;

use crate::error::{NexStreamError, Result};
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
            return Err(NexStreamError::MaxStreamsExceeded(self.max_streams));
        }

        let id = self.next_client_stream_id;
        self.next_client_stream_id = self.next_client_stream_id.wrapping_add(2); // odd IDs

        let mut stream = Stream::new(id, mode);
        stream.open()?;
        self.streams.insert(id, stream);
        Ok(id)
    }

    /// Queue data for sending on the given stream.
    pub fn send(&mut self, stream_id: StreamId, data: Bytes) -> Result<()> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(NexStreamError::StreamNotFound(stream_id))?;
        stream.send(data)
    }

    /// Receive data from the given stream (returns None if no data available).
    pub fn recv(&mut self, stream_id: StreamId) -> Result<Option<Bytes>> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(NexStreamError::StreamNotFound(stream_id))?;
        stream.recv()
    }

    /// Close a stream.
    pub fn close_stream(&mut self, stream_id: StreamId) -> Result<()> {
        let stream = self
            .streams
            .get_mut(&stream_id)
            .ok_or(NexStreamError::StreamNotFound(stream_id))?;
        stream.close()
    }

    /// Dispatch an incoming frame to the appropriate stream.
    ///
    /// For DATA frames, pushes the payload into the stream's receive buffer.
    /// For FIN frames, marks the remote side as closed.
    /// For RST frames, resets the stream.
    pub fn poll(&mut self, frame: &Frame) -> Result<()> {
        match frame {
            Frame::Data {
                stream_id,
                payload,
                ..
            } => {
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(NexStreamError::StreamNotFound(*stream_id))?;
                stream.push_recv(payload.clone());
                Ok(())
            }
            Frame::Fin { stream_id } => {
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(NexStreamError::StreamNotFound(*stream_id))?;
                stream.remote_close();
                Ok(())
            }
            Frame::Rst { stream_id, .. } => {
                let stream = self
                    .streams
                    .get_mut(stream_id)
                    .ok_or(NexStreamError::StreamNotFound(*stream_id))?;
                stream.reset();
                Ok(())
            }
            _ => {
                // Other frame types (ACK, NACK, Ping, Pong, WindowUpdate)
                // are handled by the connection layer, not the mux.
                Ok(())
            }
        }
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
    fn rst_resets_stream() {
        let mut mux = Multiplexer::new(100);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();

        let frame = Frame::Rst {
            stream_id: sid,
            error_code: 42,
        };
        mux.poll(&frame).unwrap();

        let stream = mux.get_stream(sid).unwrap();
        assert_eq!(stream.state(), StreamState::Closed);
    }
}
