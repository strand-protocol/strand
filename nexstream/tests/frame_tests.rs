//! Frame encode/decode round-trip tests.

use bytes::Bytes;
use nexstream::frame::{DataFlags, Frame, FrameType, SeqRange};

#[test]
fn data_frame_roundtrip() {
    let frame = Frame::Data {
        stream_id: 42,
        seq: 7,
        flags: DataFlags::NONE,
        payload: Bytes::from_static(b"hello world"),
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn data_frame_with_fin_flag() {
    let frame = Frame::Data {
        stream_id: 1,
        seq: 100,
        flags: DataFlags::FIN,
        payload: Bytes::from_static(b"last chunk"),
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
    if let Frame::Data { flags, .. } = &decoded {
        assert!(flags.contains(DataFlags::FIN));
    }
}

#[test]
fn data_frame_empty_payload() {
    let frame = Frame::Data {
        stream_id: 0,
        seq: 0,
        flags: DataFlags::NONE,
        payload: Bytes::new(),
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn ack_frame_roundtrip() {
    let frame = Frame::Ack {
        stream_id: 5,
        ack_seq: 99,
        ranges: vec![
            SeqRange { start: 10, end: 20 },
            SeqRange { start: 30, end: 40 },
        ],
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn ack_frame_empty_ranges() {
    let frame = Frame::Ack {
        stream_id: 1,
        ack_seq: 0,
        ranges: vec![],
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn nack_frame_roundtrip() {
    let frame = Frame::Nack {
        stream_id: 3,
        ranges: vec![SeqRange {
            start: 100,
            end: 200,
        }],
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn fin_frame_roundtrip() {
    let frame = Frame::Fin { stream_id: 77 };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn rst_frame_roundtrip() {
    let frame = Frame::Rst {
        stream_id: 12,
        error_code: 0xDEAD,
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn ping_pong_roundtrip() {
    let ping = Frame::Ping { ping_id: 12345678 };
    let encoded = ping.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(ping, decoded);

    let pong = Frame::Pong {
        ping_id: 0xFFFF_FFFF_FFFF_FFFF,
    };
    let encoded = pong.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(pong, decoded);
}

#[test]
fn window_update_roundtrip() {
    let frame = Frame::WindowUpdate {
        stream_id: 9,
        window_increment: 65536,
    };
    let encoded = frame.encode();
    let decoded = Frame::decode(&encoded).unwrap();
    assert_eq!(frame, decoded);
}

#[test]
fn frame_type_discriminant() {
    assert_eq!(
        Frame::Data {
            stream_id: 0,
            seq: 0,
            flags: DataFlags::NONE,
            payload: Bytes::new()
        }
        .frame_type(),
        FrameType::Data
    );
    assert_eq!(
        Frame::Ack {
            stream_id: 0,
            ack_seq: 0,
            ranges: vec![]
        }
        .frame_type(),
        FrameType::Ack
    );
    assert_eq!(Frame::Fin { stream_id: 0 }.frame_type(), FrameType::Fin);
}

#[test]
fn decode_empty_buffer_fails() {
    let result = Frame::decode(&[]);
    assert!(result.is_err());
}

#[test]
fn decode_unknown_type_fails() {
    let result = Frame::decode(&[0xFF]);
    assert!(result.is_err());
}

#[test]
fn decode_truncated_data_frame_fails() {
    // Type byte + only 4 bytes (need at least 13).
    let result = Frame::decode(&[0x01, 0, 0, 0, 1]);
    assert!(result.is_err());
}

#[test]
fn encoded_len_matches_encode() {
    let frames: Vec<Frame> = vec![
        Frame::Data {
            stream_id: 1,
            seq: 2,
            flags: DataFlags::KEY_FRAME,
            payload: Bytes::from_static(b"test data"),
        },
        Frame::Ack {
            stream_id: 1,
            ack_seq: 2,
            ranges: vec![SeqRange { start: 0, end: 1 }],
        },
        Frame::Nack {
            stream_id: 1,
            ranges: vec![],
        },
        Frame::Fin { stream_id: 1 },
        Frame::Rst {
            stream_id: 1,
            error_code: 0,
        },
        Frame::Ping { ping_id: 42 },
        Frame::Pong { ping_id: 42 },
        Frame::WindowUpdate {
            stream_id: 1,
            window_increment: 1024,
        },
    ];

    for frame in &frames {
        let encoded = frame.encode();
        assert_eq!(
            encoded.len(),
            frame.encoded_len(),
            "encoded_len mismatch for {:?}",
            frame.frame_type()
        );
    }
}
