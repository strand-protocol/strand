import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";
import { IamResult } from "../iam";
import { RegistryResult } from "../registry";

export function deployInference(
    config: GcpPlatformConfig,
    provider: k8s.Provider,
    registry: RegistryResult,
    iam: IamResult,
) {
    const ns = "strand-system";
    const isProd = config.environment === "prod";

    // K8s ServiceAccount with Workload Identity
    const sa = new k8s.core.v1.ServiceAccount("inference-sa", {
        metadata: {
            name: "strandapi-inference",
            namespace: ns,
            annotations: {
                "iam.gke.io/gcp-service-account": iam.inferenceSa.email,
            },
        },
    }, { provider });

    // NVIDIA NIM deployment (GPU workload)
    // NIM provides TensorRT-LLM optimized serving for LLMs
    // Falls back to custom strandapi-inference image if NIM is disabled
    const useNim = config.nim.enabled;

    const container: k8s.types.input.core.v1.Container = useNim ? {
        name: "nim-inference",
        image: config.nim.image,
        ports: [{ containerPort: 8000, name: "http" }],
        env: [
            { name: "NIM_MODEL_NAME", value: config.nim.modelName },
            { name: "NIM_MAX_MODEL_LEN", value: "4096" },
            { name: "NIM_GPU_MEMORY_UTILIZATION", value: "0.9" },
        ],
        resources: {
            requests: {
                cpu: "2000m",
                memory: "8Gi",
                "nvidia.com/gpu": `${config.nim.gpuCount}`,
            },
            limits: {
                cpu: "8000m",
                memory: "24Gi",
                "nvidia.com/gpu": `${config.nim.gpuCount}`,
            },
        },
        livenessProbe: {
            httpGet: { path: "/v1/health/live", port: 8000 },
            initialDelaySeconds: 120,
            periodSeconds: 15,
            timeoutSeconds: 5,
        },
        readinessProbe: {
            httpGet: { path: "/v1/health/ready", port: 8000 },
            initialDelaySeconds: 60,
            periodSeconds: 10,
        },
        volumeMounts: [{
            name: "model-cache",
            mountPath: "/opt/nim/.cache",
        }],
    } : {
        name: "strandapi-inference",
        image: pulumi.interpolate`${registry.repositoryUrl}/strandapi-inference:latest`,
        ports: [{ containerPort: 9000, name: "http" }],
        env: [
            { name: "STRANDAPI_ADDR", value: "0.0.0.0:9000" },
        ],
        resources: {
            requests: {
                cpu: "2000m",
                memory: "8Gi",
                "nvidia.com/gpu": "1",
            },
            limits: {
                cpu: "4000m",
                memory: "16Gi",
                "nvidia.com/gpu": "1",
            },
        },
        livenessProbe: {
            httpGet: { path: "/healthz", port: 9000 },
            initialDelaySeconds: 30,
            periodSeconds: 10,
        },
        readinessProbe: {
            httpGet: { path: "/healthz", port: 9000 },
            initialDelaySeconds: 15,
            periodSeconds: 5,
        },
    };

    const servicePort = useNim ? 8000 : 9000;

    const deployment = new k8s.apps.v1.Deployment("inference", {
        metadata: { name: "strandapi-inference", namespace: ns },
        spec: {
            replicas: config.nim.replicas,
            selector: { matchLabels: { app: "strandapi-inference" } },
            template: {
                metadata: {
                    labels: {
                        app: "strandapi-inference",
                        "strand-component": "inference",
                        "strand-gpu": "true",
                        "strand-billing": "inference",
                    },
                },
                spec: {
                    serviceAccountName: "strandapi-inference",
                    nodeSelector: { "strand-role": "gpu-inference" },
                    tolerations: [{
                        key: "nvidia.com/gpu",
                        value: "present",
                        effect: "NoSchedule",
                    }],
                    containers: [container],
                    volumes: [{
                        name: "model-cache",
                        emptyDir: { sizeLimit: "50Gi" },
                    }],
                },
            },
        },
    }, { provider });

    // ClusterIP Service
    const service = new k8s.core.v1.Service("inference-svc", {
        metadata: { name: "strandapi-inference", namespace: ns },
        spec: {
            selector: { app: "strandapi-inference" },
            ports: [{ name: "http", port: servicePort, targetPort: servicePort }],
            type: "ClusterIP",
        },
    }, { provider });

    // HorizontalPodAutoscaler
    const hpa = new k8s.autoscaling.v2.HorizontalPodAutoscaler("inference-hpa", {
        metadata: { name: "strandapi-inference", namespace: ns },
        spec: {
            scaleTargetRef: {
                apiVersion: "apps/v1",
                kind: "Deployment",
                name: "strandapi-inference",
            },
            minReplicas: config.nim.replicas,
            maxReplicas: config.nim.maxReplicas,
            metrics: [{
                type: "Resource",
                resource: {
                    name: "cpu",
                    target: { type: "Utilization", averageUtilization: 70 },
                },
            }],
            behavior: {
                scaleDown: {
                    stabilizationWindowSeconds: 300,
                    policies: [{ type: "Percent", value: 50, periodSeconds: 60 }],
                },
                scaleUp: {
                    stabilizationWindowSeconds: 30,
                    policies: [{ type: "Percent", value: 100, periodSeconds: 60 }],
                },
            },
        },
    }, { provider });

    return { sa, deployment, service, hpa, servicePort };
}
