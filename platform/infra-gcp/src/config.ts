import * as pulumi from "@pulumi/pulumi";

// ---------------------------------------------------------------------------
// Stack configuration -- reads values from Pulumi.<stack>.yaml
// ---------------------------------------------------------------------------

const cfg = new pulumi.Config("strand-gcp");

export type Environment = "dev" | "staging" | "prod";

export interface NodePoolConfig {
    minCount: number;
    maxCount: number;
    machineType: string;
    diskSizeGb: number;
    preemptible: boolean;
}

export interface GpuNodePoolConfig extends NodePoolConfig {
    gpuType: string;
    gpuCount: number;
    gpuDriverVersion: string;
}

export interface GkeConfig {
    systemPool: NodePoolConfig;
    controlPlanePool: NodePoolConfig;
    gpuInferencePool: GpuNodePoolConfig;
    releaseChannel: "RAPID" | "REGULAR" | "STABLE";
    masterAuthorizedNetworkCidr: string;
}

export interface NetworkConfig {
    podCidr: string;
    serviceCidr: string;
    masterCidr: string;
}

export interface BillingConfig {
    billingAccountId: string;
    bigQueryDatasetId: string;
    enableUsageMetering: boolean;
    billingExportDatasetId: string;
}

export interface MonitoringConfig {
    enableManagedPrometheus: boolean;
    enableCloudMonitoring: boolean;
    alertNotificationChannel: string;
}

export interface StorageConfig {
    etcdDiskSizeGb: number;
    etcdDiskType: string;
}

export interface NimConfig {
    enabled: boolean;
    image: string;
    modelName: string;
    gpuCount: number;
    replicas: number;
    maxReplicas: number;
}

export interface TenantConfig {
    name: string;
    id: string;
    plan: "free" | "starter" | "pro" | "enterprise";
}

export interface GcpPlatformConfig {
    environment: Environment;
    gcpProject: string;
    gcpRegion: string;
    gcpZone: string;
    domain: string;
    gke: GkeConfig;
    network: NetworkConfig;
    billing: BillingConfig;
    monitoring: MonitoringConfig;
    storage: StorageConfig;
    nim: NimConfig;
    tenants: TenantConfig[];
}

/**
 * Load and validate the full GCP platform configuration for the current stack.
 */
export function loadConfig(): GcpPlatformConfig {
    const environment = cfg.require("environment") as Environment;
    if (!["dev", "staging", "prod"].includes(environment)) {
        throw new Error(`Invalid environment: ${environment}. Must be dev, staging, or prod.`);
    }

    return {
        environment,
        gcpProject: cfg.require("gcpProject"),
        gcpRegion: cfg.require("gcpRegion"),
        gcpZone: cfg.require("gcpZone"),
        domain: cfg.require("domain"),
        gke: cfg.requireObject<GkeConfig>("gke"),
        network: cfg.requireObject<NetworkConfig>("network"),
        billing: cfg.requireObject<BillingConfig>("billing"),
        monitoring: cfg.requireObject<MonitoringConfig>("monitoring"),
        storage: cfg.requireObject<StorageConfig>("storage"),
        nim: cfg.getObject<NimConfig>("nim") ?? {
            enabled: true,
            image: "nvcr.io/nim/meta/llama-3.1-8b-instruct:latest",
            modelName: "meta/llama-3.1-8b-instruct",
            gpuCount: 1,
            replicas: environment === "prod" ? 2 : 1,
            maxReplicas: environment === "prod" ? 10 : 2,
        },
        tenants: cfg.getObject<TenantConfig[]>("tenants") ?? [],
    };
}

/**
 * Deterministic resource name scoped to the current stack.
 */
export function resourceName(base: string, index?: number): string {
    const stack = pulumi.getStack();
    const suffix = index !== undefined ? `-${index}` : "";
    return `strand-${stack}-${base}${suffix}`;
}

/**
 * Standard GCP labels applied to every resource.
 */
export function standardLabels(extra?: Record<string, string>): Record<string, string> {
    const stack = pulumi.getStack();
    return {
        "strand-stack": stack,
        "strand-managed-by": "pulumi",
        "strand-project": "strand-platform-gcp",
        ...extra,
    };
}
