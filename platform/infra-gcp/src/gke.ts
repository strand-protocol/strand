import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";
import { NetworkResult } from "./network";
import { IamResult } from "./iam";

export interface GkeResult {
    cluster: gcp.container.Cluster;
    systemPool: gcp.container.NodePool;
    controlPlanePool: gcp.container.NodePool;
    gpuInferencePool: gcp.container.NodePool;
    kubeconfig: pulumi.Output<string>;
}

export function createGkeCluster(
    config: GcpPlatformConfig,
    network: NetworkResult,
    iam: IamResult,
): GkeResult {
    const labels = standardLabels();
    const isProd = config.environment === "prod";

    // GKE cluster (regional for prod, zonal for dev/staging)
    const location = isProd ? config.gcpRegion : config.gcpZone;

    const cluster = new gcp.container.Cluster(resourceName("gke"), {
        name: resourceName("gke"),
        project: config.gcpProject,
        location,
        network: network.vpc.id,
        subnetwork: network.subnet.id,

        // Remove default node pool -- we manage our own
        initialNodeCount: 1,
        removeDefaultNodePool: true,

        // Private cluster
        privateClusterConfig: {
            enablePrivateNodes: true,
            enablePrivateEndpoint: isProd,
            masterIpv4CidrBlock: config.network.masterCidr,
        },

        // Master authorized networks
        masterAuthorizedNetworksConfig: {
            cidrBlocks: [{
                cidrBlock: config.gke.masterAuthorizedNetworkCidr,
                displayName: "authorized-network",
            }],
        },

        // IP allocation for pods and services
        ipAllocationPolicy: {
            clusterSecondaryRangeName: network.podRangeName,
            servicesSecondaryRangeName: network.serviceRangeName,
        },

        // Release channel
        releaseChannel: {
            channel: config.gke.releaseChannel,
        },

        // Cost management
        costManagementConfig: {
            enabled: true,
        },

        // GKE usage metering to BigQuery for billing chargeback
        resourceUsageExportConfig: {
            enableNetworkEgressMetering: true,
            enableResourceConsumptionMetering: true,
            bigqueryDestination: {
                datasetId: config.billing.bigQueryDatasetId,
            },
        },

        // Workload Identity
        workloadIdentityConfig: {
            workloadPool: `${config.gcpProject}.svc.id.goog`,
        },

        // Addons
        addonsConfig: {
            httpLoadBalancing: { disabled: false },
            gcePersistentDiskCsiDriverConfig: { enabled: true },
            dnsCacheConfig: { enabled: true },
        },

        // Managed Prometheus
        monitoringConfig: {
            enableComponents: ["SYSTEM_COMPONENTS"],
            managedPrometheus: { enabled: config.monitoring.enableManagedPrometheus },
        },

        loggingConfig: {
            enableComponents: ["SYSTEM_COMPONENTS", "WORKLOADS"],
        },

        resourceLabels: labels,
    });

    // Node Pool 1: system -- GKE system workloads
    const systemPool = new gcp.container.NodePool(resourceName("system-pool"), {
        name: "system",
        project: config.gcpProject,
        location,
        cluster: cluster.name,
        initialNodeCount: isProd ? 2 : 1,
        autoscaling: {
            minNodeCount: config.gke.systemPool.minCount,
            maxNodeCount: config.gke.systemPool.maxCount,
        },
        management: {
            autoRepair: true,
            autoUpgrade: true,
        },
        nodeConfig: {
            machineType: config.gke.systemPool.machineType,
            diskSizeGb: config.gke.systemPool.diskSizeGb,
            diskType: "pd-standard",
            oauthScopes: ["https://www.googleapis.com/auth/cloud-platform"],
            serviceAccount: iam.nodeServiceAccount.email,
            preemptible: config.gke.systemPool.preemptible,
            labels: { "strand-role": "system", ...labels },
            taints: [{
                key: "node-role",
                value: "system",
                effect: "NO_SCHEDULE",
            }],
            tags: ["gke-node"],
            metadata: {
                "disable-legacy-endpoints": "true",
            },
        },
    });

    // Node Pool 2: control-plane -- strand-cloud, etcd, httpbridge
    const controlPlanePool = new gcp.container.NodePool(resourceName("cp-pool"), {
        name: "control-plane",
        project: config.gcpProject,
        location,
        cluster: cluster.name,
        initialNodeCount: isProd ? 3 : 1,
        autoscaling: {
            minNodeCount: config.gke.controlPlanePool.minCount,
            maxNodeCount: config.gke.controlPlanePool.maxCount,
        },
        management: {
            autoRepair: true,
            autoUpgrade: true,
        },
        nodeConfig: {
            machineType: config.gke.controlPlanePool.machineType,
            diskSizeGb: config.gke.controlPlanePool.diskSizeGb,
            diskType: "pd-ssd",
            oauthScopes: ["https://www.googleapis.com/auth/cloud-platform"],
            serviceAccount: iam.nodeServiceAccount.email,
            preemptible: config.gke.controlPlanePool.preemptible,
            labels: { "strand-role": "control-plane", ...labels },
            tags: ["gke-node"],
            metadata: {
                "disable-legacy-endpoints": "true",
            },
        },
    });

    // Node Pool 3: gpu-inference -- NVIDIA L4 nodes
    const gpuInferencePool = new gcp.container.NodePool(resourceName("gpu-pool"), {
        name: "gpu-inference",
        project: config.gcpProject,
        location,
        cluster: cluster.name,
        initialNodeCount: isProd ? 1 : 0,
        autoscaling: {
            minNodeCount: config.gke.gpuInferencePool.minCount,
            maxNodeCount: config.gke.gpuInferencePool.maxCount,
        },
        management: {
            autoRepair: true,
            autoUpgrade: true,
        },
        nodeConfig: {
            machineType: config.gke.gpuInferencePool.machineType,
            diskSizeGb: config.gke.gpuInferencePool.diskSizeGb,
            diskType: "pd-ssd",
            oauthScopes: ["https://www.googleapis.com/auth/cloud-platform"],
            serviceAccount: iam.nodeServiceAccount.email,
            preemptible: config.gke.gpuInferencePool.preemptible,
            labels: {
                "strand-role": "gpu-inference",
                "gpu-type": config.gke.gpuInferencePool.gpuType,
                ...labels,
            },
            guestAccelerators: [{
                type: config.gke.gpuInferencePool.gpuType,
                count: config.gke.gpuInferencePool.gpuCount,
                gpuDriverInstallationConfig: {
                    gpuDriverVersion: config.gke.gpuInferencePool.gpuDriverVersion,
                },
            }],
            taints: [{
                key: "nvidia.com/gpu",
                value: "present",
                effect: "NO_SCHEDULE",
            }],
            tags: ["gke-node", "gpu-node"],
            metadata: {
                "disable-legacy-endpoints": "true",
            },
        },
    });

    // Construct kubeconfig from cluster outputs
    const kubeconfig = pulumi.all([
        cluster.name,
        cluster.endpoint,
        cluster.masterAuth,
    ]).apply(([name, endpoint, auth]) => {
        const context = `${config.gcpProject}_${location}_${name}`;
        return `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${auth.clusterCaCertificate}
    server: https://${endpoint}
  name: ${context}
contexts:
- context:
    cluster: ${context}
    user: ${context}
  name: ${context}
current-context: ${context}
kind: Config
users:
- name: ${context}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin
      installHint: Install gke-gcloud-auth-plugin for use with kubectl
      provideClusterInfo: true`;
    });

    return { cluster, systemPool, controlPlanePool, gpuInferencePool, kubeconfig };
}
