package apiserver

import (
	"net/http"
)

// Billing plan definitions served via the API. These match platform/auth/pkg/billing/plans.go.
var billingPlans = []map[string]interface{}{
	{
		"name": "free", "display_name": "Free", "base_price_cents": 0,
		"max_clusters": 1, "max_nodes_per_cluster": 3, "mics_included": 100,
		"traffic_gb_included": 1, "support_level": "community", "uptime_sla": "none",
	},
	{
		"name": "starter", "display_name": "Starter", "base_price_cents": 50000,
		"max_clusters": 1, "max_nodes_per_cluster": 10, "mics_included": 1000,
		"mic_overage_cents": 300, "traffic_gb_included": 10, "traffic_overage_cents": 8,
		"support_level": "email", "uptime_sla": "99.5%",
	},
	{
		"name": "pro", "display_name": "Pro", "base_price_cents": 500000,
		"max_clusters": 3, "max_nodes_per_cluster": 50, "mics_included": 10000,
		"mic_overage_cents": 250, "traffic_gb_included": 100, "traffic_overage_cents": 5,
		"support_level": "priority", "uptime_sla": "99.9%",
	},
	{
		"name": "enterprise", "display_name": "Enterprise", "base_price_cents": 1500000,
		"max_clusters": 999, "max_nodes_per_cluster": 9999, "mics_included": 999999,
		"mic_overage_cents": 200, "traffic_gb_included": 10000, "traffic_overage_cents": 2,
		"support_level": "dedicated", "uptime_sla": "99.99%",
	},
}

func (s *Server) handleListPlans(w http.ResponseWriter, _ *http.Request) {
	s.metrics.IncRequest()
	writeJSON(w, http.StatusOK, billingPlans)
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	// In production, this would query the usage_records table.
	// For now, return placeholder usage data.
	usage := map[string]interface{}{
		"tenant_id":         tenantID,
		"mics_issued":       847,
		"traffic_bytes":     int64(45956300800), // ~42.8 GB
		"node_hours":        5760.0,
		"total_charge_cents": 521200,
	}
	writeJSON(w, http.StatusOK, usage)
}
