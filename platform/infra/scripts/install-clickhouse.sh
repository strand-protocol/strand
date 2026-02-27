#!/usr/bin/env bash
# install-clickhouse.sh -- Install and configure ClickHouse for Strand telemetry
#
# Expected environment variables:
#   STRAND_ENV          - Environment (dev/staging/prod)
#   STRAND_CH_DATA_DIR  - Data directory (e.g. /data/strand/clickhouse)
set -euo pipefail

STRAND_CH_DATA_DIR="${STRAND_CH_DATA_DIR:-/data/strand/clickhouse}"

echo "==> Installing ClickHouse..."

# Add ClickHouse APT repository
if [ ! -f /etc/apt/sources.list.d/clickhouse.list ]; then
    curl -fsSL https://packages.clickhouse.com/rpm/lts/repodata/repomd.xml.key \
        | gpg --dearmor -o /usr/share/keyrings/clickhouse-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/clickhouse-keyring.gpg] \
        https://packages.clickhouse.com/deb stable main" \
        > /etc/apt/sources.list.d/clickhouse.list
fi

apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    clickhouse-server \
    clickhouse-client

echo "==> Configuring ClickHouse..."

# Stop ClickHouse to reconfigure
systemctl stop clickhouse-server 2>/dev/null || true

# Create data directories
mkdir -p "${STRAND_CH_DATA_DIR}"/{data,tmp,user_files,format_schemas}
chown -R clickhouse:clickhouse "${STRAND_CH_DATA_DIR}"

# Main ClickHouse config override
mkdir -p /etc/clickhouse-server/config.d
cat > /etc/clickhouse-server/config.d/strand.xml <<CHCONF
<?xml version="1.0"?>
<clickhouse>
    <!-- Storage paths -->
    <path>${STRAND_CH_DATA_DIR}/data/</path>
    <tmp_path>${STRAND_CH_DATA_DIR}/tmp/</tmp_path>
    <user_files_path>${STRAND_CH_DATA_DIR}/user_files/</user_files_path>
    <format_schema_path>${STRAND_CH_DATA_DIR}/format_schemas/</format_schema_path>

    <!-- Listen on all interfaces for private network access -->
    <listen_host>0.0.0.0</listen_host>

    <!-- HTTP interface -->
    <http_port>8123</http_port>

    <!-- Native protocol -->
    <tcp_port>9000</tcp_port>

    <!-- Logging -->
    <logger>
        <level>information</level>
        <log>/var/log/strand/clickhouse/clickhouse-server.log</log>
        <errorlog>/var/log/strand/clickhouse/clickhouse-server.err.log</errorlog>
        <size>100M</size>
        <count>10</count>
    </logger>

    <!-- Prometheus metrics endpoint -->
    <prometheus>
        <endpoint>/metrics</endpoint>
        <port>9363</port>
        <metrics>true</metrics>
        <events>true</events>
        <asynchronous_metrics>true</asynchronous_metrics>
    </prometheus>

    <!-- MergeTree settings tuned per environment -->
    <merge_tree>
        <max_suspicious_broken_parts>5</max_suspicious_broken_parts>
    </merge_tree>

    <!-- Mark cache -- adjust per environment -->
CHCONF

case "${STRAND_ENV}" in
    dev)
        cat >> /etc/clickhouse-server/config.d/strand.xml <<CHMEM
    <mark_cache_size>268435456</mark_cache_size>
    <max_server_memory_usage_to_ram_ratio>0.5</max_server_memory_usage_to_ram_ratio>
CHMEM
        ;;
    staging)
        cat >> /etc/clickhouse-server/config.d/strand.xml <<CHMEM
    <mark_cache_size>1073741824</mark_cache_size>
    <max_server_memory_usage_to_ram_ratio>0.7</max_server_memory_usage_to_ram_ratio>
CHMEM
        ;;
    prod)
        cat >> /etc/clickhouse-server/config.d/strand.xml <<CHMEM
    <mark_cache_size>4294967296</mark_cache_size>
    <max_server_memory_usage_to_ram_ratio>0.8</max_server_memory_usage_to_ram_ratio>
CHMEM
        ;;
esac

cat >> /etc/clickhouse-server/config.d/strand.xml <<CHCONF
</clickhouse>
CHCONF

# Create log directory
mkdir -p /var/log/strand/clickhouse
chown clickhouse:clickhouse /var/log/strand/clickhouse

# Start ClickHouse
echo "==> Starting ClickHouse..."
systemctl start clickhouse-server
systemctl enable clickhouse-server

# Wait for ClickHouse to be ready
for i in $(seq 1 30); do
    if clickhouse-client --query "SELECT 1" > /dev/null 2>&1; then
        break
    fi
    sleep 2
done

# Create the strand_telemetry database and tables
echo "==> Creating strand_telemetry database and tables..."
clickhouse-client --multiquery <<CHSQL
CREATE DATABASE IF NOT EXISTS strand_telemetry;

-- Request traces -- every inference request/response pair
CREATE TABLE IF NOT EXISTS strand_telemetry.request_traces (
    trace_id        UUID,
    request_id      UInt32,
    source_node_id  FixedString(16),
    dest_node_id    FixedString(16),
    message_type    UInt16,
    status_code     UInt16,
    latency_us      UInt64,
    payload_bytes   UInt32,
    tokens_in       UInt32,
    tokens_out      UInt32,
    model_arch      String,
    region          LowCardinality(String),
    tenant_id       UUID,
    metadata        String,
    timestamp       DateTime64(6, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (tenant_id, timestamp, trace_id)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Node metrics -- periodic snapshots from each node
CREATE TABLE IF NOT EXISTS strand_telemetry.node_metrics (
    node_id             FixedString(16),
    hostname            LowCardinality(String),
    role                LowCardinality(String),
    cpu_usage_pct       Float32,
    memory_usage_pct    Float32,
    disk_usage_pct      Float32,
    network_rx_bytes    UInt64,
    network_tx_bytes    UInt64,
    active_streams      UInt32,
    active_connections  UInt32,
    overlay_rtt_us      UInt32,
    region              LowCardinality(String),
    timestamp           DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (node_id, timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Stream events -- connection opens, closes, errors
CREATE TABLE IF NOT EXISTS strand_telemetry.stream_events (
    event_id        UUID,
    stream_id       UInt32,
    connection_id   UUID,
    source_node_id  FixedString(16),
    dest_node_id    FixedString(16),
    event_type      LowCardinality(String),
    delivery_mode   LowCardinality(String),
    bytes_sent      UInt64,
    bytes_received  UInt64,
    packets_lost    UInt32,
    rtt_us          UInt32,
    congestion_window UInt32,
    error_code      UInt16,
    error_message   String,
    timestamp       DateTime64(6, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (source_node_id, timestamp, event_id)
TTL timestamp + INTERVAL 60 DAY
SETTINGS index_granularity = 8192;

-- Route changes -- gossip updates, SAD resolutions
CREATE TABLE IF NOT EXISTS strand_telemetry.route_events (
    event_id        UUID,
    event_type      LowCardinality(String),
    sad_descriptor  String,
    resolved_node   FixedString(16),
    resolution_score Float32,
    resolution_us   UInt32,
    candidates      UInt16,
    region          LowCardinality(String),
    timestamp       DateTime64(6, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, event_id)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Trust events -- handshakes, certificate operations
CREATE TABLE IF NOT EXISTS strand_telemetry.trust_events (
    event_id        UUID,
    event_type      LowCardinality(String),
    initiator_node  FixedString(16),
    responder_node  FixedString(16),
    cipher_suite    UInt16,
    handshake_us    UInt32,
    success         UInt8,
    error_message   String,
    mic_serial      String,
    timestamp       DateTime64(6, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, event_id)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Materialized view: hourly request aggregates per tenant
CREATE MATERIALIZED VIEW IF NOT EXISTS strand_telemetry.hourly_request_stats
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (tenant_id, model_arch, hour)
AS SELECT
    tenant_id,
    model_arch,
    toStartOfHour(timestamp) AS hour,
    count() AS request_count,
    sum(tokens_in) AS total_tokens_in,
    sum(tokens_out) AS total_tokens_out,
    sum(payload_bytes) AS total_payload_bytes,
    avg(latency_us) AS avg_latency_us,
    quantile(0.99)(latency_us) AS p99_latency_us
FROM strand_telemetry.request_traces
GROUP BY tenant_id, model_arch, hour;
CHSQL

echo "==> ClickHouse installation complete."
echo "    Database: strand_telemetry"
echo "    HTTP port: 8123"
echo "    Native port: 9000"
echo "    Data dir: ${STRAND_CH_DATA_DIR}"
