// Package observability provides lightweight internal metrics counters for the
// Strand Cloud control plane.
package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds simple atomic counters for key control-plane operations.
type Metrics struct {
	requestCount atomic.Int64
	errorCount   atomic.Int64
	nodeCount    atomic.Int64
	routeCount   atomic.Int64

	// Latency tracking: rolling window of recent request durations.
	latMu      sync.Mutex
	latencies  []time.Duration // circular buffer
	latIdx     int
	latFull    bool
	activeConn atomic.Int64
}

const latencyWindowSize = 1000

// NewMetrics returns a zero-initialised Metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		latencies: make([]time.Duration, latencyWindowSize),
	}
}

func (m *Metrics) IncRequest()  { m.requestCount.Add(1) }
func (m *Metrics) IncError()    { m.errorCount.Add(1) }
func (m *Metrics) IncNode()     { m.nodeCount.Add(1) }
func (m *Metrics) DecNode()     { m.nodeCount.Add(-1) }
func (m *Metrics) IncRoute()    { m.routeCount.Add(1) }
func (m *Metrics) DecRoute()    { m.routeCount.Add(-1) }
func (m *Metrics) IncConn()     { m.activeConn.Add(1) }
func (m *Metrics) DecConn()     { m.activeConn.Add(-1) }

// RecordLatency records a request duration for percentile computation.
func (m *Metrics) RecordLatency(d time.Duration) {
	m.latMu.Lock()
	m.latencies[m.latIdx] = d
	m.latIdx++
	if m.latIdx >= latencyWindowSize {
		m.latIdx = 0
		m.latFull = true
	}
	m.latMu.Unlock()
}

// LatencySnapshot returns a copy of the current latency samples.
func (m *Metrics) LatencySnapshot() []time.Duration {
	m.latMu.Lock()
	defer m.latMu.Unlock()
	var n int
	if m.latFull {
		n = latencyWindowSize
	} else {
		n = m.latIdx
	}
	out := make([]time.Duration, n)
	copy(out, m.latencies[:n])
	return out
}

// GetMetrics returns a snapshot of the counters.
func (m *Metrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"request_count":      m.requestCount.Load(),
		"error_count":        m.errorCount.Load(),
		"node_count":         m.nodeCount.Load(),
		"route_count":        m.routeCount.Load(),
		"active_connections": m.activeConn.Load(),
	}
}
