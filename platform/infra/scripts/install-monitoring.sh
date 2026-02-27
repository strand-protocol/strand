#!/usr/bin/env bash
# install-monitoring.sh -- Install Prometheus, Grafana, and Caddy reverse proxy
#
# Expected environment variables:
#   STRAND_ENV               - Environment (dev/staging/prod)
#   STRAND_DOMAIN            - Domain name for TLS certificates
#   GRAFANA_ADMIN_PASSWORD  - Grafana admin password
set -euo pipefail

STRAND_ENV="${STRAND_ENV:-dev}"
STRAND_DOMAIN="${STRAND_DOMAIN:-localhost}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"

echo "==> Installing monitoring stack (Prometheus + Grafana + Caddy)..."

# -------------------------------------------------------------------------
# 1. Prometheus
# -------------------------------------------------------------------------
echo "==> Installing Prometheus..."

PROM_VERSION="2.51.0"
PROM_ARCH="linux-amd64"
PROM_URL="https://github.com/prometheus/prometheus/releases/download/v${PROM_VERSION}/prometheus-${PROM_VERSION}.${PROM_ARCH}.tar.gz"

# Create prometheus user
useradd --system --no-create-home --shell /usr/sbin/nologin prometheus 2>/dev/null || true
mkdir -p /etc/prometheus /var/lib/prometheus /var/log/strand/prometheus
chown prometheus:prometheus /var/lib/prometheus /var/log/strand/prometheus

# Download and install
cd /tmp
curl -fsSL "${PROM_URL}" -o prometheus.tar.gz
tar xzf prometheus.tar.gz
cp "prometheus-${PROM_VERSION}.${PROM_ARCH}/prometheus" /usr/local/bin/
cp "prometheus-${PROM_VERSION}.${PROM_ARCH}/promtool" /usr/local/bin/
cp -r "prometheus-${PROM_VERSION}.${PROM_ARCH}/consoles" /etc/prometheus/
cp -r "prometheus-${PROM_VERSION}.${PROM_ARCH}/console_libraries" /etc/prometheus/
rm -rf prometheus.tar.gz "prometheus-${PROM_VERSION}.${PROM_ARCH}"
cd /

# Default Prometheus config (will be overwritten by Pulumi monitoring module)
cat > /etc/prometheus/prometheus.yml <<'PROMCONF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "prometheus"
    static_configs:
      - targets:
        - localhost:9090

  - job_name: "node_exporter"
    static_configs:
      - targets:
        - localhost:9100
PROMCONF

chown -R prometheus:prometheus /etc/prometheus

# Prometheus alert rules for Strand
cat > /etc/prometheus/strand_alerts.yml <<'ALERTSCONF'
groups:
  - name: strand_platform
    interval: 30s
    rules:
      - alert: NodeDown
        expr: up{job="node_exporter"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Node {{ $labels.instance }} is down"
          description: "Node exporter on {{ $labels.instance }} has been unreachable for 2 minutes."

      - alert: HighCpuUsage
        expr: 100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High CPU usage on {{ $labels.instance }}"
          description: "CPU usage is above 90% for 5 minutes on {{ $labels.instance }}."

      - alert: HighMemoryUsage
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage on {{ $labels.instance }}"
          description: "Memory usage is above 90% for 5 minutes on {{ $labels.instance }}."

      - alert: DiskSpaceLow
        expr: (1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100 > 85
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Disk space low on {{ $labels.instance }}"
          description: "Root filesystem usage is above 85% on {{ $labels.instance }}."

      - alert: StrandCloudDown
        expr: up{job="strand_cloud"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Strand Cloud service down on {{ $labels.instance }}"
          description: "Strand Cloud API server is unreachable on {{ $labels.instance }}."

      - alert: HighOverlayLatency
        expr: histogram_quantile(0.99, rate(strand_overlay_rtt_seconds_bucket[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High StrandLink overlay latency"
          description: "P99 overlay RTT is above 100ms for 5 minutes."

      - alert: PostgresDown
        expr: up{job="postgres"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "PostgreSQL is down"
          description: "PostgreSQL exporter is unreachable."

      - alert: ClickHouseDown
        expr: up{job="clickhouse"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "ClickHouse is down"
          description: "ClickHouse metrics endpoint is unreachable."
ALERTSCONF

# Add alert rules to prometheus config
cat >> /etc/prometheus/prometheus.yml <<'RULESCONF'

rule_files:
  - "strand_alerts.yml"
RULESCONF

# Create Prometheus systemd service
cat > /etc/systemd/system/prometheus.service <<PROMSVC
[Unit]
Description=Prometheus Monitoring System
Documentation=https://prometheus.io/docs/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=prometheus
Group=prometheus
ExecStart=/usr/local/bin/prometheus \
    --config.file=/etc/prometheus/prometheus.yml \
    --storage.tsdb.path=/var/lib/prometheus \
    --storage.tsdb.retention.time=30d \
    --storage.tsdb.retention.size=10GB \
    --web.listen-address=0.0.0.0:9090 \
    --web.console.templates=/etc/prometheus/consoles \
    --web.console.libraries=/etc/prometheus/console_libraries \
    --web.enable-lifecycle
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

StandardOutput=append:/var/log/strand/prometheus/prometheus.log
StandardError=append:/var/log/strand/prometheus/prometheus-error.log

[Install]
WantedBy=multi-user.target
PROMSVC

systemctl daemon-reload
systemctl enable prometheus
systemctl start prometheus

# -------------------------------------------------------------------------
# 2. Grafana
# -------------------------------------------------------------------------
echo "==> Installing Grafana..."

# Add Grafana APT repository
if [ ! -f /etc/apt/sources.list.d/grafana.list ]; then
    curl -fsSL https://apt.grafana.com/gpg.key \
        | gpg --dearmor -o /usr/share/keyrings/grafana-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/grafana-keyring.gpg] \
        https://apt.grafana.com stable main" \
        > /etc/apt/sources.list.d/grafana.list
fi

apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq grafana

# Configure Grafana
cat > /etc/grafana/grafana.ini <<GRAFANACONF
[server]
http_addr = 127.0.0.1
http_port = 3000
domain = grafana.${STRAND_DOMAIN}
root_url = https://grafana.${STRAND_DOMAIN}/
serve_from_sub_path = false

[security]
admin_user = admin
admin_password = ${GRAFANA_ADMIN_PASSWORD}
disable_gravatar = true
cookie_secure = true
cookie_samesite = lax

[users]
allow_sign_up = false
allow_org_create = false
auto_assign_org = true
auto_assign_org_role = Viewer

[auth.anonymous]
enabled = false

[log]
mode = file
level = info

[log.file]
log_rotate = true
max_lines = 1000000
max_size_shift = 28
daily_rotate = true
max_days = 14

[analytics]
reporting_enabled = false
check_for_updates = false

[dashboards]
min_refresh_interval = 10s

[alerting]
enabled = true

[plugins]
allow_loading_unsigned_plugins = grafana-clickhouse-datasource
GRAFANACONF

# Install ClickHouse Grafana plugin
grafana-cli plugins install grafana-clickhouse-datasource 2>/dev/null || true

# Create Grafana provisioning directories
mkdir -p /etc/grafana/provisioning/{datasources,dashboards,notifiers}

# Datasources will be pushed by the Pulumi monitoring module

# Create a dashboard provisioning config
cat > /etc/grafana/provisioning/dashboards/strand.yaml <<'DASHPROV'
apiVersion: 1

providers:
  - name: "Strand Protocol"
    orgId: 1
    folder: "Strand"
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards/strand
      foldersFromFilesStructure: false
DASHPROV

# Create a default Strand overview dashboard
mkdir -p /var/lib/grafana/dashboards/strand
cat > /var/lib/grafana/dashboards/strand/overview.json <<'DASHJSON'
{
  "dashboard": {
    "id": null,
    "uid": "strand-overview",
    "title": "Strand Protocol Overview",
    "tags": ["strand"],
    "timezone": "utc",
    "refresh": "30s",
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "panels": [
      {
        "id": 1,
        "title": "Node Status",
        "type": "stat",
        "gridPos": { "h": 4, "w": 6, "x": 0, "y": 0 },
        "targets": [
          {
            "expr": "count(up{job=\"node_exporter\"} == 1)",
            "legendFormat": "Nodes Up"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                { "value": 0, "color": "red" },
                { "value": 1, "color": "green" }
              ]
            }
          }
        }
      },
      {
        "id": 2,
        "title": "Strand Cloud Services",
        "type": "stat",
        "gridPos": { "h": 4, "w": 6, "x": 6, "y": 0 },
        "targets": [
          {
            "expr": "count(up{job=\"strand_cloud\"} == 1)",
            "legendFormat": "Services Up"
          }
        ]
      },
      {
        "id": 3,
        "title": "CPU Usage by Node",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 0, "y": 4 },
        "targets": [
          {
            "expr": "100 - (avg by(instance) (rate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)",
            "legendFormat": "{{ instance }}"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "max": 100,
            "min": 0
          }
        }
      },
      {
        "id": 4,
        "title": "Memory Usage by Node",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 12, "y": 4 },
        "targets": [
          {
            "expr": "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100",
            "legendFormat": "{{ instance }}"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "max": 100,
            "min": 0
          }
        }
      },
      {
        "id": 5,
        "title": "Network Traffic",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 0, "y": 12 },
        "targets": [
          {
            "expr": "rate(node_network_receive_bytes_total{device!=\"lo\"}[5m])",
            "legendFormat": "{{ instance }} RX"
          },
          {
            "expr": "rate(node_network_transmit_bytes_total{device!=\"lo\"}[5m])",
            "legendFormat": "{{ instance }} TX"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "Bps"
          }
        }
      },
      {
        "id": 6,
        "title": "Disk Usage",
        "type": "gauge",
        "gridPos": { "h": 8, "w": 12, "x": 12, "y": 12 },
        "targets": [
          {
            "expr": "(1 - (node_filesystem_avail_bytes{mountpoint=\"/\"} / node_filesystem_size_bytes{mountpoint=\"/\"})) * 100",
            "legendFormat": "{{ instance }}"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "max": 100,
            "min": 0,
            "thresholds": {
              "steps": [
                { "value": 0, "color": "green" },
                { "value": 70, "color": "yellow" },
                { "value": 85, "color": "red" }
              ]
            }
          }
        }
      }
    ]
  },
  "overwrite": true
}
DASHJSON

chown -R grafana:grafana /var/lib/grafana/dashboards

# Start Grafana
systemctl enable grafana-server
systemctl start grafana-server

# -------------------------------------------------------------------------
# 3. Caddy (HTTPS reverse proxy)
# -------------------------------------------------------------------------
echo "==> Installing Caddy..."

# Add Caddy APT repository
if [ ! -f /etc/apt/sources.list.d/caddy-fury.list ]; then
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
        | gpg --dearmor -o /usr/share/keyrings/caddy-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
        | tee /etc/apt/sources.list.d/caddy-fury.list
fi

apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq caddy

# Configure Caddy
mkdir -p /etc/caddy
cat > /etc/caddy/Caddyfile <<CADDYCONF
{
    email ops@${STRAND_DOMAIN}
}

api.${STRAND_DOMAIN} {
    reverse_proxy localhost:8080 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
    }

    log {
        output file /var/log/strand/caddy/api-access.log {
            roll_size 100MiB
            roll_keep 10
        }
    }
}

grafana.${STRAND_DOMAIN} {
    reverse_proxy localhost:3000 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options nosniff
        X-Frame-Options SAMEORIGIN
        Referrer-Policy strict-origin-when-cross-origin
    }

    log {
        output file /var/log/strand/caddy/grafana-access.log {
            roll_size 100MiB
            roll_keep 10
        }
    }
}
CADDYCONF

# Create Caddy log directory
mkdir -p /var/log/strand/caddy
chown caddy:caddy /var/log/strand/caddy

# Start Caddy
systemctl enable caddy
systemctl start caddy

echo "==> Monitoring stack installation complete."
echo "    Prometheus: http://localhost:9090"
echo "    Grafana: http://localhost:3000 (admin / ${GRAFANA_ADMIN_PASSWORD})"
echo "    Caddy: https://api.${STRAND_DOMAIN}, https://grafana.${STRAND_DOMAIN}"
