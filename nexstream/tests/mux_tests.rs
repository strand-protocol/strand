//! Multiplexer tests: multiple streams.

use bytes::Bytes;
use nexstream::frame::{DataFlags, Frame};
use nexstream::mux::Multiplexer;
use nexstream::stream::StreamState;
use nexstream::transport::TransportMode;

#[test]
fn create_multiple_streams() {
    let mut mux = Multiplexer::new(100);
    let s1 = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
    let s2 = mux.create_stream(TransportMode::ReliableUnordered).unwrap();
    let s3 = mux.create_stream(TransportMode::BestEffort).unwrap();
    let s4 = mux.create_stream(TransportMode::Probabilistic).unwrap();

    // Stream IDs should be unique odd numbers.
    assert_eq!(s1, 1);
    assert_eq!(s2, 3);
    assert_eq!(s3, 5);
    assert_eq!(s4, 7);

    assert_eq!(mux.active_stream_count(), 4);
}

#[test]
fn send_and_recv_on_different_streams() {
    let mut mux = Multiplexer::new(100);
    let s1 = mux.create_stream(TransportMode::BestEffort).unwrap();
    let s2 = mux.create_stream(TransportMode::BestEffort).unwrap();

    mux.send(s1, Bytes::from_static(b"stream1-data")).unwrap();
    mux.send(s2, Bytes::from_static(b"stream2-data")).unwrap();

    // Drain send buffers.
    let d1 = mux.get_stream_mut(s1).unwrap().drain_send();
    assert_eq!(d1.len(), 1);
    assert_eq!(&d1[0][..], b"stream1-data");

    let d2 = mux.get_stream_mut(s2).unwrap().drain_send();
    assert_eq!(d2.len(), 1);
    assert_eq!(&d2[0][..], b"stream2-data");
}

#[test]
fn dispatch_data_to_correct_stream() {
    let mut mux = Multiplexer::new(100);
    let s1 = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
    let s2 = mux.create_stream(TransportMode::ReliableOrdered).unwrap();

    let frame1 = Frame::Data {
        stream_id: s1,
        seq: 0,
        flags: DataFlags::NONE,
        payload: Bytes::from_static(b"for-s1"),
    };
    let frame2 = Frame::Data {
        stream_id: s2,
        seq: 0,
        flags: DataFlags::NONE,
        payload: Bytes::from_static(b"for-s2"),
    };

    mux.poll(&frame1).unwrap();
    mux.poll(&frame2).unwrap();

    assert_eq!(&mux.recv(s1).unwrap().unwrap()[..], b"for-s1");
    assert_eq!(&mux.recv(s2).unwrap().unwrap()[..], b"for-s2");
}

#[test]
fn fin_closes_remote_side() {
    let mut mux = Multiplexer::new(100);
    let s = mux.create_stream(TransportMode::BestEffort).unwrap();

    let fin = Frame::Fin { stream_id: s };
    mux.poll(&fin).unwrap();

    let stream = mux.get_stream(s).unwrap();
    assert_eq!(stream.state(), StreamState::HalfClosedRemote);
}

#[test]
fn rst_removes_stream() {
    let mut mux = Multiplexer::new(100);
    let s = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
    assert_eq!(mux.stream_count(), 1);

    // Send some data.
    mux.send(s, Bytes::from_static(b"data")).unwrap();

    let rst = Frame::Rst {
        stream_id: s,
        error_code: 1,
    };
    mux.poll(&rst).unwrap();

    // RST removes the stream immediately to prevent HashMap exhaustion.
    assert_eq!(mux.stream_count(), 0);
    assert!(mux.get_stream(s).is_none());
}

#[test]
fn max_streams_limit() {
    let mut mux = Multiplexer::new(3);
    mux.create_stream(TransportMode::BestEffort).unwrap();
    mux.create_stream(TransportMode::BestEffort).unwrap();
    mux.create_stream(TransportMode::BestEffort).unwrap();

    let result = mux.create_stream(TransportMode::BestEffort);
    assert!(result.is_err());
}

#[test]
fn send_to_nonexistent_stream_fails() {
    let mut mux = Multiplexer::new(100);
    let result = mux.send(999, Bytes::from_static(b"fail"));
    assert!(result.is_err());
}

#[test]
fn recv_from_nonexistent_stream_fails() {
    let mut mux = Multiplexer::new(100);
    let result = mux.recv(999);
    assert!(result.is_err());
}

#[test]
fn dispatch_to_nonexistent_stream_fails() {
    let mut mux = Multiplexer::new(100);
    let frame = Frame::Data {
        stream_id: 999,
        seq: 0,
        flags: DataFlags::NONE,
        payload: Bytes::from_static(b"orphan"),
    };
    let result = mux.poll(&frame);
    assert!(result.is_err());
}

#[test]
fn close_then_fin_transitions_to_closed() {
    let mut mux = Multiplexer::new(100);
    let s = mux.create_stream(TransportMode::ReliableOrdered).unwrap();

    mux.close_stream(s).unwrap();
    assert_eq!(
        mux.get_stream(s).unwrap().state(),
        StreamState::HalfClosedLocal
    );

    let fin = Frame::Fin { stream_id: s };
    mux.poll(&fin).unwrap();
    assert_eq!(mux.get_stream(s).unwrap().state(), StreamState::Closed);
}

#[test]
fn stream_modes_preserved() {
    let mut mux = Multiplexer::new(100);
    let s1 = mux.create_stream(TransportMode::ReliableOrdered).unwrap();
    let s2 = mux.create_stream(TransportMode::Probabilistic).unwrap();

    assert_eq!(
        mux.get_stream(s1).unwrap().mode(),
        TransportMode::ReliableOrdered
    );
    assert_eq!(
        mux.get_stream(s2).unwrap().mode(),
        TransportMode::Probabilistic
    );
}
