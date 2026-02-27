//! CUBIC congestion control tests: slow start and congestion avoidance.

use strandstream::congestion::cubic::Cubic;
use strandstream::congestion::CongestionController;

const MSS: usize = 1200;

#[test]
fn initial_state() {
    let c = Cubic::new();
    assert_eq!(c.window(), 10 * MSS); // INITIAL_WINDOW
    assert_eq!(c.bytes_in_flight(), 0);
    assert!(c.in_slow_start());
}

#[test]
fn slow_start_doubles_per_rtt_approx() {
    let mut c = Cubic::new();
    let initial = c.window();

    // Simulate one RTT: send INITIAL_WINDOW worth, then ACK all.
    let segments = initial / MSS;
    for _ in 0..segments {
        c.on_packet_sent(MSS);
    }
    for _ in 0..segments {
        c.on_ack(MSS);
    }

    // After one RTT of slow start, cwnd should roughly double.
    assert!(
        c.window() >= initial + segments * MSS / 2,
        "cwnd should have grown significantly: {} -> {}",
        initial,
        c.window()
    );
}

#[test]
fn loss_triggers_multiplicative_decrease() {
    let mut c = Cubic::new();

    // Pump window up in slow start.
    for _ in 0..50 {
        c.on_packet_sent(MSS);
        c.on_ack(MSS);
    }

    let pre_loss = c.window();
    c.on_loss(MSS);
    let post_loss = c.window();

    // After loss: cwnd = ssthresh = cwnd * beta (0.7).
    let expected = ((pre_loss as f64) * 0.7) as usize;
    // Allow small rounding difference.
    assert!(
        (post_loss as isize - expected as isize).unsigned_abs() <= MSS,
        "post_loss {} not close to expected {}",
        post_loss,
        expected
    );
}

#[test]
fn congestion_avoidance_window_grows() {
    let mut c = Cubic::new();

    // Enter congestion avoidance by triggering a loss.
    for _ in 0..50 {
        c.on_packet_sent(MSS);
        c.on_ack(MSS);
    }
    c.on_loss(MSS);
    assert!(!c.in_slow_start());

    let post_loss = c.window();

    // Continue sending and acking -- window should grow (CUBIC).
    for _ in 0..200 {
        c.on_packet_sent(MSS);
        c.on_ack(MSS);
    }

    assert!(
        c.window() > post_loss,
        "window should grow in congestion avoidance: {} -> {}",
        post_loss,
        c.window()
    );
}

#[test]
fn repeated_losses_dont_go_below_minimum() {
    let mut c = Cubic::new();

    for _ in 0..100 {
        c.on_loss(MSS);
    }

    // Minimum window is 2 * MSS = 2400.
    assert!(
        c.window() >= 2 * MSS,
        "window {} below minimum {}",
        c.window(),
        2 * MSS
    );
}

#[test]
fn can_send_respects_window() {
    let mut c = Cubic::new();

    // Fill up the window.
    let window = c.window();
    c.on_packet_sent(window);
    assert!(!c.can_send(1));

    // ACK some data.
    c.on_ack(MSS);
    assert!(c.can_send(MSS));
}

#[test]
fn bytes_in_flight_tracking() {
    let mut c = Cubic::new();

    c.on_packet_sent(1000);
    assert_eq!(c.bytes_in_flight(), 1000);

    c.on_packet_sent(500);
    assert_eq!(c.bytes_in_flight(), 1500);

    c.on_ack(600);
    assert_eq!(c.bytes_in_flight(), 900);

    c.on_loss(400);
    assert_eq!(c.bytes_in_flight(), 500);
}

#[test]
fn slow_start_exits_on_loss() {
    let mut c = Cubic::new();
    assert!(c.in_slow_start());

    // Grow the window a bit.
    for _ in 0..5 {
        c.on_packet_sent(MSS);
        c.on_ack(MSS);
    }
    assert!(c.in_slow_start()); // ssthresh still MAX

    c.on_loss(MSS);
    assert!(!c.in_slow_start()); // now cwnd == ssthresh
}
