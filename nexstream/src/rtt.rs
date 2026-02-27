//! RTT estimation using the Jacobson/Karels algorithm (RFC 6298).
//!
//! SRTT  = 7/8 * SRTT  + 1/8 * sample
//! RTTVAR = 3/4 * RTTVAR + 1/4 * |SRTT - sample|
//! RTO   = SRTT + max(1ms, 4 * RTTVAR)
//! RTO is clamped to [200ms, 60s].

use std::time::Duration;

/// Minimum RTO: 200 ms (per RFC 6298 recommendation).
const MIN_RTO: Duration = Duration::from_millis(200);
/// Maximum RTO: 60 seconds.
const MAX_RTO: Duration = Duration::from_secs(60);
/// Granularity floor for the variance component: 1 ms.
const GRANULARITY: Duration = Duration::from_millis(1);

/// RTT estimator implementing Jacobson/Karels smoothing.
#[derive(Debug, Clone)]
pub struct RttEstimator {
    /// Smoothed RTT.
    srtt: Option<Duration>,
    /// RTT variance.
    rttvar: Option<Duration>,
    /// Current retransmission timeout.
    rto: Duration,
}

impl RttEstimator {
    /// Create a new estimator with default initial RTO of 1 second.
    pub fn new() -> Self {
        Self {
            srtt: None,
            rttvar: None,
            rto: Duration::from_secs(1),
        }
    }

    /// Update the estimator with a new RTT sample.
    pub fn update(&mut self, sample: Duration) {
        match self.srtt {
            None => {
                // First sample: SRTT = sample, RTTVAR = sample / 2
                self.srtt = Some(sample);
                self.rttvar = Some(sample / 2);
            }
            Some(srtt) => {
                // RTTVAR = 3/4 * RTTVAR + 1/4 * |SRTT - sample|
                let diff = if srtt > sample {
                    srtt - sample
                } else {
                    sample - srtt
                };
                let rttvar = self.rttvar.unwrap_or(diff);
                let new_rttvar = (rttvar * 3 + diff) / 4;
                self.rttvar = Some(new_rttvar);

                // SRTT = 7/8 * SRTT + 1/8 * sample
                let new_srtt = (srtt * 7 + sample) / 8;
                self.srtt = Some(new_srtt);
            }
        }

        self.recompute_rto();
    }

    /// Recompute RTO from current SRTT/RTTVAR.
    fn recompute_rto(&mut self) {
        if let (Some(srtt), Some(rttvar)) = (self.srtt, self.rttvar) {
            let var_component = std::cmp::max(GRANULARITY, rttvar * 4);
            self.rto = (srtt + var_component).clamp(MIN_RTO, MAX_RTO);
        }
    }

    /// Returns the current smoothed RTT, or `None` if no samples yet.
    pub fn srtt(&self) -> Option<Duration> {
        self.srtt
    }

    /// Returns the current RTT variance, or `None` if no samples yet.
    pub fn rttvar(&self) -> Option<Duration> {
        self.rttvar
    }

    /// Returns the current retransmission timeout.
    pub fn rto(&self) -> Duration {
        self.rto
    }
}

impl Default for RttEstimator {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn first_sample_initializes() {
        let mut est = RttEstimator::new();
        est.update(Duration::from_millis(100));
        assert_eq!(est.srtt(), Some(Duration::from_millis(100)));
        assert_eq!(est.rttvar(), Some(Duration::from_millis(50)));
    }

    #[test]
    fn subsequent_samples_smooth() {
        let mut est = RttEstimator::new();
        est.update(Duration::from_millis(100));
        est.update(Duration::from_millis(120));

        // SRTT = 7/8 * 100 + 1/8 * 120 = 87.5 + 15 = 102.5ms
        let srtt = est.srtt().unwrap();
        assert!(
            srtt.as_millis() >= 102 && srtt.as_millis() <= 103,
            "srtt = {:?}",
            srtt
        );
    }

    #[test]
    fn rto_clamped_min() {
        let mut est = RttEstimator::new();
        // Very tiny RTT -> RTO should be at least MIN_RTO.
        est.update(Duration::from_micros(100));
        assert!(est.rto() >= MIN_RTO);
    }

    #[test]
    fn rto_clamped_max() {
        let mut est = RttEstimator::new();
        est.update(Duration::from_secs(100));
        assert!(est.rto() <= MAX_RTO);
    }
}
