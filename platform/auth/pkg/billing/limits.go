package billing

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
)

// LimitChecker enforces plan limits per tenant.
type LimitChecker struct {
	db *sql.DB
}

// NewLimitChecker creates a new limit checker.
func NewLimitChecker(db *sql.DB) *LimitChecker {
	return &LimitChecker{db: db}
}

// CheckNodeLimit returns an error if the tenant has reached their max node count.
func (lc *LimitChecker) CheckNodeLimit(ctx context.Context, tenantID string) error {
	var plan string
	var currentNodes int
	err := lc.db.QueryRowContext(ctx,
		`SELECT t.plan, COALESCE(SUM(c.node_count), 0)
		 FROM tenants t LEFT JOIN clusters c ON c.tenant_id = t.id
		 WHERE t.id = $1 GROUP BY t.plan`,
		tenantID,
	).Scan(&plan, &currentNodes)
	if err != nil {
		return fmt.Errorf("limits: check nodes: %w", err)
	}

	p, ok := GetPlan(plan)
	if !ok {
		return fmt.Errorf("limits: unknown plan %q", plan)
	}

	maxTotal := p.MaxClusters * p.MaxNodesPerCluster
	if currentNodes >= maxTotal {
		return fmt.Errorf("plan limit reached: %d/%d nodes (upgrade to add more)", currentNodes, maxTotal)
	}
	return nil
}

// CheckClusterLimit returns an error if the tenant has reached their max cluster count.
func (lc *LimitChecker) CheckClusterLimit(ctx context.Context, tenantID string) error {
	var plan string
	var currentClusters int
	err := lc.db.QueryRowContext(ctx,
		`SELECT t.plan, COUNT(c.id)
		 FROM tenants t LEFT JOIN clusters c ON c.tenant_id = t.id
		 WHERE t.id = $1 GROUP BY t.plan`,
		tenantID,
	).Scan(&plan, &currentClusters)
	if err != nil {
		return fmt.Errorf("limits: check clusters: %w", err)
	}

	p, ok := GetPlan(plan)
	if !ok {
		return fmt.Errorf("limits: unknown plan %q", plan)
	}

	if currentClusters >= p.MaxClusters {
		return fmt.Errorf("plan limit reached: %d/%d clusters (upgrade to add more)", currentClusters, p.MaxClusters)
	}
	return nil
}

// CheckMICLimit returns an error if the tenant has reached their monthly MIC limit (free plan only).
func (lc *LimitChecker) CheckMICLimit(ctx context.Context, tenantID string) error {
	meter := NewMeter(lc.db)
	usage, err := meter.GetCurrentUsage(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("limits: check mics: %w", err)
	}

	var plan string
	err = lc.db.QueryRowContext(ctx, `SELECT plan FROM tenants WHERE id = $1`, tenantID).Scan(&plan)
	if err != nil {
		return fmt.Errorf("limits: get plan: %w", err)
	}

	p, ok := GetPlan(plan)
	if !ok {
		return fmt.Errorf("limits: unknown plan %q", plan)
	}

	// Free plan has hard limits (no overage)
	if plan == "free" && usage.MICsIssued >= p.MICsIncluded {
		return fmt.Errorf("plan limit reached: %d/%d MICs this month (upgrade to issue more)", usage.MICsIssued, p.MICsIncluded)
	}

	return nil
}

// EnforceLimitsMiddleware returns a 402 Payment Required when plan limits are exceeded.
func EnforceLimitsMiddleware(checker *LimitChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit enforcement is done per-handler, not globally.
			// This middleware is a placeholder for the pattern.
			next.ServeHTTP(w, r)
		})
	}
}
