import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { loadConfig } from "./src/config";
import { setupNetwork } from "./src/network";
import { setupIam } from "./src/iam";
import { createGkeCluster } from "./src/gke";
import { createRegistry } from "./src/registry";
import { provisionStorage } from "./src/storage";
import { setupBilling } from "./src/billing";
import { setupMonitoring } from "./src/monitoring";
import { setupDns } from "./src/dns";
import { createNamespaces } from "./src/k8s/namespaces";
import { deployEtcd } from "./src/k8s/etcd";
import { deployStrandCloud } from "./src/k8s/strand-cloud";
import { deployInference } from "./src/k8s/inference";
import { deployHttpBridge } from "./src/k8s/httpbridge";
import { setupIngress } from "./src/k8s/ingress";
import { setupK8sMonitoring } from "./src/k8s/monitoring";

// ---------------------------------------------------------------------------
// Strand Protocol -- GCP GKE Infrastructure
//
// Orchestrates the full deployment:
//   1. VPC networking (private nodes + Cloud NAT)
//   2. IAM and Workload Identity service accounts
//   3. GKE cluster with 3 node pools (system, control-plane, gpu-inference)
//   4. Artifact Registry for container images
//   5. Storage (GCS for etcd backups + model artifacts)
//   6. Billing (BigQuery datasets + usage metering + chargeback views)
//   7. Kubernetes workloads (etcd, strand-cloud, NIM inference, httpbridge)
//   8. Ingress (global static IP, managed SSL, host-based routing)
//   9. DNS (Cloud DNS zone + records)
//  10. Monitoring (Managed Prometheus, DCGM exporter, alert policies)
// ---------------------------------------------------------------------------

const config = loadConfig();

// --- 1. Network ---
const network = setupNetwork(config);

// --- 2. IAM ---
const iam = setupIam(config);

// --- 3. GKE Cluster ---
const gke = createGkeCluster(config, network, iam);

// --- 4. Artifact Registry ---
const registry = createRegistry(config);

// --- 5. Storage ---
const storage = provisionStorage(config);

// --- 6. Billing ---
const billing = setupBilling(config);

// --- 7. DNS + Static IP ---
const dns = setupDns(config);

// --- 8. K8s Provider ---
const k8sProvider = new k8s.Provider("gke-k8s", {
    kubeconfig: gke.kubeconfig,
});

// --- 9. K8s Workloads ---
const namespaces = createNamespaces(config, k8sProvider);
const etcd = deployEtcd(config, k8sProvider);
const strandCloud = deployStrandCloud(config, k8sProvider, registry, iam, etcd.endpoints);
const inference = deployInference(config, k8sProvider, registry, iam);
const httpbridge = deployHttpBridge(config, k8sProvider, registry, inference.servicePort);

// --- 10. Ingress ---
const ingress = setupIngress(config, k8sProvider, dns);

// --- 11. Monitoring ---
const monitoring = setupMonitoring(config);
const k8sMonitoring = setupK8sMonitoring(config, k8sProvider);

// ---------------------------------------------------------------------------
// Stack outputs
// ---------------------------------------------------------------------------

export const environment = config.environment;
export const gcpProject = config.gcpProject;
export const gcpRegion = config.gcpRegion;
export const domain = config.domain;

export const vpcId = network.vpc.id;
export const subnetId = network.subnet.id;

export const gkeClusterName = gke.cluster.name;
export const gkeClusterEndpoint = gke.cluster.endpoint;
export const kubeconfig = pulumi.secret(gke.kubeconfig);

export const registryUrl = registry.repositoryUrl;

export const ingressIp = dns.ingressIp.address;
export const apiEndpoint = pulumi.interpolate`https://api.${config.domain}`;
export const inferenceEndpoint = pulumi.interpolate`https://inference.${config.domain}`;

export const billingDatasetId = billing.usageDataset.datasetId;
export const billingExportDatasetId = billing.exportDataset.datasetId;

export const etcdBackupBucket = storage.etcdBackupBucket.name;
export const modelArtifactsBucket = storage.modelArtifactsBucket.name;
