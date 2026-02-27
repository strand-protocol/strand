//! CUBIC congestion control (RFC 8312).
//!
//! Slow start: cwnd += MSS per ACK (approximately doubles per RTT).
//! Congestion avoidance: W(t) = C * (t - K)^3 + w_max
//!   where C = 0.4, K = cbrt(w_max * beta / C), beta = 0.7
//! On loss: w_max = cwnd, ssthresh = cwnd * beta, cwnd = ssthresh.

use std::time::Instant;

use crate::congestion::CongestionController;

/// CUBIC constants.
const C: f64 = 0.4;
const BETA: f64 = 0.7;

/// Default maximum segment size.
const MSS: usize = 1200;

/// Default initial congestion window: 10 * MSS (per RFC 6928).
const INITIAL_WINDOW: usize = 10 * MSS;

/// Minimum congestion window: 2 * MSS.
const MIN_WINDOW: usize = 2 * MSS;

/// Maximum congestion window: 1 GiB. Clamps cwnd to prevent integer overflow
/// and unbounded memory pressure on extremely high-bandwidth links.
const MAX_CWND: usize = 1024 * 1024 * 1024;

/// CUBIC congestion controller.
#[derive(Debug)]
pub struct Cubic {
    /// Current congestion window in bytes.
    cwnd: usize,
    /// Slow-start threshold.
    ssthresh: usize,
    /// Window size just before the last loss event.
    w_max: f64,
    /// Time when the current congestion avoidance epoch started.
    epoch_start: Option<Instant>,
    /// Precomputed K value for the current epoch.
    k: f64,
    /// Bytes in flight.
    in_flight: usize,
    /// Count of ACKed bytes in the current slow-start/CA cycle
    /// used for window increase calculation.
    ack_accum: usize,
}

impl Cubic {
    /// Create a new CUBIC controller with default parameters.
    pub fn new() -> Self {
        Self {
            cwnd: INITIAL_WINDOW,
            ssthresh: usize::MAX,
            w_max: 0.0,
            epoch_start: None,
            k: 0.0,
            in_flight: 0,
            ack_accum: 0,
        }
    }

    /// Returns whether we are in slow start.
    pub fn in_slow_start(&self) -> bool {
        self.cwnd < self.ssthresh
    }

    /// Compute the CUBIC target window at time `t` seconds since epoch start.
    fn cubic_window(&self, t: f64) -> f64 {
        let dt = t - self.k;
        C * dt * dt * dt + self.w_max
    }
}

impl Default for Cubic {
    fn default() -> Self {
        Self::new()
    }
}

impl CongestionController for Cubic {
    fn on_packet_sent(&mut self, bytes: usize) {
        self.in_flight += bytes;
    }

    fn on_ack(&mut self, bytes: usize) {
        self.in_flight = self.in_flight.saturating_sub(bytes);

        if self.in_slow_start() {
            // Slow start: increase cwnd by one MSS per ACK.
            self.cwnd = self.cwnd.saturating_add(MSS).min(MAX_CWND);
            if self.cwnd >= self.ssthresh {
                // Exiting slow start, start congestion avoidance epoch.
                self.epoch_start = Some(Instant::now());
                self.k = ((self.w_max * (1.0 - BETA)) / C).cbrt();
            }
        } else {
            // Congestion avoidance (CUBIC).
            let now = Instant::now();
            if self.epoch_start.is_none() {
                self.epoch_start = Some(now);
                self.k = ((self.w_max * (1.0 - BETA)) / C).cbrt();
                self.ack_accum = 0;
            }

            let t = now
                .duration_since(self.epoch_start.unwrap())
                .as_secs_f64();
            let w_cubic = self.cubic_window(t);
            let target = w_cubic.max(self.cwnd as f64);

            // Increase cwnd towards target.
            self.ack_accum += bytes;
            if self.ack_accum >= self.cwnd {
                let increase = ((target - self.cwnd as f64) / (self.cwnd as f64 / MSS as f64))
                    .max(0.0) as usize;
                self.cwnd = self.cwnd.saturating_add(increase.max(1)).min(MAX_CWND);
                self.ack_accum = 0;
            }
        }
    }

    fn on_loss(&mut self, bytes: usize) {
        self.in_flight = self.in_flight.saturating_sub(bytes);

        // Multiplicative decrease.
        self.w_max = self.cwnd as f64;
        self.ssthresh = ((self.cwnd as f64 * BETA) as usize).max(MIN_WINDOW);
        self.cwnd = self.ssthresh;

        // Reset epoch.
        self.epoch_start = None;
        self.k = ((self.w_max * (1.0 - BETA)) / C).cbrt();
        self.ack_accum = 0;
    }

    fn window(&self) -> usize {
        self.cwnd
    }

    fn bytes_in_flight(&self) -> usize {
        self.in_flight
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn initial_window() {
        let c = Cubic::new();
        assert_eq!(c.window(), INITIAL_WINDOW);
        assert!(c.in_slow_start());
    }

    #[test]
    fn slow_start_increases_cwnd() {
        let mut c = Cubic::new();
        let initial = c.window();
        c.on_packet_sent(MSS);
        c.on_ack(MSS);
        assert!(c.window() > initial);
    }

    #[test]
    fn loss_reduces_window() {
        let mut c = Cubic::new();
        // Pump up the window.
        for _ in 0..20 {
            c.on_packet_sent(MSS);
            c.on_ack(MSS);
        }
        let pre_loss = c.window();
        c.on_loss(MSS);
        assert!(c.window() < pre_loss);
    }

    #[test]
    fn loss_sets_ssthresh_using_beta() {
        let mut c = Cubic::new();
        // Set cwnd to a known value via slow start.
        while c.window() < 100 * MSS {
            c.on_packet_sent(MSS);
            c.on_ack(MSS);
        }
        let pre_loss = c.window();
        c.on_loss(MSS);
        let expected_ssthresh = ((pre_loss as f64 * BETA) as usize).max(MIN_WINDOW);
        assert_eq!(c.window(), expected_ssthresh);
    }

    #[test]
    fn min_window_enforced() {
        let mut c = Cubic::new();
        // Trigger many losses to push window down.
        for _ in 0..50 {
            c.on_loss(MSS);
        }
        assert!(c.window() >= MIN_WINDOW);
    }

    #[test]
    fn max_window_clamped() {
        let mut c = Cubic::new();
        // Drive many ACKs to grow cwnd as large as possible.
        for _ in 0..1_000_000 {
            c.on_packet_sent(MSS);
            c.on_ack(MSS);
        }
        // cwnd must never exceed MAX_CWND regardless of ACK count.
        assert!(c.window() <= MAX_CWND, "cwnd {} exceeds MAX_CWND {}", c.window(), MAX_CWND);
    }
}
