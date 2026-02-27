-- 000005_create_usage.up.sql
-- Per-tenant usage records for billing. Aggregated from ClickHouse telemetry.

CREATE TABLE usage_records (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id),
    period_start            DATE NOT NULL,
    period_end              DATE NOT NULL,
    mics_issued             INT NOT NULL DEFAULT 0,
    traffic_bytes           BIGINT NOT NULL DEFAULT 0,
    node_hours              NUMERIC(12,2) NOT NULL DEFAULT 0,
    overage_mics            INT NOT NULL DEFAULT 0,
    overage_traffic_bytes   BIGINT NOT NULL DEFAULT 0,
    total_charge_cents      INT NOT NULL DEFAULT 0,
    finalized               BOOLEAN NOT NULL DEFAULT false,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_usage_tenant_period ON usage_records(tenant_id, period_start);
CREATE UNIQUE INDEX idx_usage_tenant_period_unique
    ON usage_records(tenant_id, period_start, period_end) WHERE NOT finalized;

CREATE TRIGGER usage_records_updated_at
    BEFORE UPDATE ON usage_records
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
