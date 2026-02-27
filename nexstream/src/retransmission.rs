//! Retransmission engine using a BinaryHeap ordered by retransmit time.
//!
//! Supports exponential backoff (rto *= 2) on each retransmission,
//! with a maximum of 3 retries per packet.

use std::cmp::Ordering;
use std::collections::{BinaryHeap, HashMap};
use std::time::{Duration, Instant};

use bytes::Bytes;

/// Maximum number of retransmission attempts before giving up.
const MAX_RETRIES: u32 = 3;

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
pub struct RetransmissionEngine {
    heap: BinaryHeap<RetransmitEntry>,
    /// Track which sequences are still pending (for on_ack removal).
    pending: HashMap<u64, ()>,
}

impl RetransmissionEngine {
    pub fn new() -> Self {
        Self {
            heap: BinaryHeap::new(),
            pending: HashMap::new(),
        }
    }

    /// Register a packet for potential retransmission.
    pub fn push(&mut self, seq: u64, data: Bytes, rto: Duration) {
        let entry = RetransmitEntry {
            seq,
            data,
            retransmit_at: Instant::now() + rto,
            rto,
            attempts: 0,
        };
        self.pending.insert(seq, ());
        self.heap.push(entry);
    }

    /// Acknowledge a packet, removing it from the retransmission queue.
    ///
    /// Returns `true` if the packet was still pending.
    pub fn on_ack(&mut self, seq: u64) -> bool {
        self.pending.remove(&seq).is_some()
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
                self.pending.remove(&entry.seq);
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

                // Re-enqueue with exponential backoff.
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
        engine.push(1, Bytes::from_static(b"hello"), Duration::from_millis(100));
        assert_eq!(engine.pending_count(), 1);
        assert!(engine.on_ack(1));
        assert_eq!(engine.pending_count(), 0);
    }

    #[test]
    fn poll_before_expiry_returns_nothing() {
        let mut engine = RetransmissionEngine::new();
        let now = Instant::now();
        engine.push(1, Bytes::from_static(b"A"), Duration::from_secs(10));

        let (retx, given) = engine.poll_expired(now);
        assert!(retx.is_empty());
        assert!(given.is_empty());
    }

    #[test]
    fn poll_after_expiry_returns_packet() {
        let mut engine = RetransmissionEngine::new();
        engine.push(1, Bytes::from_static(b"A"), Duration::from_millis(10));

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
        engine.push(1, Bytes::from_static(b"A"), rto);

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
    }
}
