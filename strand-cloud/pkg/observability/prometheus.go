package observability

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

// PrometheusHandler returns an http.HandlerFunc that exports metrics in
// Prometheus text exposition format at /metrics.
func (m *Metrics) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		snap := m.GetMetrics()
		fmt.Fprintf(w, "# HELP strand_requests_total Total number of API requests.\n")
		fmt.Fprintf(w, "# TYPE strand_requests_total counter\n")
		fmt.Fprintf(w, "strand_requests_total %d\n\n", snap["request_count"])

		fmt.Fprintf(w, "# HELP strand_errors_total Total number of API errors.\n")
		fmt.Fprintf(w, "# TYPE strand_errors_total counter\n")
		fmt.Fprintf(w, "strand_errors_total %d\n\n", snap["error_count"])

		fmt.Fprintf(w, "# HELP strand_nodes_registered Current number of registered nodes.\n")
		fmt.Fprintf(w, "# TYPE strand_nodes_registered gauge\n")
		fmt.Fprintf(w, "strand_nodes_registered %d\n\n", snap["node_count"])

		fmt.Fprintf(w, "# HELP strand_routes_active Current number of active routes.\n")
		fmt.Fprintf(w, "# TYPE strand_routes_active gauge\n")
		fmt.Fprintf(w, "strand_routes_active %d\n\n", snap["route_count"])

		fmt.Fprintf(w, "# HELP strand_active_connections Current number of active connections.\n")
		fmt.Fprintf(w, "# TYPE strand_active_connections gauge\n")
		fmt.Fprintf(w, "strand_active_connections %d\n\n", snap["active_connections"])

		// Latency percentiles from the rolling window.
		latencies := m.LatencySnapshot()
		if len(latencies) > 0 {
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			fmt.Fprintf(w, "# HELP strand_request_duration_seconds Request latency percentiles.\n")
			fmt.Fprintf(w, "# TYPE strand_request_duration_seconds summary\n")
			fmt.Fprintf(w, "strand_request_duration_seconds{quantile=\"0.5\"} %f\n", percentile(latencies, 0.5))
			fmt.Fprintf(w, "strand_request_duration_seconds{quantile=\"0.95\"} %f\n", percentile(latencies, 0.95))
			fmt.Fprintf(w, "strand_request_duration_seconds{quantile=\"0.99\"} %f\n", percentile(latencies, 0.99))
			fmt.Fprintf(w, "strand_request_duration_seconds_count %d\n\n", len(latencies))
		}
	}
}

// percentile returns the p-th percentile value from sorted durations.
func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx].Seconds()
}
