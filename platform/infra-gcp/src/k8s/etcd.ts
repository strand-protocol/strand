import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";

export function deployEtcd(config: GcpPlatformConfig, provider: k8s.Provider) {
    const replicas = config.environment === "dev" ? 1 : 3;
    const ns = "strand-system";

    // High-performance SSD StorageClass for etcd
    const storageClass = new k8s.storage.v1.StorageClass("etcd-ssd", {
        metadata: { name: "etcd-ssd" },
        provisioner: "pd.csi.storage.gke.io",
        parameters: { type: config.storage.etcdDiskType },
        reclaimPolicy: "Retain",
        volumeBindingMode: "WaitForFirstConsumer",
        allowVolumeExpansion: true,
    }, { provider });

    // Headless service for peer discovery
    const headlessSvc = new k8s.core.v1.Service("etcd-headless", {
        metadata: {
            name: "etcd",
            namespace: ns,
            labels: { app: "etcd" },
        },
        spec: {
            clusterIP: "None",
            ports: [
                { name: "client", port: 2379, targetPort: 2379 },
                { name: "peer", port: 2380, targetPort: 2380 },
            ],
            selector: { app: "etcd" },
        },
    }, { provider });

    // Build initial cluster string from StatefulSet pod names
    const initialCluster = Array.from({ length: replicas }, (_, i) =>
        `etcd-${i}=http://etcd-${i}.etcd.${ns}.svc.cluster.local:2380`,
    ).join(",");

    const endpoints = Array.from({ length: replicas }, (_, i) =>
        `http://etcd-${i}.etcd.${ns}.svc.cluster.local:2379`,
    ).join(",");

    // StatefulSet
    const sts = new k8s.apps.v1.StatefulSet("etcd", {
        metadata: { name: "etcd", namespace: ns },
        spec: {
            serviceName: "etcd",
            replicas,
            selector: { matchLabels: { app: "etcd" } },
            template: {
                metadata: {
                    labels: {
                        app: "etcd",
                        "strand-component": "etcd",
                        "strand-billing": "system",
                    },
                },
                spec: {
                    nodeSelector: { "strand-role": "control-plane" },
                    containers: [{
                        name: "etcd",
                        image: "quay.io/coreos/etcd:v3.5.12",
                        ports: [
                            { containerPort: 2379, name: "client" },
                            { containerPort: 2380, name: "peer" },
                        ],
                        env: [
                            {
                                name: "ETCD_NAME",
                                valueFrom: { fieldRef: { fieldPath: "metadata.name" } },
                            },
                            { name: "ETCD_INITIAL_CLUSTER", value: initialCluster },
                            { name: "ETCD_INITIAL_CLUSTER_STATE", value: "new" },
                            { name: "ETCD_INITIAL_CLUSTER_TOKEN", value: `strand-etcd-${config.environment}` },
                            { name: "ETCD_DATA_DIR", value: "/var/run/etcd/default.etcd" },
                            {
                                name: "ETCD_ADVERTISE_CLIENT_URLS",
                                value: `http://$(ETCD_NAME).etcd.${ns}.svc.cluster.local:2379`,
                            },
                            {
                                name: "ETCD_LISTEN_CLIENT_URLS",
                                value: "http://0.0.0.0:2379",
                            },
                            {
                                name: "ETCD_INITIAL_ADVERTISE_PEER_URLS",
                                value: `http://$(ETCD_NAME).etcd.${ns}.svc.cluster.local:2380`,
                            },
                            {
                                name: "ETCD_LISTEN_PEER_URLS",
                                value: "http://0.0.0.0:2380",
                            },
                        ],
                        volumeMounts: [{
                            name: "etcd-data",
                            mountPath: "/var/run/etcd",
                        }],
                        resources: {
                            requests: { cpu: "200m", memory: "256Mi" },
                            limits: { cpu: "1000m", memory: "1Gi" },
                        },
                        livenessProbe: {
                            exec: {
                                command: ["etcdctl", "endpoint", "health", "--endpoints=http://localhost:2379"],
                            },
                            initialDelaySeconds: 15,
                            periodSeconds: 10,
                            timeoutSeconds: 5,
                        },
                        readinessProbe: {
                            exec: {
                                command: ["etcdctl", "endpoint", "health", "--endpoints=http://localhost:2379"],
                            },
                            initialDelaySeconds: 5,
                            periodSeconds: 5,
                        },
                    }],
                },
            },
            volumeClaimTemplates: [{
                metadata: { name: "etcd-data" },
                spec: {
                    accessModes: ["ReadWriteOnce"],
                    storageClassName: "etcd-ssd",
                    resources: {
                        requests: { storage: `${config.storage.etcdDiskSizeGb}Gi` },
                    },
                },
            }],
        },
    }, { provider, dependsOn: [storageClass, headlessSvc] });

    return { storageClass, headlessSvc, statefulSet: sts, endpoints };
}
