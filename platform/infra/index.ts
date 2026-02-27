import * as pulumi from "@pulumi/pulumi";
import { loadConfig } from "./src/config";
import { setupNetwork } from "./src/network";
import { provisionServers } from "./src/servers";
import { provisionStorage } from "./src/storage";
import { provisionAll } from "./src/provision";
import { setupDns } from "./src/dns";
import { configureMonitoring } from "./src/monitoring";

// ---------------------------------------------------------------------------
// Strand Protocol -- PhoenixNAP BMC Infrastructure
//
// Orchestrates the full deployment:
//   1. Private network
//   2. Bare-metal servers (control plane, workers, database)
//   3. Block storage for databases
//   4. Post-provision software installation
//   5. DNS records
//   6. Monitoring configuration
// ---------------------------------------------------------------------------

const config = loadConfig();

// --- 1. Network ---
const network = setupNetwork(config);

// --- 2. Servers ---
const servers = provisionServers(config, network);

// --- 3. Storage ---
const storage = provisionStorage(
    config,
    network,
    servers.databases,
    servers.controlPlane,
);

// --- 4. Post-provision ---
const provisionCommands = provisionAll(
    config,
    servers.controlPlane,
    servers.workers,
    servers.databases,
);

// --- 5. DNS ---
const dns = setupDns(
    config,
    servers.controlPlane,
    servers.workers,
    servers.databases,
);

// --- 6. Monitoring ---
let monitoringCommands: pulumi.Resource[] = [];
if (config.monitoring.enabled && servers.controlPlane.length > 0) {
    monitoringCommands = configureMonitoring(
        config,
        servers.controlPlane[0],
        servers.all,
        provisionCommands,
    );
}

// ---------------------------------------------------------------------------
// Stack outputs
// ---------------------------------------------------------------------------

export const environment = config.environment;
export const region = config.region;
export const domain = config.domain;

export const privateNetworkId = network.privateNetworkId;

export const controlPlaneIps = servers.controlPlane.map(s => s.publicIp);
export const workerIps = servers.workers.map(s => s.publicIp);
export const databaseIps = servers.databases.map(s => s.publicIp);

export const controlPlanePrivateIps = servers.controlPlane.map(s => s.privateIp);
export const workerPrivateIps = servers.workers.map(s => s.privateIp);
export const databasePrivateIps = servers.databases.map(s => s.privateIp);

export const storageVolumes = storage.volumes.map(v => ({
    name: v.name,
    sizeGb: v.sizeGb,
    attachedTo: v.attachedTo,
}));

export const dnsRecords = dns.records.map(r => ({
    name: r.name,
    ip: r.ip,
}));

export const apiEndpoint = pulumi.interpolate`https://api.${config.domain}`;
export const grafanaEndpoint = config.monitoring.enabled
    ? pulumi.interpolate`https://grafana.${config.domain}`
    : pulumi.output("(monitoring disabled)");

export const serverCount = servers.all.length;
