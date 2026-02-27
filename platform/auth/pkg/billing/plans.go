// Package billing defines pricing plans, metering, and limit enforcement.
package billing

// Plan represents a pricing tier with its limits and rates.
type Plan struct {
	Name                string `json:"name"`
	DisplayName         string `json:"display_name"`
	BasePriceCents      int    `json:"base_price_cents"`      // Monthly base price
	MaxClusters         int    `json:"max_clusters"`
	MaxNodesPerCluster  int    `json:"max_nodes_per_cluster"`
	MICsIncluded        int    `json:"mics_included"`
	MICOverageCents     int    `json:"mic_overage_cents"`      // Per-MIC overage
	TrafficGBIncluded   int    `json:"traffic_gb_included"`
	TrafficOverageCents int    `json:"traffic_overage_cents"`  // Per-GB overage
	SupportLevel        string `json:"support_level"`
	UptimeSLA           string `json:"uptime_sla"`
}

// Plans defines all available pricing tiers.
var Plans = map[string]Plan{
	"free": {
		Name:                "free",
		DisplayName:         "Free",
		BasePriceCents:      0,
		MaxClusters:         1,
		MaxNodesPerCluster:  3,
		MICsIncluded:        100,
		MICOverageCents:     0, // No overage on free -- hard limit
		TrafficGBIncluded:   1,
		TrafficOverageCents: 0, // No overage on free -- hard limit
		SupportLevel:        "community",
		UptimeSLA:           "none",
	},
	"starter": {
		Name:                "starter",
		DisplayName:         "Starter",
		BasePriceCents:      50000, // $500/mo
		MaxClusters:         1,
		MaxNodesPerCluster:  10,
		MICsIncluded:        1000,
		MICOverageCents:     300, // $3/MIC
		TrafficGBIncluded:   10,
		TrafficOverageCents: 8, // $0.08/GB
		SupportLevel:        "email",
		UptimeSLA:           "99.5%",
	},
	"pro": {
		Name:                "pro",
		DisplayName:         "Pro",
		BasePriceCents:      500000, // $5,000/mo
		MaxClusters:         3,
		MaxNodesPerCluster:  50,
		MICsIncluded:        10000,
		MICOverageCents:     250, // $2.50/MIC
		TrafficGBIncluded:   100,
		TrafficOverageCents: 5, // $0.05/GB
		SupportLevel:        "priority",
		UptimeSLA:           "99.9%",
	},
	"enterprise": {
		Name:                "enterprise",
		DisplayName:         "Enterprise",
		BasePriceCents:      1500000, // $15,000/mo minimum
		MaxClusters:         999,
		MaxNodesPerCluster:  9999,
		MICsIncluded:        999999,
		MICOverageCents:     200, // $2/MIC
		TrafficGBIncluded:   10000,
		TrafficOverageCents: 2, // $0.02/GB
		SupportLevel:        "dedicated",
		UptimeSLA:           "99.99%",
	},
}

// GetPlan returns the plan definition for a given plan name.
func GetPlan(name string) (Plan, bool) {
	p, ok := Plans[name]
	return p, ok
}
