use thiserror::Error;

/// All errors produced by the NexStream transport layer.
#[derive(Debug, Error)]
pub enum NexStreamError {
    #[error("frame too short: expected at least {expected} bytes, got {actual}")]
    FrameTooShort { expected: usize, actual: usize },

    #[error("unknown frame type: 0x{0:02x}")]
    UnknownFrameType(u8),

    #[error("invalid transport mode: 0x{0:02x}")]
    InvalidTransportMode(u8),

    #[error("stream {0} not found")]
    StreamNotFound(u32),

    #[error("stream {0} already exists")]
    StreamAlreadyExists(u32),

    #[error("stream {0} is closed")]
    StreamClosed(u32),

    #[error("connection is closed")]
    ConnectionClosed,

    #[error("connection timeout")]
    ConnectionTimeout,

    #[error("maximum streams ({0}) exceeded")]
    MaxStreamsExceeded(u32),

    #[error("maximum retransmissions ({0}) exceeded for stream {1}")]
    MaxRetransmissionsExceeded(u32, u32),

    #[error("flow control window exhausted for stream {0}")]
    FlowControlBlocked(u32),

    #[error("invalid state transition from {from} to {to}")]
    InvalidStateTransition { from: String, to: String },

    #[error("payload too large: {size} bytes exceeds maximum {max}")]
    PayloadTooLarge { size: usize, max: usize },

    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

    #[error("internal error: {0}")]
    Internal(String),
}

pub type Result<T> = std::result::Result<T, NexStreamError>;
