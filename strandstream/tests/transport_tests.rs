//! Tests for each transport mode's delivery guarantees.

use bytes::Bytes;
use strandstream::frame::Frame;
use strandstream::transport::best_effort::{BestEffortReceiver, BestEffortSender};
use strandstream::transport::probabilistic::{ProbabilisticReceiver, ProbabilisticSender};
use strandstream::transport::reliable_ordered::{ReliableOrderedReceiver, ReliableOrderedSender};
use strandstream::transport::reliable_unordered::{
    ReliableUnorderedReceiver, ReliableUnorderedSender,
};
use strandstream::transport::{TransportReceiver, TransportSender};

// ---------------------------------------------------------------------------
// Reliable-Ordered
// ---------------------------------------------------------------------------

#[test]
fn ro_in_order_delivery_multiple() {
    let mut sender = ReliableOrderedSender::new();
    let mut receiver = ReliableOrderedReceiver::new();

    let mut frames = Vec::new();
    for i in 0..10 {
        let f = sender
            .send(1, Bytes::from(format!("msg-{i}")))
            .unwrap();
        frames.push(f.into_iter().next().unwrap());
    }

    // Deliver in order.
    for (i, f) in frames.iter().enumerate() {
        let d = receiver.receive(f).unwrap();
        assert_eq!(d.len(), 1, "frame {i} should deliver");
        assert_eq!(d[0], Bytes::from(format!("msg-{i}")));
    }
}

#[test]
fn ro_reorder_delivers_all_at_once() {
    let mut sender = ReliableOrderedSender::new();
    let mut receiver = ReliableOrderedReceiver::new();

    let f0 = sender.send(1, Bytes::from_static(b"0")).unwrap().remove(0);
    let f1 = sender.send(1, Bytes::from_static(b"1")).unwrap().remove(0);
    let f2 = sender.send(1, Bytes::from_static(b"2")).unwrap().remove(0);
    let f3 = sender.send(1, Bytes::from_static(b"3")).unwrap().remove(0);
    let f4 = sender.send(1, Bytes::from_static(b"4")).unwrap().remove(0);

    // Deliver: 4, 2, 3, 1, 0
    assert!(receiver.receive(&f4).unwrap().is_empty());
    assert!(receiver.receive(&f2).unwrap().is_empty());
    assert!(receiver.receive(&f3).unwrap().is_empty());
    assert!(receiver.receive(&f1).unwrap().is_empty());

    let d = receiver.receive(&f0).unwrap();
    assert_eq!(d.len(), 5);
    for (i, data) in d.iter().enumerate() {
        assert_eq!(data[0], b'0' + i as u8);
    }
}

#[test]
fn ro_retransmit_returns_unacked() {
    let mut sender = ReliableOrderedSender::new();
    sender.send(1, Bytes::from_static(b"a")).unwrap();
    sender.send(1, Bytes::from_static(b"b")).unwrap();

    let retx = sender.retransmit();
    assert_eq!(retx.len(), 2);

    sender.on_ack(0);
    let retx = sender.retransmit();
    assert_eq!(retx.len(), 1);
}

#[test]
fn ro_duplicate_ignored() {
    let mut sender = ReliableOrderedSender::new();
    let mut receiver = ReliableOrderedReceiver::new();

    let f = sender.send(1, Bytes::from_static(b"X")).unwrap().remove(0);
    let d1 = receiver.receive(&f).unwrap();
    assert_eq!(d1.len(), 1);

    // Deliver same frame again -> no new data.
    let d2 = receiver.receive(&f).unwrap();
    assert!(d2.is_empty());
}

// ---------------------------------------------------------------------------
// Reliable-Unordered
// ---------------------------------------------------------------------------

#[test]
fn ru_delivers_immediately_regardless_of_order() {
    let mut sender = ReliableUnorderedSender::new();
    let mut receiver = ReliableUnorderedReceiver::new();

    let f0 = sender.send(1, Bytes::from_static(b"A")).unwrap().remove(0);
    let f1 = sender.send(1, Bytes::from_static(b"B")).unwrap().remove(0);
    let f2 = sender.send(1, Bytes::from_static(b"C")).unwrap().remove(0);

    // Deliver out of order: 2, 0, 1
    let d = receiver.receive(&f2).unwrap();
    assert_eq!(d.len(), 1);
    assert_eq!(&d[0][..], b"C");

    let d = receiver.receive(&f0).unwrap();
    assert_eq!(d.len(), 1);
    assert_eq!(&d[0][..], b"A");

    let d = receiver.receive(&f1).unwrap();
    assert_eq!(d.len(), 1);
    assert_eq!(&d[0][..], b"B");
}

#[test]
fn ru_exactly_once_dedup() {
    let mut sender = ReliableUnorderedSender::new();
    let mut receiver = ReliableUnorderedReceiver::new();

    let f = sender.send(1, Bytes::from_static(b"once")).unwrap().remove(0);
    assert_eq!(receiver.receive(&f).unwrap().len(), 1);
    assert_eq!(receiver.receive(&f).unwrap().len(), 0); // duplicate
    assert_eq!(receiver.receive(&f).unwrap().len(), 0); // triple
}

#[test]
fn ru_retransmit_tracks_unacked() {
    let mut sender = ReliableUnorderedSender::new();
    sender.send(1, Bytes::from_static(b"X")).unwrap();
    sender.send(1, Bytes::from_static(b"Y")).unwrap();
    assert_eq!(sender.in_flight(), 2);
    assert_eq!(sender.retransmit().len(), 2);

    sender.on_ack(0);
    assert_eq!(sender.in_flight(), 1);
    assert_eq!(sender.retransmit().len(), 1);
}

// ---------------------------------------------------------------------------
// Best-Effort
// ---------------------------------------------------------------------------

#[test]
fn be_fire_and_forget_delivery() {
    let mut sender = BestEffortSender::new();
    let mut receiver = BestEffortReceiver::new();

    for i in 0..10 {
        let f = sender
            .send(1, Bytes::from(format!("pkt-{i}")))
            .unwrap()
            .remove(0);
        let d = receiver.receive(&f).unwrap();
        assert_eq!(d.len(), 1);
    }
}

#[test]
fn be_no_retransmission() {
    let mut sender = BestEffortSender::new();
    sender.send(1, Bytes::from_static(b"gone")).unwrap();
    assert!(sender.retransmit().is_empty());
}

#[test]
fn be_ack_is_noop() {
    let mut sender = BestEffortSender::new();
    sender.send(1, Bytes::from_static(b"data")).unwrap();
    sender.on_ack(0); // should not panic
}

// ---------------------------------------------------------------------------
// Probabilistic
// ---------------------------------------------------------------------------

#[test]
fn pr_probability_one_delivers_all() {
    let mut sender = ProbabilisticSender::new();
    let mut receiver = ProbabilisticReceiver::new(1.0);

    for _ in 0..50 {
        let f = sender.send(1, Bytes::from_static(b"d")).unwrap().remove(0);
        let d = receiver.receive(&f).unwrap();
        assert_eq!(d.len(), 1);
    }
}

#[test]
fn pr_probability_zero_drops_all() {
    let mut sender = ProbabilisticSender::new();
    let mut receiver = ProbabilisticReceiver::new(0.0);

    for _ in 0..50 {
        let f = sender.send(1, Bytes::from_static(b"d")).unwrap().remove(0);
        let d = receiver.receive(&f).unwrap();
        assert!(d.is_empty());
    }
}

#[test]
fn pr_no_retransmission() {
    let mut sender = ProbabilisticSender::new();
    sender.send(1, Bytes::from_static(b"x")).unwrap();
    assert!(sender.retransmit().is_empty());
}

#[test]
fn pr_receiver_rejects_non_data_frame() {
    let mut receiver = ProbabilisticReceiver::new(1.0);
    let frame = Frame::Fin { stream_id: 1 };
    let result = receiver.receive(&frame);
    assert!(result.is_err());
}

// ---------------------------------------------------------------------------
// Cross-mode: ensure non-data frames are rejected
// ---------------------------------------------------------------------------

#[test]
fn non_data_frame_rejected_by_all_receivers() {
    let fin = Frame::Fin { stream_id: 1 };

    assert!(ReliableOrderedReceiver::new().receive(&fin).is_err());
    assert!(ReliableUnorderedReceiver::new().receive(&fin).is_err());
    assert!(BestEffortReceiver::new().receive(&fin).is_err());
    assert!(ProbabilisticReceiver::new(1.0).receive(&fin).is_err());
}
