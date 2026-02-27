package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UsageRecord represents a billing period's usage for a tenant.
type UsageRecord struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	PeriodStart          time.Time `json:"period_start"`
	PeriodEnd            time.Time `json:"period_end"`
	MICsIssued           int       `json:"mics_issued"`
	TrafficBytes         int64     `json:"traffic_bytes"`
	NodeHours            float64   `json:"node_hours"`
	OverageMICs          int       `json:"overage_mics"`
	OverageTrafficBytes  int64     `json:"overage_traffic_bytes"`
	TotalChargeCents     int       `json:"total_charge_cents"`
	Finalized            bool      `json:"finalized"`
}

// Meter tracks usage for billing purposes.
type Meter struct {
	db *sql.DB
}

// NewMeter creates a new billing meter.
func NewMeter(db *sql.DB) *Meter {
	return &Meter{db: db}
}

// IncrementMICs records MIC issuances for the current billing period.
func (m *Meter) IncrementMICs(ctx context.Context, tenantID string, count int) error {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO usage_records (tenant_id, period_start, period_end, mics_issued)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, period_start, period_end) WHERE NOT finalized
		 DO UPDATE SET mics_issued = usage_records.mics_issued + $4,
		              updated_at = now()`,
		tenantID, periodStart, periodEnd, count,
	)
	if err != nil {
		return fmt.Errorf("meter: increment mics: %w", err)
	}
	return nil
}

// GetCurrentUsage returns the current period's usage for a tenant.
func (m *Meter) GetCurrentUsage(ctx context.Context, tenantID string) (*UsageRecord, error) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var r UsageRecord
	err := m.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, period_start, period_end, mics_issued, traffic_bytes,
		        node_hours, overage_mics, overage_traffic_bytes, total_charge_cents, finalized
		 FROM usage_records WHERE tenant_id = $1 AND period_start = $2`,
		tenantID, periodStart,
	).Scan(
		&r.ID, &r.TenantID, &r.PeriodStart, &r.PeriodEnd,
		&r.MICsIssued, &r.TrafficBytes, &r.NodeHours,
		&r.OverageMICs, &r.OverageTrafficBytes, &r.TotalChargeCents, &r.Finalized,
	)
	if err == sql.ErrNoRows {
		return &UsageRecord{
			TenantID:    tenantID,
			PeriodStart: periodStart,
			PeriodEnd:   periodStart.AddDate(0, 1, 0),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("meter: get usage: %w", err)
	}
	return &r, nil
}
