package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TelemetryMeter writes usage events to ClickHouse for real-time metering.
type TelemetryMeter struct {
	ch *sql.DB
}

// NewTelemetryMeter creates a new ClickHouse-backed telemetry meter.
func NewTelemetryMeter(ch *sql.DB) *TelemetryMeter {
	return &TelemetryMeter{ch: ch}
}

// RecordTraffic writes a traffic event to the ClickHouse strand_traffic table.
func (m *TelemetryMeter) RecordTraffic(ctx context.Context, tenantID, nodeID, direction string, bytes int64) error {
	_, err := m.ch.ExecContext(ctx,
		`INSERT INTO strand_telemetry.strand_traffic
		 (tenant_id, node_id, direction, bytes, timestamp)
		 VALUES (?, ?, ?, ?, ?)`,
		tenantID, nodeID, direction, bytes, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("telemetry: record traffic: %w", err)
	}
	return nil
}

// RecordMICIssuance writes a MIC issuance event for billing aggregation.
func (m *TelemetryMeter) RecordMICIssuance(ctx context.Context, tenantID, nodeID, micID string) error {
	_, err := m.ch.ExecContext(ctx,
		`INSERT INTO strand_telemetry.strand_telemetry
		 (tenant_id, node_id, metric_name, metric_value, timestamp)
		 VALUES (?, ?, 'mic_issued', 1, ?)`,
		tenantID, nodeID, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("telemetry: record mic: %w", err)
	}
	return nil
}

// TrafficSummary holds aggregated traffic data for a billing period.
type TrafficSummary struct {
	TenantID   string  `json:"tenant_id"`
	TotalBytes int64   `json:"total_bytes"`
	TotalGB    float64 `json:"total_gb"`
}

// GetTrafficSummary returns aggregated traffic for a tenant in the current billing period.
func (m *TelemetryMeter) GetTrafficSummary(ctx context.Context, tenantID string) (*TrafficSummary, error) {
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var totalBytes int64
	err := m.ch.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(bytes), 0)
		 FROM strand_telemetry.strand_traffic
		 WHERE tenant_id = ? AND timestamp >= ?`,
		tenantID, periodStart,
	).Scan(&totalBytes)
	if err != nil {
		return nil, fmt.Errorf("telemetry: traffic summary: %w", err)
	}

	return &TrafficSummary{
		TenantID:   tenantID,
		TotalBytes: totalBytes,
		TotalGB:    float64(totalBytes) / (1024 * 1024 * 1024),
	}, nil
}
