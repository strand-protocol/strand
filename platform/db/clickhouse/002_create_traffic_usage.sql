-- 002_create_traffic_usage.sql
-- ClickHouse table for per-frame traffic metering (billing source of truth).
-- Materialized view rolls up hourly aggregates for efficient billing queries.

CREATE TABLE IF NOT EXISTS strand_traffic (
    tenant_id           UUID,
    cluster_id          UUID,
    src_node_id         String,
    dst_node_id         String,
    bytes_transferred   UInt64,
    frame_count         UInt64,
    qos_class           UInt8,       -- 0=BE, 1=RO, 2=RU, 3=PR
    timestamp           DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, cluster_id, timestamp)
TTL toDateTime(timestamp) + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- Hourly rollup for billing aggregation
CREATE MATERIALIZED VIEW IF NOT EXISTS strand_traffic_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (tenant_id, cluster_id, hour)
AS SELECT
    tenant_id,
    cluster_id,
    toStartOfHour(timestamp) AS hour,
    sum(bytes_transferred)   AS total_bytes,
    sum(frame_count)         AS total_frames
FROM strand_traffic
GROUP BY tenant_id, cluster_id, hour;

-- Daily rollup for dashboard overview
CREATE MATERIALIZED VIEW IF NOT EXISTS strand_traffic_daily
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (tenant_id, cluster_id, day)
AS SELECT
    tenant_id,
    cluster_id,
    toStartOfDay(timestamp)  AS day,
    sum(bytes_transferred)   AS total_bytes,
    sum(frame_count)         AS total_frames,
    count()                  AS event_count
FROM strand_traffic
GROUP BY tenant_id, cluster_id, day;
