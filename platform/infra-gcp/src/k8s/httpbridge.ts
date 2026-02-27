import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";
import { RegistryResult } from "../registry";

export function deployHttpBridge(
    config: GcpPlatformConfig,
    provider: k8s.Provider,
    registry: RegistryResult,
    inferenceServicePort: number,
) {
    const ns = "strand-system";
    const isProd = config.environment === "prod";
    const replicas = isProd ? 2 : 1;

    // httpbridge translates OpenAI-compatible REST to either:
    // - NIM's /v1/chat/completions (if NIM is the inference backend)
    // - StrandAPI protocol on port 9000 (if using custom inference)
    const backendUrl = config.nim.enabled
        ? `http://strandapi-inference.${ns}.svc.cluster.local:${inferenceServicePort}`
        : `http://strandapi-inference.${ns}.svc.cluster.local:9000`;

    const deployment = new k8s.apps.v1.Deployment("httpbridge", {
        metadata: { name: "strandapi-httpbridge", namespace: ns },
        spec: {
            replicas,
            selector: { matchLabels: { app: "strandapi-httpbridge" } },
            template: {
                metadata: {
                    labels: {
                        app: "strandapi-httpbridge",
                        "strand-component": "httpbridge",
                        "strand-billing": "system",
                    },
                },
                spec: {
                    nodeSelector: { "strand-role": "control-plane" },
                    containers: [{
                        name: "httpbridge",
                        image: pulumi.interpolate`${registry.repositoryUrl}/strandapi-httpbridge:latest`,
                        command: ["/strandapi-httpbridge"],
                        args: ["0.0.0.0:9001"],
                        ports: [{ containerPort: 9001, name: "http" }],
                        env: [
                            { name: "STRANDAPI_BACKEND_URL", value: backendUrl },
                            { name: "STRANDAPI_CORS_ORIGINS", value: `https://api.${config.domain},https://inference.${config.domain}` },
                        ],
                        resources: {
                            requests: { cpu: "100m", memory: "128Mi" },
                            limits: { cpu: "500m", memory: "512Mi" },
                        },
                        livenessProbe: {
                            httpGet: { path: "/healthz", port: 9001 },
                            initialDelaySeconds: 5,
                            periodSeconds: 10,
                        },
                        readinessProbe: {
                            httpGet: { path: "/healthz", port: 9001 },
                            initialDelaySeconds: 3,
                            periodSeconds: 5,
                        },
                    }],
                },
            },
        },
    }, { provider });

    // ClusterIP Service
    const service = new k8s.core.v1.Service("httpbridge-svc", {
        metadata: {
            name: "strandapi-httpbridge",
            namespace: ns,
            annotations: {
                "cloud.google.com/backend-config":
                    '{"default": "httpbridge-backend"}',
            },
        },
        spec: {
            selector: { app: "strandapi-httpbridge" },
            ports: [{ name: "http", port: 9001, targetPort: 9001 }],
            type: "ClusterIP",
        },
    }, { provider });

    return { deployment, service };
}
