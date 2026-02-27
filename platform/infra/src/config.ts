import * as pulumi from "@pulumi/pulumi";

// ---------------------------------------------------------------------------
// Stack configuration -- reads values from Pulumi.<stack>.yaml
// ---------------------------------------------------------------------------

const cfg = new pulumi.Config("strand-platform");

export type Environment = "dev" | "staging" | "prod";

export interface ServerTier {
    count: number;
    type: string;
    colocated?: boolean;
}

export interface StorageConfig {
    dbDiskSizeGb: number;
}

export interface MonitoringConfig {
    enabled: boolean;
    grafanaAdminPassword: string;
}

export interface NetworkConfig {
    cidr: string;
    vlanId: number;
}

export interface PlatformConfig {
    environment: Environment;
    region: string;
    sshKeyId: string;
    bmcClientId: string;
    bmcClientSecret: pulumi.Output<string>;
    domain: string;
    controlPlane: ServerTier;
    workers: ServerTier;
    database: ServerTier;
    storage: StorageConfig;
    monitoring: MonitoringConfig;
    network: NetworkConfig;
}

/** PhoenixNAP BMC API base URL. */
export const BMC_API_BASE = "https://api.phoenixnap.com/bmc/v1";

/** PhoenixNAP auth token endpoint. */
export const BMC_AUTH_URL = "https://auth.phoenixnap.com/auth/realms/BMC/protocol/openid-connect/token";

/**
 * Load and validate the full platform configuration for the current stack.
 */
export function loadConfig(): PlatformConfig {
    const environment = cfg.require("environment") as Environment;
    if (!["dev", "staging", "prod"].includes(environment)) {
        throw new Error(`Invalid environment: ${environment}. Must be dev, staging, or prod.`);
    }

    const controlPlane = cfg.requireObject<ServerTier>("controlPlane");
    const workers = cfg.requireObject<ServerTier>("workers");
    const database = cfg.requireObject<ServerTier>("database");
    const storage = cfg.requireObject<StorageConfig>("storage");
    const monitoring = cfg.requireObject<MonitoringConfig>("monitoring");
    const network = cfg.requireObject<NetworkConfig>("network");

    return {
        environment,
        region: cfg.require("region"),
        sshKeyId: cfg.require("sshKeyId"),
        bmcClientId: cfg.require("bmcClientId"),
        bmcClientSecret: cfg.requireSecret("bmcClientSecret"),
        domain: cfg.require("domain"),
        controlPlane,
        workers,
        database,
        storage,
        monitoring: {
            enabled: monitoring.enabled,
            grafanaAdminPassword: cfg.get("monitoring")
                ? (monitoring as any).grafanaAdminPassword ?? "admin"
                : "admin",
        },
        network,
    };
}

/**
 * Return a human-readable label for resources. Keeps names short and
 * deterministic so Pulumi can track them across updates.
 */
export function resourceName(base: string, index?: number): string {
    const stack = pulumi.getStack();
    const suffix = index !== undefined ? `-${index}` : "";
    return `strand-${stack}-${base}${suffix}`;
}

/**
 * Standard tags applied to every BMC resource that supports tagging.
 */
export function standardTags(extra?: Record<string, string>): Record<string, string> {
    const stack = pulumi.getStack();
    return {
        "strand:stack": stack,
        "strand:managed-by": "pulumi",
        "strand:project": "strand-platform",
        ...extra,
    };
}
