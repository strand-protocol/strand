// StrandTrust cryptographic benchmarks using criterion.
//
// Measures:
//   - Ed25519 key generation
//   - Ed25519 sign / verify throughput
//   - ChaCha20-Poly1305 encrypt / decrypt at various payload sizes
//   - MIC build + serialize
//   - Full StrandTrust handshake latency

use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use std::time::Duration;

use strandtrust::crypto::aead::AeadCipher;
use strandtrust::crypto::keys::IdentityKeyPair;
use strandtrust::mic::builder::MICBuilder;
use strandtrust::mic::serializer;
use strandtrust::mic::Capability;
use strandtrust::handshake::protocol::{Initiator, Responder};

// ---------------------------------------------------------------------------
// Key generation
// ---------------------------------------------------------------------------

fn bench_keygen(c: &mut Criterion) {
    c.bench_function("ed25519_keygen", |b| {
        b.iter(|| {
            black_box(IdentityKeyPair::generate());
        });
    });
}

// ---------------------------------------------------------------------------
// Ed25519 sign / verify
// ---------------------------------------------------------------------------

fn bench_sign_verify(c: &mut Criterion) {
    let kp = IdentityKeyPair::generate();
    let message = b"StrandTrust benchmark message for Ed25519 sign/verify throughput testing";

    c.bench_function("ed25519_sign", |b| {
        b.iter(|| {
            black_box(kp.sign(black_box(message)));
        });
    });

    let sig = kp.sign(message);
    c.bench_function("ed25519_verify", |b| {
        b.iter(|| {
            black_box(kp.verify(black_box(message), black_box(&sig)).unwrap());
        });
    });
}

// ---------------------------------------------------------------------------
// ChaCha20-Poly1305 encrypt / decrypt
// ---------------------------------------------------------------------------

fn bench_chacha20(c: &mut Criterion) {
    let key = [0x42u8; 32];
    let nonce = [0u8; 12];
    let cipher = AeadCipher::new(key);

    let sizes: &[usize] = &[64, 1024, 64 * 1024, 1024 * 1024];

    let mut group = c.benchmark_group("chacha20_poly1305_encrypt");
    for &size in sizes {
        let plaintext = vec![0xABu8; size];
        group.throughput(Throughput::Bytes(size as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(format!("{size}B")),
            &plaintext,
            |b, pt| {
                b.iter(|| {
                    black_box(cipher.encrypt(&nonce, black_box(pt), b"").unwrap());
                });
            },
        );
    }
    group.finish();

    let mut group = c.benchmark_group("chacha20_poly1305_decrypt");
    for &size in sizes {
        let plaintext = vec![0xABu8; size];
        let ciphertext = cipher.encrypt(&nonce, &plaintext, b"").unwrap();
        group.throughput(Throughput::Bytes(size as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(format!("{size}B")),
            &ciphertext,
            |b, ct| {
                b.iter(|| {
                    black_box(cipher.decrypt(&nonce, black_box(ct), b"").unwrap());
                });
            },
        );
    }
    group.finish();
}

// ---------------------------------------------------------------------------
// MIC build + serialize
// ---------------------------------------------------------------------------

fn bench_mic_build_serialize(c: &mut Criterion) {
    let kp = IdentityKeyPair::generate();

    c.bench_function("mic_build", |b| {
        b.iter(|| {
            let mic = MICBuilder::new(black_box(&kp))
                .model_hash([0xAA; 32])
                .add_capability(Capability::TextGeneration)
                .add_capability(Capability::CodeGeneration)
                .add_capability(Capability::ToolUse)
                .validity(1000, 9999999)
                .build()
                .unwrap();
            black_box(mic);
        });
    });

    let mic = MICBuilder::new(&kp)
        .model_hash([0xAA; 32])
        .add_capability(Capability::TextGeneration)
        .add_capability(Capability::CodeGeneration)
        .validity(1000, 9999999)
        .build()
        .unwrap();

    c.bench_function("mic_serialize", |b| {
        b.iter(|| {
            black_box(serializer::serialize(black_box(&mic)));
        });
    });

    let bytes = serializer::serialize(&mic);
    c.bench_function("mic_deserialize", |b| {
        b.iter(|| {
            black_box(serializer::deserialize(black_box(&bytes)).unwrap());
        });
    });
}

// ---------------------------------------------------------------------------
// Full handshake latency
// ---------------------------------------------------------------------------

fn bench_full_handshake(c: &mut Criterion) {
    c.bench_function("full_handshake", |b| {
        b.iter(|| {
            let client_kp = IdentityKeyPair::generate();
            let client_mic = MICBuilder::new(&client_kp)
                .model_hash([0xDD; 32])
                .add_capability(Capability::TextGeneration)
                .validity(1000, 9999999)
                .build()
                .unwrap();

            let server_kp = IdentityKeyPair::generate();
            let server_mic = MICBuilder::new(&server_kp)
                .model_hash([0xEE; 32])
                .add_capability(Capability::TextGeneration)
                .validity(1000, 9999999)
                .build()
                .unwrap();

            let mut initiator = Initiator::new(client_kp, client_mic);
            let mut responder = Responder::new(server_kp, server_mic);

            let now = 5000u64;

            let init_msg = initiator.create_init().unwrap();
            let response_msg = responder.process_init(init_msg, now).unwrap();
            let complete_msg = initiator.process_response(response_msg, now).unwrap();
            responder.process_complete(complete_msg).unwrap();

            black_box(initiator.completed_state());
            black_box(responder.completed_state());
        });
    });
}

// ---------------------------------------------------------------------------
// Criterion harness
// ---------------------------------------------------------------------------

criterion_group! {
    name = crypto_benches;
    config = Criterion::default()
        .sample_size(100)
        .measurement_time(Duration::from_secs(5));
    targets =
        bench_keygen,
        bench_sign_verify,
        bench_chacha20,
        bench_mic_build_serialize,
        bench_full_handshake
}

criterion_main!(crypto_benches);
