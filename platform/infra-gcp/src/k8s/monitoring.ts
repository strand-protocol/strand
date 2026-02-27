import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";

export function setupK8sMonitoring(config: GcpPlatformConfig, provider: k8s.Provider) {
    const ns = "strand-system";

    // PodMonitoring CRDs for GKE Managed Prometheus

    // strand-cloud metrics
    const cloudMonitor = new k8s.apiextensions.CustomResource("strand-cloud-monitor", {
        apiVersion: "monitoring.googleapis.com/v1",
        kind: "PodMonitoring",
        metadata: { name: "strand-cloud", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "strand-cloud" } },
            endpoints: [{
                port: "http",
                path: "/metrics",
                interval: "15s",
            }],
        },
    }, { provider });

    // httpbridge metrics
    const bridgeMonitor = new k8s.apiextensions.CustomResource("httpbridge-monitor", {
        apiVersion: "monitoring.googleapis.com/v1",
        kind: "PodMonitoring",
        metadata: { name: "strandapi-httpbridge", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "strandapi-httpbridge" } },
            endpoints: [{
                port: "http",
                path: "/metrics",
                interval: "15s",
            }],
        },
    }, { provider });

    // Inference metrics
    const inferenceMonitor = new k8s.apiextensions.CustomResource("inference-monitor", {
        apiVersion: "monitoring.googleapis.com/v1",
        kind: "PodMonitoring",
        metadata: { name: "strandapi-inference", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "strandapi-inference" } },
            endpoints: [{
                port: "http",
                path: config.nim.enabled ? "/metrics" : "/metrics",
                interval: "15s",
            }],
        },
    }, { provider });

    // etcd metrics
    const etcdMonitor = new k8s.apiextensions.CustomResource("etcd-monitor", {
        apiVersion: "monitoring.googleapis.com/v1",
        kind: "PodMonitoring",
        metadata: { name: "etcd", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "etcd" } },
            endpoints: [{
                port: "client",
                path: "/metrics",
                interval: "30s",
            }],
        },
    }, { provider });

    // NVIDIA DCGM Exporter DaemonSet for GPU metrics
    const dcgmExporter = new k8s.apps.v1.DaemonSet("dcgm-exporter", {
        metadata: { name: "dcgm-exporter", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "dcgm-exporter" } },
            template: {
                metadata: {
                    labels: {
                        app: "dcgm-exporter",
                        "strand-component": "monitoring",
                    },
                },
                spec: {
                    nodeSelector: { "strand-role": "gpu-inference" },
                    tolerations: [{
                        key: "nvidia.com/gpu",
                        value: "present",
                        effect: "NoSchedule",
                    }],
                    containers: [{
                        name: "dcgm-exporter",
                        image: "nvcr.io/nvidia/k8s/dcgm-exporter:3.3.5-3.4.1-ubuntu22.04",
                        ports: [{ containerPort: 9400, name: "metrics" }],
                        env: [
                            { name: "DCGM_EXPORTER_LISTEN", value: ":9400" },
                            { name: "DCGM_EXPORTER_KUBERNETES", value: "true" },
                        ],
                        resources: {
                            requests: { cpu: "100m", memory: "128Mi" },
                            limits: { cpu: "200m", memory: "256Mi" },
                        },
                        securityContext: {
                            privileged: true,
                        },
                        volumeMounts: [{
                            name: "dcgm-socket",
                            mountPath: "/var/lib/kubelet/pod-resources",
                        }],
                    }],
                    volumes: [{
                        name: "dcgm-socket",
                        hostPath: { path: "/var/lib/kubelet/pod-resources" },
                    }],
                },
            },
        },
    }, { provider });

    // PodMonitoring for DCGM GPU metrics
    const dcgmMonitor = new k8s.apiextensions.CustomResource("dcgm-monitor", {
        apiVersion: "monitoring.googleapis.com/v1",
        kind: "PodMonitoring",
        metadata: { name: "dcgm-exporter", namespace: ns },
        spec: {
            selector: { matchLabels: { app: "dcgm-exporter" } },
            endpoints: [{
                port: "metrics",
                path: "/metrics",
                interval: "15s",
            }],
        },
    }, { provider });

    return {
        cloudMonitor,
        bridgeMonitor,
        inferenceMonitor,
        etcdMonitor,
        dcgmExporter,
        dcgmMonitor,
    };
}
