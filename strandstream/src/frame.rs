use bytes::{Buf, BufMut, Bytes, BytesMut};

use crate::error::{NexStreamError, Result};

/// Frame type identifiers carried inside NexStream.
///
/// Values 0x01–0x08 are data-path frames. Values 0x10–0x13 are connection
/// lifecycle control frames. 0x40 is the congestion-signalling frame.
/// All wire values match the spec (§4.3 NexStream Control Frame Types).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u8)]
pub enum FrameType {
    // Data-path frames (0x01–0x08)
    Data = 0x01,
    Ack = 0x02,
    Nack = 0x03,
    Fin = 0x04,
    Rst = 0x05,
    Ping = 0x06,
    Pong = 0x07,
    WindowUpdate = 0x08,
    // Connection lifecycle control frames (0x10–0x13)
    StreamOpen = 0x10,
    StreamAck = 0x11,
    StreamClose = 0x12,
    StreamReset = 0x13,
    // Congestion-signalling frame (0x40)
    Congestion = 0x40,
}

impl TryFrom<u8> for FrameType {
    type Error = NexStreamError;

    fn try_from(value: u8) -> Result<Self> {
        match value {
            0x01 => Ok(FrameType::Data),
            0x02 => Ok(FrameType::Ack),
            0x03 => Ok(FrameType::Nack),
            0x04 => Ok(FrameType::Fin),
            0x05 => Ok(FrameType::Rst),
            0x06 => Ok(FrameType::Ping),
            0x07 => Ok(FrameType::Pong),
            0x08 => Ok(FrameType::WindowUpdate),
            0x10 => Ok(FrameType::StreamOpen),
            0x11 => Ok(FrameType::StreamAck),
            0x12 => Ok(FrameType::StreamClose),
            0x13 => Ok(FrameType::StreamReset),
            0x40 => Ok(FrameType::Congestion),
            other => Err(NexStreamError::UnknownFrameType(other)),
        }
    }
}

/// Flags carried in DATA frames.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct DataFlags(pub u8);

impl DataFlags {
    pub const NONE: Self = Self(0x00);
    pub const FIN: Self = Self(0x01);
    pub const KEY_FRAME: Self = Self(0x02);

    pub fn contains(self, flag: DataFlags) -> bool {
        (self.0 & flag.0) == flag.0
    }
}

/// A range of sequence numbers used in NACK frames (selective negative ACK).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct SeqRange {
    pub start: u32,
    pub end: u32,
}

/// NexStream wire frame.
///
/// Binary layout (all fields big-endian):
///
/// ```text
/// +-------+----------+--- variable ---+
/// | type  |  ... fields per type ...  |
/// | (1B)  |                            |
/// +-------+----------------------------+
/// ```
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Frame {
    /// DATA: stream_id(4) + seq(4) + flags(1) + payload_len(4) + payload(N)
    Data {
        stream_id: u32,
        seq: u32,
        flags: DataFlags,
        payload: Bytes,
    },
    /// ACK: stream_id(4) + ack_seq(4) + range_count(2) + ranges(8*N)
    Ack {
        stream_id: u32,
        ack_seq: u32,
        ranges: Vec<SeqRange>,
    },
    /// NACK: stream_id(4) + range_count(2) + ranges(8*N)
    Nack {
        stream_id: u32,
        ranges: Vec<SeqRange>,
    },
    /// FIN: stream_id(4)
    Fin {
        stream_id: u32,
    },
    /// RST: stream_id(4) + error_code(4)
    Rst {
        stream_id: u32,
        error_code: u32,
    },
    /// PING: ping_id(8)
    Ping {
        ping_id: u64,
    },
    /// PONG: ping_id(8)
    Pong {
        ping_id: u64,
    },
    /// WINDOW_UPDATE: stream_id(4) + window_increment(4)
    WindowUpdate {
        stream_id: u32,
        window_increment: u32,
    },
    /// STREAM_OPEN: stream_id(4) + transport_mode(1)
    StreamOpen {
        stream_id: u32,
        transport_mode: u8,
    },
    /// STREAM_ACK: stream_id(4)
    StreamAck {
        stream_id: u32,
    },
    /// STREAM_CLOSE: stream_id(4)
    StreamClose {
        stream_id: u32,
    },
    /// STREAM_RESET: stream_id(4) + error_code(4)
    StreamReset {
        stream_id: u32,
        error_code: u32,
    },
    /// CONGESTION: stream_id(4) + cwnd(4) + rtt_us(4)
    Congestion {
        stream_id: u32,
        cwnd: u32,
        rtt_us: u32,
    },
}

impl Frame {
    /// Return the frame type discriminant.
    pub fn frame_type(&self) -> FrameType {
        match self {
            Frame::Data { .. } => FrameType::Data,
            Frame::Ack { .. } => FrameType::Ack,
            Frame::Nack { .. } => FrameType::Nack,
            Frame::Fin { .. } => FrameType::Fin,
            Frame::Rst { .. } => FrameType::Rst,
            Frame::Ping { .. } => FrameType::Ping,
            Frame::Pong { .. } => FrameType::Pong,
            Frame::WindowUpdate { .. } => FrameType::WindowUpdate,
            Frame::StreamOpen { .. } => FrameType::StreamOpen,
            Frame::StreamAck { .. } => FrameType::StreamAck,
            Frame::StreamClose { .. } => FrameType::StreamClose,
            Frame::StreamReset { .. } => FrameType::StreamReset,
            Frame::Congestion { .. } => FrameType::Congestion,
        }
    }

    /// Encode this frame into a byte buffer.
    pub fn encode(&self) -> Bytes {
        let mut buf = BytesMut::with_capacity(self.encoded_len());
        self.encode_into(&mut buf);
        buf.freeze()
    }

    /// Encode into a pre-allocated `BytesMut`.
    pub fn encode_into(&self, buf: &mut BytesMut) {
        match self {
            Frame::Data {
                stream_id,
                seq,
                flags,
                payload,
            } => {
                buf.put_u8(FrameType::Data as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*seq);
                buf.put_u8(flags.0);
                buf.put_u32(payload.len() as u32);
                buf.put_slice(payload);
            }
            Frame::Ack {
                stream_id,
                ack_seq,
                ranges,
            } => {
                buf.put_u8(FrameType::Ack as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*ack_seq);
                buf.put_u16(ranges.len() as u16);
                for r in ranges {
                    buf.put_u32(r.start);
                    buf.put_u32(r.end);
                }
            }
            Frame::Nack { stream_id, ranges } => {
                buf.put_u8(FrameType::Nack as u8);
                buf.put_u32(*stream_id);
                buf.put_u16(ranges.len() as u16);
                for r in ranges {
                    buf.put_u32(r.start);
                    buf.put_u32(r.end);
                }
            }
            Frame::Fin { stream_id } => {
                buf.put_u8(FrameType::Fin as u8);
                buf.put_u32(*stream_id);
            }
            Frame::Rst {
                stream_id,
                error_code,
            } => {
                buf.put_u8(FrameType::Rst as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*error_code);
            }
            Frame::Ping { ping_id } => {
                buf.put_u8(FrameType::Ping as u8);
                buf.put_u64(*ping_id);
            }
            Frame::Pong { ping_id } => {
                buf.put_u8(FrameType::Pong as u8);
                buf.put_u64(*ping_id);
            }
            Frame::WindowUpdate {
                stream_id,
                window_increment,
            } => {
                buf.put_u8(FrameType::WindowUpdate as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*window_increment);
            }
            Frame::StreamOpen {
                stream_id,
                transport_mode,
            } => {
                buf.put_u8(FrameType::StreamOpen as u8);
                buf.put_u32(*stream_id);
                buf.put_u8(*transport_mode);
            }
            Frame::StreamAck { stream_id } => {
                buf.put_u8(FrameType::StreamAck as u8);
                buf.put_u32(*stream_id);
            }
            Frame::StreamClose { stream_id } => {
                buf.put_u8(FrameType::StreamClose as u8);
                buf.put_u32(*stream_id);
            }
            Frame::StreamReset {
                stream_id,
                error_code,
            } => {
                buf.put_u8(FrameType::StreamReset as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*error_code);
            }
            Frame::Congestion {
                stream_id,
                cwnd,
                rtt_us,
            } => {
                buf.put_u8(FrameType::Congestion as u8);
                buf.put_u32(*stream_id);
                buf.put_u32(*cwnd);
                buf.put_u32(*rtt_us);
            }
        }
    }

    /// The total number of bytes this frame will occupy when encoded.
    pub fn encoded_len(&self) -> usize {
        // 1 byte for type tag in every variant
        1 + match self {
            Frame::Data { payload, .. } => 4 + 4 + 1 + 4 + payload.len(),
            Frame::Ack { ranges, .. } => 4 + 4 + 2 + ranges.len() * 8,
            Frame::Nack { ranges, .. } => 4 + 2 + ranges.len() * 8,
            Frame::Fin { .. } => 4,
            Frame::Rst { .. } => 4 + 4,
            Frame::Ping { .. } => 8,
            Frame::Pong { .. } => 8,
            Frame::WindowUpdate { .. } => 4 + 4,
            Frame::StreamOpen { .. } => 4 + 1,
            Frame::StreamAck { .. } => 4,
            Frame::StreamClose { .. } => 4,
            Frame::StreamReset { .. } => 4 + 4,
            Frame::Congestion { .. } => 4 + 4 + 4,
        }
    }

    /// Decode a frame from the given byte buffer.
    pub fn decode(mut data: &[u8]) -> Result<Self> {
        if data.is_empty() {
            return Err(NexStreamError::FrameTooShort {
                expected: 1,
                actual: 0,
            });
        }

        let frame_type = FrameType::try_from(data[0])?;
        data = &data[1..];

        match frame_type {
            FrameType::Data => {
                Self::ensure_len(data, 13, "DATA")?; // 4+4+1+4
                let stream_id = (&data[0..4]).get_u32();
                let seq = (&data[4..8]).get_u32();
                let flags = DataFlags(data[8]);
                let payload_len = (&data[9..13]).get_u32() as usize;
                let data = &data[13..];
                Self::ensure_len(data, payload_len, "DATA payload")?;
                let payload = Bytes::copy_from_slice(&data[..payload_len]);
                Ok(Frame::Data {
                    stream_id,
                    seq,
                    flags,
                    payload,
                })
            }
            FrameType::Ack => {
                Self::ensure_len(data, 10, "ACK")?; // 4+4+2
                let stream_id = (&data[0..4]).get_u32();
                let ack_seq = (&data[4..8]).get_u32();
                let range_count = (&data[8..10]).get_u16() as usize;
                let data = &data[10..];
                Self::ensure_len(data, range_count * 8, "ACK ranges")?;
                let ranges = Self::decode_ranges(data, range_count);
                Ok(Frame::Ack {
                    stream_id,
                    ack_seq,
                    ranges,
                })
            }
            FrameType::Nack => {
                Self::ensure_len(data, 6, "NACK")?; // 4+2
                let stream_id = (&data[0..4]).get_u32();
                let range_count = (&data[4..6]).get_u16() as usize;
                let data = &data[6..];
                Self::ensure_len(data, range_count * 8, "NACK ranges")?;
                let ranges = Self::decode_ranges(data, range_count);
                Ok(Frame::Nack { stream_id, ranges })
            }
            FrameType::Fin => {
                Self::ensure_len(data, 4, "FIN")?;
                let stream_id = (&data[0..4]).get_u32();
                Ok(Frame::Fin { stream_id })
            }
            FrameType::Rst => {
                Self::ensure_len(data, 8, "RST")?;
                let stream_id = (&data[0..4]).get_u32();
                let error_code = (&data[4..8]).get_u32();
                Ok(Frame::Rst {
                    stream_id,
                    error_code,
                })
            }
            FrameType::Ping => {
                Self::ensure_len(data, 8, "PING")?;
                let ping_id = (&data[0..8]).get_u64();
                Ok(Frame::Ping { ping_id })
            }
            FrameType::Pong => {
                Self::ensure_len(data, 8, "PONG")?;
                let ping_id = (&data[0..8]).get_u64();
                Ok(Frame::Pong { ping_id })
            }
            FrameType::WindowUpdate => {
                Self::ensure_len(data, 8, "WINDOW_UPDATE")?;
                let stream_id = (&data[0..4]).get_u32();
                let window_increment = (&data[4..8]).get_u32();
                Ok(Frame::WindowUpdate {
                    stream_id,
                    window_increment,
                })
            }
            FrameType::StreamOpen => {
                Self::ensure_len(data, 5, "STREAM_OPEN")?;
                let stream_id = (&data[0..4]).get_u32();
                let transport_mode = data[4];
                Ok(Frame::StreamOpen {
                    stream_id,
                    transport_mode,
                })
            }
            FrameType::StreamAck => {
                Self::ensure_len(data, 4, "STREAM_ACK")?;
                let stream_id = (&data[0..4]).get_u32();
                Ok(Frame::StreamAck { stream_id })
            }
            FrameType::StreamClose => {
                Self::ensure_len(data, 4, "STREAM_CLOSE")?;
                let stream_id = (&data[0..4]).get_u32();
                Ok(Frame::StreamClose { stream_id })
            }
            FrameType::StreamReset => {
                Self::ensure_len(data, 8, "STREAM_RESET")?;
                let stream_id = (&data[0..4]).get_u32();
                let error_code = (&data[4..8]).get_u32();
                Ok(Frame::StreamReset {
                    stream_id,
                    error_code,
                })
            }
            FrameType::Congestion => {
                Self::ensure_len(data, 12, "CONGESTION")?;
                let stream_id = (&data[0..4]).get_u32();
                let cwnd = (&data[4..8]).get_u32();
                let rtt_us = (&data[8..12]).get_u32();
                Ok(Frame::Congestion {
                    stream_id,
                    cwnd,
                    rtt_us,
                })
            }
        }
    }

    fn ensure_len(data: &[u8], needed: usize, context: &str) -> Result<()> {
        if data.len() < needed {
            Err(NexStreamError::FrameTooShort {
                expected: needed,
                actual: data.len(),
            })
        } else {
            let _ = context;
            Ok(())
        }
    }

    fn decode_ranges(mut data: &[u8], count: usize) -> Vec<SeqRange> {
        let mut ranges = Vec::with_capacity(count);
        for _ in 0..count {
            let start = (&data[0..4]).get_u32();
            let end = (&data[4..8]).get_u32();
            data = &data[8..];
            ranges.push(SeqRange { start, end });
        }
        ranges
    }
}
