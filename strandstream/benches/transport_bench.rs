// StrandStream transport-layer benchmarks using criterion.
//
// Measures:
//   - Frame encode / decode throughput
//   - CUBIC congestion window computation
//   - Multiplexer dispatch throughput

use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use std::time::Duration;

use bytes::Bytes;
use strandstream::congestion::cubic::Cubic;
use strandstream::congestion::CongestionController;
use strandstream::frame::{DataFlags, Frame, SeqRange};
use strandstream::mux::Multiplexer;
use strandstream::transport::TransportMode;

// ---------------------------------------------------------------------------
// Frame encode throughput
// ---------------------------------------------------------------------------

fn bench_frame_encode(c: &mut Criterion) {
    let sizes: &[usize] = &[64, 1024, 8192, 65536];

    let mut group = c.benchmark_group("frame_encode");
    for &size in sizes {
        let payload = Bytes::from(vec![0xABu8; size]);
        let frame = Frame::Data {
            stream_id: 1,
            seq: 42,
            flags: DataFlags::NONE,
            payload: payload.clone(),
        };
        group.throughput(Throughput::Bytes(size as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(format!("{size}B")),
            &frame,
            |b, f| {
                b.iter(|| {
                    black_box(f.encode());
                });
            },
        );
    }
    group.finish();
}

// ---------------------------------------------------------------------------
// Frame decode throughput
// ---------------------------------------------------------------------------

fn bench_frame_decode(c: &mut Criterion) {
    let sizes: &[usize] = &[64, 1024, 8192, 65536];

    let mut group = c.benchmark_group("frame_decode");
    for &size in sizes {
        let payload = Bytes::from(vec![0xABu8; size]);
        let frame = Frame::Data {
            stream_id: 1,
            seq: 42,
            flags: DataFlags::NONE,
            payload,
        };
        let encoded = frame.encode();
        group.throughput(Throughput::Bytes(encoded.len() as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(format!("{size}B")),
            &encoded,
            |b, data| {
                b.iter(|| {
                    black_box(Frame::decode(black_box(data)).unwrap());
                });
            },
        );
    }
    group.finish();
}

// ---------------------------------------------------------------------------
// ACK frame encode/decode
// ---------------------------------------------------------------------------

fn bench_ack_frame(c: &mut Criterion) {
    let ranges: Vec<SeqRange> = (0..10)
        .map(|i| SeqRange {
            start: i * 100,
            end: i * 100 + 50,
        })
        .collect();

    let frame = Frame::Ack {
        stream_id: 1,
        ack_seq: 999,
        ranges: ranges.clone(),
    };

    c.bench_function("ack_frame_encode", |b| {
        b.iter(|| {
            black_box(frame.encode());
        });
    });

    let encoded = frame.encode();
    c.bench_function("ack_frame_decode", |b| {
        b.iter(|| {
            black_box(Frame::decode(black_box(&encoded)).unwrap());
        });
    });
}

// ---------------------------------------------------------------------------
// CUBIC congestion window computation
// ---------------------------------------------------------------------------

fn bench_cubic_window(c: &mut Criterion) {
    c.bench_function("cubic_slow_start_20_acks", |b| {
        b.iter(|| {
            let mut cubic = Cubic::new();
            for _ in 0..20 {
                cubic.on_packet_sent(1200);
                cubic.on_ack(1200);
            }
            black_box(cubic.window());
        });
    });

    c.bench_function("cubic_loss_recovery", |b| {
        b.iter(|| {
            let mut cubic = Cubic::new();
            // Build up window.
            for _ in 0..50 {
                cubic.on_packet_sent(1200);
                cubic.on_ack(1200);
            }
            // Trigger loss.
            cubic.on_loss(1200);
            // Continue with acks.
            for _ in 0..50 {
                cubic.on_packet_sent(1200);
                cubic.on_ack(1200);
            }
            black_box(cubic.window());
        });
    });

    c.bench_function("cubic_repeated_loss_cycles", |b| {
        b.iter(|| {
            let mut cubic = Cubic::new();
            for _ in 0..5 {
                for _ in 0..20 {
                    cubic.on_packet_sent(1200);
                    cubic.on_ack(1200);
                }
                cubic.on_loss(1200);
            }
            black_box(cubic.window());
        });
    });
}

// ---------------------------------------------------------------------------
// Multiplexer dispatch throughput
// ---------------------------------------------------------------------------

fn bench_mux_dispatch(c: &mut Criterion) {
    c.bench_function("mux_create_stream", |b| {
        b.iter(|| {
            let mut mux = Multiplexer::new(1024);
            for _ in 0..100 {
                black_box(mux.create_stream(TransportMode::BestEffort).unwrap());
            }
        });
    });

    c.bench_function("mux_send_recv", |b| {
        let mut mux = Multiplexer::new(1024);
        let sid = mux.create_stream(TransportMode::BestEffort).unwrap();

        b.iter(|| {
            let data = Bytes::from_static(b"benchmark payload for multiplexer dispatch");
            mux.send(sid, data).unwrap();
            // Drain send buffer.
            if let Some(stream) = mux.get_stream_mut(sid) {
                let pending = stream.drain_send();
                black_box(pending);
            }
        });
    });

    c.bench_function("mux_poll_data_frame", |b| {
        let mut mux = Multiplexer::new(1024);
        let sid = mux.create_stream(TransportMode::ReliableOrdered).unwrap();

        let frame = Frame::Data {
            stream_id: sid,
            seq: 0,
            flags: DataFlags::NONE,
            payload: Bytes::from_static(b"incoming benchmark data"),
        };

        b.iter(|| {
            mux.poll(black_box(&frame)).unwrap();
            // Drain the receive buffer so it does not grow unboundedly.
            let _ = mux.recv(sid);
        });
    });
}

// ---------------------------------------------------------------------------
// Criterion harness
// ---------------------------------------------------------------------------

criterion_group! {
    name = transport_benches;
    config = Criterion::default()
        .sample_size(100)
        .measurement_time(Duration::from_secs(5));
    targets =
        bench_frame_encode,
        bench_frame_decode,
        bench_ack_frame,
        bench_cubic_window,
        bench_mux_dispatch
}

criterion_main!(transport_benches);
