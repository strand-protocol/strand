import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import { PlatformConfig, resourceName } from "./config";
import { ServerInfo } from "./servers";

// ---------------------------------------------------------------------------
// Monitoring stack configuration
//
// Generates Prometheus scrape targets dynamically from the server inventory
// and pushes the configuration to the monitoring node.
// ---------------------------------------------------------------------------

const SSH_OPTS = "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30";

/**
 * Generate a Prometheus scrape config YAML fragment for all nodes.
 */
function generatePromConfig(
    config: PlatformConfig,
    allServers: ServerInfo[],
): pulumi.Output<string> {
    const targetsOutput = pulumi.all(
        allServers.map(s => pulumi.all([s.privateIp, pulumi.output(s.hostname)]))
    ).apply(servers => {
        const targets = servers
            .map(([ip, hostname]) => `        - ${ip}:9100  # ${hostname} (node_exporter)`)
            .join("\n");

        const strandTargets = servers
            .filter((_, i) => allServers[i].role === "control-plane" || allServers[i].role === "worker")
            .map(([ip, hostname]) => `        - ${ip}:8080  # ${hostname} (strand-cloud metrics)`)
            .join("\n");

        return `global:
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
${targets}

  - job_name: "strand_cloud"
    metrics_path: /metrics
    static_configs:
      - targets:
${strandTargets}

  - job_name: "postgres"
    static_configs:
      - targets:
        - localhost:9187  # postgres_exporter

  - job_name: "clickhouse"
    static_configs:
      - targets:
        - localhost:9363  # clickhouse_exporter
`;
    });

    return targetsOutput;
}

/**
 * Generate a Grafana provisioning datasource config.
 */
function grafanaDatasourceConfig(): string {
    return `apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://localhost:9090
    isDefault: true
    editable: false

  - name: ClickHouse
    type: grafana-clickhouse-datasource
    access: proxy
    url: http://localhost:8123
    jsonData:
      defaultDatabase: strand_telemetry
    editable: false
`;
}

/**
 * Generate a Caddy reverse-proxy config for HTTPS termination.
 */
function caddyConfig(config: PlatformConfig): string {
    return `{
    email ops@${config.domain}
}

api.${config.domain} {
    reverse_proxy localhost:8080
}

grafana.${config.domain} {
    reverse_proxy localhost:3000
}
`;
}

/**
 * Push monitoring configuration to the monitoring node and reload services.
 */
export function configureMonitoring(
    config: PlatformConfig,
    monitoringServer: ServerInfo,
    allServers: ServerInfo[],
    dependsOn?: pulumi.Resource[],
): command.local.Command[] {
    const commands: command.local.Command[] = [];

    if (!config.monitoring.enabled) {
        return commands;
    }

    const promConfig = generatePromConfig(config, allServers);
    const grafanaDatasource = grafanaDatasourceConfig();
    const caddyCfg = caddyConfig(config);

    // Push Prometheus config
    const promCmd = new command.local.Command(resourceName("monitoring-prom-config"), {
        create: pulumi.interpolate`ssh ${SSH_OPTS} root@${monitoringServer.publicIp} bash -s <<'SCRIPT'
mkdir -p /etc/prometheus
cat > /etc/prometheus/prometheus.yml <<'PROMEOF'
${promConfig}
PROMEOF
systemctl reload prometheus 2>/dev/null || true
SCRIPT`,
    }, { dependsOn });
    commands.push(promCmd);

    // Push Grafana datasource config
    const grafanaCmd = new command.local.Command(resourceName("monitoring-grafana-ds"), {
        create: pulumi.interpolate`ssh ${SSH_OPTS} root@${monitoringServer.publicIp} bash -s <<'SCRIPT'
mkdir -p /etc/grafana/provisioning/datasources
cat > /etc/grafana/provisioning/datasources/strand.yaml <<'GRAFEOF'
${grafanaDatasource}
GRAFEOF
systemctl restart grafana-server 2>/dev/null || true
SCRIPT`,
    }, { dependsOn: [promCmd] });
    commands.push(grafanaCmd);

    // Push Caddy config for HTTPS reverse proxy
    const caddyCmd = new command.local.Command(resourceName("monitoring-caddy-config"), {
        create: pulumi.interpolate`ssh ${SSH_OPTS} root@${monitoringServer.publicIp} bash -s <<'SCRIPT'
mkdir -p /etc/caddy
cat > /etc/caddy/Caddyfile <<'CADDYEOF'
${caddyCfg}
CADDYEOF
systemctl reload caddy 2>/dev/null || true
SCRIPT`,
    }, { dependsOn: [grafanaCmd] });
    commands.push(caddyCmd);

    return commands;
}
