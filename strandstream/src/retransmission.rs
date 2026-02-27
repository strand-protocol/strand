//! Retransmission engine using a BinaryHeap ordered by retransmit time.
//!
//! Supports exponential backoff (rto *= 2) on each retransmission,
//! with a maximum of 3 retries per packet.

use std::cmp::Ordering;
use std::collections::{BinaryHeap, HashMap};
use std::time::{Duration, Instant};

use bytes::Bytes;

use crate::error::{NexStreamError, Result};

/// Maximum number of retransmission attempts before giving up.
const MAX_RETRIES: u32 = 3;

/// Default maximum bytes held in the retransmit buffer across all in-flight
/// packets. 64 MiB prevents unbounded memory growth under sustained loss.
const MAX_INFLIGHT_BYTES: usize = 64 * 1024 * 1024;

/// An entry in the retransmission queue.
#[derive(Debug, Clone)]
struct RetransmitEntry {
    /// Sequence number.
    seq: u64,
    /// Data to retransmit.
    data: Bytes,
    /// When this packet should be retransmitted.
    retransmit_at: Instant,
    /// Current RTO for this packet.
    rto: Duration,
    /// Number of retransmission attempts so far.
    attempts: u32,
}

// BinaryHeap is a max-heap; we want the *earliest* retransmit_at first,
// so we reverse the ordering.
impl PartialEq for RetransmitEntry {
    fn eq(&self, other: &Self) -> bool {
        self.retransmit_at == other.retransmit_at
    }
}

impl Eq for RetransmitEntry {}

impl PartialOrd for RetransmitEntry {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for RetransmitEntry {
    fn cmp(&self, other: &Self) -> Ordering {
        // Reverse ordering so that the earliest deadline is popped first.
        other.retransmit_at.cmp(&self.retransmit_at)
    }
}

/// A packet that has exceeded its maximum retransmission attempts.
#[derive(Debug)]
pub struct GivenUp {
    pub seq: u64,
    pub data: Bytes,
    pub attempts: u32,
}

/// A packet ready for retransmission.
#[derive(Debug)]
pub struct RetransmitPacket {
    pub seq: u64,
    pub data: Bytes,
}

/// Retransmission engine.
///
/// Packets are pushed with an initial RTO. `poll_expired` returns packets
/// whose timer has fired. Exponential backoff is applied on each retransmit.
/// After `MAX_RETRIES` the packet is reported as given up.
///
/// The total in-flight byte count is capped at `max_bytes` (default 64 MiB)
/// to prevent unbounded memory growth under sustained packet loss.
pub struct RetransmissionEngine {
    heap: BinaryHeap<RetransmitEntry>,
    /// Track which sequences are still pending and their payload size.
    pending: HashMap<u64, usize>,
    /// Total bytes currently held in the retransmit buffer.
    inflight_bytes: usize,
    /// Maximum allowed in-flight bytes.
    max_bytes: usize,
}

impl RetransmissionEngine {
    pub fn new() -> Self {
        Self {
            heap: BinaryHeap::new(),
            pending: HashMap::new(),
            inflight_bytes: 0,
            max_bytes: MAX_INFLIGHT_BYTES,
        }
    }

    /// Create an engine with a custom in-flight byte limit.
    pub fn with_max_bytes(max_bytes: usize) -> Self {
        Self {
            heap: BinaryHeap::new(),
            pending: HashMap::new(),
            inflight_bytes: 0,
            max_bytes,
        }
    }

    /// Register a packet for potential retransmission.
    ///
    /// Returns `Err(RetransmitBufferFull)` if adding this packet would exceed
    /// the configured in-flight byte limit.
    pub fn push(&mut self, seq: u64, data: Bytes, rto: Duration) -> Result<()> {
        let new_inflight = self
            .inflight_bytes
            .checked_add(data.len())
            .unwrap_or(usize::MAX);
        if new_inflight > self.max_bytes {
            return Err(NexStreamError::RetransmitBufferFull {
                inflight: self.inflight_bytes,
                max: self.max_bytes,
            });
        }
        self.inflight_bytes = new_inflight;
        self.pending.insert(seq, data.len());
        let entry = RetransmitEntry {
            seq,
            data,
            retransmit_at: Instant::now() + rto,
            rto,
            attempts: 0,
        };
        self.heap.push(entry);
        Ok(())
    }

    /// Acknowledge a packet, removing it from the retransmission queue.
    ///
    /// Returns `true` if the packet was still pending.
    pub fn on_ack(&mut self, seq: u64) -> bool {
        if let Some(len) = self.pending.remove(&seq) {
            self.inflight_bytes = self.inflight_bytes.saturating_sub(len);
            true
        } else {
            false
        }
        // The entry may still be in the heap but will be skipped by poll_expired.
    }

    /// Poll for packets whose retransmission timer has expired.
    ///
    /// Returns packets to retransmit and packets that have exceeded the max
    /// retry count.
    pub fn poll_expired(&mut self, now: Instant) -> (Vec<RetransmitPacket>, Vec<GivenUp>) {
        let mut to_retransmit = Vec::new();
        let mut given_up = Vec::new();

        while let Some(entry) = self.heap.peek() {
            if entry.retransmit_at > now {
                break;
            }

            let entry = self.heap.pop().unwrap();

            // Skip if already ACKed.
            if !self.pending.contains_key(&entry.seq) {
                continue;
            }

            if entry.attempts >= MAX_RETRIES {
                let len = self.pending.remove(&entry.seq).unwrap_or(0);
                self.inflight_bytes = self.inflight_bytes.saturating_sub(len);
                given_up.push(GivenUp {
                    seq: entry.seq,
                    data: entry.data,
                    attempts: entry.attempts,
                });
            } else {
                to_retransmit.push(RetransmitPacket {
                    seq: entry.seq,
                    data: entry.data.clone(),
                });

                // Re-enqueue with exponential backoff. inflight_bytes unchanged.
                let new_rto = entry.rto * 2;
                self.heap.push(RetransmitEntry {
                    seq: entry.seq,
                    data: entry.data,
                    retransmit_at: now + new_rto,
                    rto: new_rto,
                    attempts: entry.attempts + 1,
                });
            }
        }

        (to_retransmit, given_up)
    }

    /// Number of packets still pending retransmission.
    pub fn pending_count(&self) -> usize {
        self.pending.len()
    }

    /// Total bytes currently held in the retransmit buffer.
    pub fn inflight_bytes(&self) -> usize {
        self.inflight_bytes
    }
}

impl Default for RetransmissionEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn push_and_ack() {
        let mut engine = RetransmissionEngine::new();
        engine
            .push(1, Bytes::from_static(b"hello"), Duration::from_millis(100))
            .unwrap();
        assert_eq!(engine.pending_count(), 1);
        assert_eq!(engine.inflight_bytes(), 5);
        assert!(engine.on_ack(1));
        assert_eq!(engine.pending_count(), 0);
        assert_eq!(engine.inflight_bytes(), 0);
    }

    #[test]
    fn poll_before_expiry_returns_nothing() {
        let mut engine = RetransmissionEngine::new();
        let now = Instant::now();
        engine
            .push(1, Bytes::from_static(b"A"), Duration::from_secs(10))
            .unwrap();

        let (retx, given) = engine.poll_expired(now);
        assert!(retx.is_empty());
        assert!(given.is_empty());
    }

    #[test]
    fn poll_after_expiry_returns_packet() {
        let mut engine = RetransmissionEngine::new();
        engine
            .push(1, Bytes::from_static(b"A"), Duration::from_millis(10))
            .unwrap();

        // Simulate time passing.
        let later = Instant::now() + Duration::from_millis(50);
        let (retx, given) = engine.poll_expired(later);
        assert_eq!(retx.len(), 1);
        assert_eq!(retx[0].seq, 1);
        assert!(given.is_empty());
    }

    #[test]
    fn exponential_backoff_and_give_up() {
        let mut engine = RetransmissionEngine::new();
        let rto = Duration::from_millis(10);
        engine
            .push(1, Bytes::from_static(b"A"), rto)
            .unwrap();

        // Attempt 1 (attempts goes from 0 to 1, new rto = 20ms)
        let t1 = Instant::now() + Duration::from_millis(50);
        let (retx, _) = engine.poll_expired(t1);
        assert_eq!(retx.len(), 1);

        // Attempt 2 (attempts goes from 1 to 2, new rto = 40ms)
        let t2 = t1 + Duration::from_millis(50);
        let (retx, _) = engine.poll_expired(t2);
        assert_eq!(retx.len(), 1);

        // Attempt 3 (attempts goes from 2 to 3, new rto = 80ms)
        let t3 = t2 + Duration::from_millis(100);
        let (retx, _) = engine.poll_expired(t3);
        assert_eq!(retx.len(), 1);

        // Attempt 4 (attempts == 3 == MAX_RETRIES, gives up)
        let t4 = t3 + Duration::from_millis(200);
        let (retx, given) = engine.poll_expired(t4);
        assert!(retx.is_empty());
        assert_eq!(given.len(), 1);
        assert_eq!(given[0].seq, 1);
        assert_eq!(engine.pending_count(), 0);
        assert_eq!(engine.inflight_bytes(), 0);
    }

    #[test]
    fn retransmit_buffer_limit_rejects_overflow() {
        // Create an engine with a tiny limit (16 bytes).
        let mut engine = RetransmissionEngine::with_max_bytes(16);

        // First push fits: 10 bytes in flight.
        engine
            .push(1, Bytes::from(vec![0u8; 10]), Duration::from_secs(10))
            .unwrap();

        // Second push would bring total to 20 bytes, exceeding 16-byte limit.
        let result = engine.push(2, Bytes::from(vec![0u8; 10]), Duration::from_secs(10));
        assert!(result.is_err());
        matches!(
            result.unwrap_err(),
            NexStreamError::RetransmitBufferFull { .. }
        );

        // After ACKing the first packet, push succeeds again.
        assert!(engine.on_ack(1));
        engine
            .push(2, Bytes::from(vec![0u8; 10]), Duration::from_secs(10))
            .unwrap();
    }
}
