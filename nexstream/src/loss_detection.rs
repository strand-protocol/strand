//! Loss detection using packet-threshold and time-threshold methods
//! as specified in RFC 9002 section 6.1.
//!
//! A packet is declared lost if:
//! - **packet_threshold**: at least 3 packets with higher sequence numbers
//!   have been acknowledged, OR
//! - **time_threshold**: more than max(SRTT * 9/8, 1ms) has elapsed since
//!   the packet was sent.

use std::collections::BTreeMap;
use std::time::{Duration, Instant};

/// Number of later-acknowledged packets before a packet is declared lost.
const PACKET_THRESHOLD: u64 = 3;
/// Minimum time threshold for loss detection.
const MIN_TIME_THRESHOLD: Duration = Duration::from_millis(1);

/// Tracks sent packets and detects losses.
pub struct LossDetector {
    /// Maps sequence number -> time the packet was sent.
    sent_packets: BTreeMap<u64, Instant>,
    /// The largest acknowledged sequence number.
    largest_acked: Option<u64>,
}

impl LossDetector {
    pub fn new() -> Self {
        Self {
            sent_packets: BTreeMap::new(),
            largest_acked: None,
        }
    }

    /// Record that a packet with the given sequence number was sent.
    pub fn on_packet_sent(&mut self, seq: u64, sent_time: Instant) {
        self.sent_packets.insert(seq, sent_time);
    }

    /// Process an ACK and return the set of sequence numbers now considered lost.
    ///
    /// `ack_seq` is the sequence number being acknowledged.
    /// `srtt` is the current smoothed RTT estimate (for time-based loss detection).
    /// `now` is the current time.
    pub fn on_ack_received(
        &mut self,
        ack_seq: u64,
        srtt: Duration,
        now: Instant,
    ) -> Vec<u64> {
        // Remove the acknowledged packet.
        self.sent_packets.remove(&ack_seq);

        // Update largest acked.
        self.largest_acked = Some(match self.largest_acked {
            Some(prev) => prev.max(ack_seq),
            None => ack_seq,
        });

        let largest = self.largest_acked.unwrap();

        // Time threshold: max(SRTT * 9/8, 1ms)
        let time_threshold = std::cmp::max(srtt * 9 / 8, MIN_TIME_THRESHOLD);

        let mut lost = Vec::new();
        let seqs: Vec<u64> = self.sent_packets.keys().copied().collect();

        for seq in seqs {
            // Packet threshold: 3 packets with higher seq numbers were ACKed.
            let packet_lost = largest >= seq + PACKET_THRESHOLD;

            // Time threshold: sent more than time_threshold ago.
            let time_lost = if let Some(&sent_time) = self.sent_packets.get(&seq) {
                now.duration_since(sent_time) > time_threshold
            } else {
                false
            };

            if packet_lost || time_lost {
                self.sent_packets.remove(&seq);
                lost.push(seq);
            }
        }

        lost
    }

    /// Returns the number of packets still in flight (sent but not acknowledged
    /// or declared lost).
    pub fn in_flight(&self) -> usize {
        self.sent_packets.len()
    }
}

impl Default for LossDetector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn no_loss_when_all_acked_in_order() {
        let mut ld = LossDetector::new();
        let now = Instant::now();

        for i in 0..5 {
            ld.on_packet_sent(i, now);
        }

        for i in 0..5 {
            let lost = ld.on_ack_received(i, Duration::from_millis(100), now);
            assert!(lost.is_empty(), "unexpected loss at ack {i}");
        }

        assert_eq!(ld.in_flight(), 0);
    }

    #[test]
    fn packet_threshold_loss() {
        let mut ld = LossDetector::new();
        let now = Instant::now();

        // Send packets 0..6
        for i in 0..6 {
            ld.on_packet_sent(i, now);
        }

        // ACK packets 1,2,3 (skip 0).
        let _ = ld.on_ack_received(1, Duration::from_millis(100), now);
        let _ = ld.on_ack_received(2, Duration::from_millis(100), now);
        // After acking 3, packet 0 should be lost (3 higher seq nums acked: 1,2,3).
        let lost = ld.on_ack_received(3, Duration::from_millis(100), now);
        assert!(lost.contains(&0), "packet 0 should be declared lost");
    }

    #[test]
    fn time_threshold_loss() {
        let mut ld = LossDetector::new();
        let start = Instant::now();

        ld.on_packet_sent(0, start);
        ld.on_packet_sent(1, start);

        // Simulate time passing > 9/8 * SRTT
        let srtt = Duration::from_millis(100);
        let later = start + Duration::from_millis(200); // well past 112.5ms threshold

        // ACK packet 1 only, at a later time.
        let lost = ld.on_ack_received(1, srtt, later);
        assert!(lost.contains(&0), "packet 0 should be time-threshold lost");
    }
}
