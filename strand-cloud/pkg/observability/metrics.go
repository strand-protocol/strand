// Package observability provides lightweight internal metrics counters for the
// Nexus Cloud control plane.
package observability

import "sync/atomic"

// Metrics holds simple atomic counters for key control-plane operations.
type Metrics struct {
	requestCount atomic.Int64
	errorCount   atomic.Int64
	nodeCount    atomic.Int64
	routeCount   atomic.Int64
}

// NewMetrics returns a zero-initialised Metrics.
func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncRequest()  { m.requestCount.Add(1) }
func (m *Metrics) IncError()    { m.errorCount.Add(1) }
func (m *Metrics) IncNode()     { m.nodeCount.Add(1) }
func (m *Metrics) DecNode()     { m.nodeCount.Add(-1) }
func (m *Metrics) IncRoute()    { m.routeCount.Add(1) }
func (m *Metrics) DecRoute()    { m.routeCount.Add(-1) }

// GetMetrics returns a snapshot of the counters.
func (m *Metrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"request_count": m.requestCount.Load(),
		"error_count":   m.errorCount.Load(),
		"node_count":    m.nodeCount.Load(),
		"route_count":   m.routeCount.Load(),
	}
}
