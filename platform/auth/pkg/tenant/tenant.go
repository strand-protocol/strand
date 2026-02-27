// Package tenant provides multi-tenancy enforcement via PostgreSQL Row Level Security.
package tenant

import "time"

// Tenant represents a platform tenant (organization).
type Tenant struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	Plan               string    `json:"plan"`
	Status             string    `json:"status"`
	MaxClusters        int       `json:"max_clusters"`
	MaxNodes           int       `json:"max_nodes"`
	MaxMICsMonth       int       `json:"max_mics_month"`
	TrafficGBIncluded  float64   `json:"traffic_gb_included"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// PlanLimits returns the limits for a given plan name.
func PlanLimits(plan string) (maxClusters, maxNodes, maxMICs int, trafficGB float64) {
	switch plan {
	case "free":
		return 1, 3, 100, 1.0
	case "starter":
		return 1, 10, 1000, 10.0
	case "pro":
		return 3, 50, 10000, 100.0
	case "enterprise":
		return 999, 9999, 999999, 10000.0
	default:
		return 1, 3, 100, 1.0
	}
}
