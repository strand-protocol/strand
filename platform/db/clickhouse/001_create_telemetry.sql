-- 001_create_telemetry.sql
-- ClickHouse table for high-throughput node telemetry ingestion.
-- Designed for millions of writes/sec with automatic TTL-based cleanup.

CREATE TABLE IF NOT EXISTS strand_telemetry (
    tenant_id       UUID,
    node_id         String,
    cluster_id      UUID,
    metric_name     String,
    metric_value    Float64,
    timestamp       DateTime64(3),
    labels          Map(String, String),

    INDEX idx_metric metric_name TYPE bloom_filter GRANULARITY 4
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, node_id, metric_name, timestamp)
TTL toDateTime(timestamp) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
