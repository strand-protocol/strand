import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";
import { IamResult } from "../iam";
import { RegistryResult } from "../registry";

export function deployStrandCloud(
    config: GcpPlatformConfig,
    provider: k8s.Provider,
    registry: RegistryResult,
    iam: IamResult,
    etcdEndpoints: string,
) {
    const ns = "strand-system";
    const isProd = config.environment === "prod";
    const replicas = isProd ? 3 : 1;

    // K8s ServiceAccount with Workload Identity
    const sa = new k8s.core.v1.ServiceAccount("strand-cloud-sa", {
        metadata: {
            name: "strand-cloud",
            namespace: ns,
            annotations: {
                "iam.gke.io/gcp-service-account": iam.strandCloudSa.email,
            },
        },
    }, { provider });

    // Deployment
    const deployment = new k8s.apps.v1.Deployment("strand-cloud", {
        metadata: { name: "strand-cloud", namespace: ns },
        spec: {
            replicas,
            selector: { matchLabels: { app: "strand-cloud" } },
            template: {
                metadata: {
                    labels: {
                        app: "strand-cloud",
                        "strand-component": "control-plane",
                        "strand-billing": "system",
                    },
                },
                spec: {
                    serviceAccountName: "strand-cloud",
                    nodeSelector: { "strand-role": "control-plane" },
                    ...(isProd ? {
                        topologySpreadConstraints: [{
                            maxSkew: 1,
                            topologyKey: "topology.kubernetes.io/zone",
                            whenUnsatisfiable: "DoNotSchedule",
                            labelSelector: { matchLabels: { app: "strand-cloud" } },
                        }],
                    } : {}),
                    containers: [{
                        name: "strand-cloud",
                        image: pulumi.interpolate`${registry.repositoryUrl}/strand-cloud:latest`,
                        ports: [{ containerPort: 8080, name: "http" }],
                        env: [
                            { name: "STRAND_ADDR", value: "0.0.0.0:8080" },
                            { name: "STRAND_STORE_TYPE", value: "etcd" },
                            { name: "STRAND_ETCD_ENDPOINTS", value: etcdEndpoints },
                        ],
                        resources: {
                            requests: { cpu: "250m", memory: "256Mi" },
                            limits: { cpu: "1000m", memory: "1Gi" },
                        },
                        livenessProbe: {
                            httpGet: { path: "/healthz", port: 8080 },
                            initialDelaySeconds: 10,
                            periodSeconds: 10,
                        },
                        readinessProbe: {
                            httpGet: { path: "/readyz", port: 8080 },
                            initialDelaySeconds: 5,
                            periodSeconds: 5,
                        },
                    }],
                },
            },
        },
    }, { provider });

    // ClusterIP Service
    const service = new k8s.core.v1.Service("strand-cloud-svc", {
        metadata: {
            name: "strand-cloud",
            namespace: ns,
            annotations: {
                "cloud.google.com/backend-config":
                    '{"default": "strand-cloud-backend"}',
            },
        },
        spec: {
            selector: { app: "strand-cloud" },
            ports: [{ name: "http", port: 8080, targetPort: 8080 }],
            type: "ClusterIP",
        },
    }, { provider });

    // PodDisruptionBudget (prod only)
    let pdb: k8s.policy.v1.PodDisruptionBudget | undefined;
    if (isProd) {
        pdb = new k8s.policy.v1.PodDisruptionBudget("strand-cloud-pdb", {
            metadata: { name: "strand-cloud", namespace: ns },
            spec: {
                minAvailable: 2,
                selector: { matchLabels: { app: "strand-cloud" } },
            },
        }, { provider });
    }

    return { sa, deployment, service, pdb };
}
