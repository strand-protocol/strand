package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// InvoiceCalculator generates billing totals for a period.
type InvoiceCalculator struct {
	db *sql.DB
}

// NewInvoiceCalculator creates a new invoice calculator.
func NewInvoiceCalculator(db *sql.DB) *InvoiceCalculator {
	return &InvoiceCalculator{db: db}
}

// Invoice represents a calculated billing invoice for a tenant.
type Invoice struct {
	TenantID            string  `json:"tenant_id"`
	Plan                string  `json:"plan"`
	PeriodStart         string  `json:"period_start"`
	PeriodEnd           string  `json:"period_end"`
	BasePriceCents      int     `json:"base_price_cents"`
	MICsUsed            int     `json:"mics_used"`
	MICsIncluded        int     `json:"mics_included"`
	MICOverageCount     int     `json:"mic_overage_count"`
	MICOverageCents     int     `json:"mic_overage_cents"`
	TrafficGB           float64 `json:"traffic_gb"`
	TrafficGBIncluded   float64 `json:"traffic_gb_included"`
	TrafficOverageGB    float64 `json:"traffic_overage_gb"`
	TrafficOverageCents int     `json:"traffic_overage_cents"`
	TotalCents          int     `json:"total_cents"`
}

// Calculate generates an invoice for the current billing period.
func (ic *InvoiceCalculator) Calculate(ctx context.Context, tenantID string) (*Invoice, error) {
	// Get current usage
	meter := NewMeter(ic.db)
	usage, err := meter.GetCurrentUsage(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("invoice: get usage: %w", err)
	}

	// Get tenant plan
	var planName string
	err = ic.db.QueryRowContext(ctx, `SELECT plan FROM tenants WHERE id = $1`, tenantID).Scan(&planName)
	if err != nil {
		return nil, fmt.Errorf("invoice: get plan: %w", err)
	}

	plan, ok := GetPlan(planName)
	if !ok {
		return nil, fmt.Errorf("invoice: unknown plan %q", planName)
	}

	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	inv := &Invoice{
		TenantID:          tenantID,
		Plan:              planName,
		PeriodStart:       periodStart.Format("2006-01-02"),
		PeriodEnd:         periodEnd.Format("2006-01-02"),
		BasePriceCents:    plan.BasePriceCents,
		MICsUsed:          usage.MICsIssued,
		MICsIncluded:      plan.MICsIncluded,
		TrafficGB:         float64(usage.TrafficBytes) / (1024 * 1024 * 1024),
		TrafficGBIncluded: float64(plan.TrafficGBIncluded),
	}

	// Calculate MIC overage
	if usage.MICsIssued > plan.MICsIncluded {
		inv.MICOverageCount = usage.MICsIssued - plan.MICsIncluded
		inv.MICOverageCents = inv.MICOverageCount * plan.MICOverageCents
	}

	// Calculate traffic overage
	trafficUsedGB := inv.TrafficGB
	if trafficUsedGB > float64(plan.TrafficGBIncluded) {
		inv.TrafficOverageGB = trafficUsedGB - float64(plan.TrafficGBIncluded)
		inv.TrafficOverageCents = int(inv.TrafficOverageGB * float64(plan.TrafficOverageCents))
	}

	inv.TotalCents = plan.BasePriceCents + inv.MICOverageCents + inv.TrafficOverageCents

	return inv, nil
}
