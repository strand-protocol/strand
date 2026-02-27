import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig } from "../config";

// Plan-based resource quotas
const planQuotas: Record<string, { cpu: string; memory: string; gpu: string; pods: string }> = {
    free:       { cpu: "2",   memory: "4Gi",   gpu: "0",  pods: "10" },
    starter:    { cpu: "8",   memory: "16Gi",  gpu: "1",  pods: "50" },
    pro:        { cpu: "32",  memory: "64Gi",  gpu: "4",  pods: "200" },
    enterprise: { cpu: "128", memory: "256Gi", gpu: "16", pods: "1000" },
};

export function createNamespaces(
    config: GcpPlatformConfig,
    provider: k8s.Provider,
) {
    // System namespace for all Strand Protocol infrastructure
    const systemNs = new k8s.core.v1.Namespace("strand-system", {
        metadata: {
            name: "strand-system",
            labels: {
                "strand-component": "system",
                "pod-security.kubernetes.io/enforce": "restricted",
                "pod-security.kubernetes.io/audit": "restricted",
            },
        },
    }, { provider });

    // Per-tenant namespaces with resource quotas and network isolation
    const tenantNamespaces = config.tenants.map(tenant => {
        const ns = new k8s.core.v1.Namespace(`tenant-${tenant.name}`, {
            metadata: {
                name: `tenant-${tenant.name}`,
                labels: {
                    "strand-tenant": tenant.name,
                    "strand-plan": tenant.plan,
                    "strand-billing": "enabled",
                },
                annotations: {
                    "strandprotocol.com/tenant-id": tenant.id,
                    "strandprotocol.com/plan": tenant.plan,
                },
            },
        }, { provider });

        const quota = planQuotas[tenant.plan] ?? planQuotas.free;

        // Resource quota based on plan
        new k8s.core.v1.ResourceQuota(`quota-${tenant.name}`, {
            metadata: {
                name: "tenant-quota",
                namespace: `tenant-${tenant.name}`,
            },
            spec: {
                hard: {
                    "requests.cpu": quota.cpu,
                    "requests.memory": quota.memory,
                    "requests.nvidia.com/gpu": quota.gpu,
                    pods: quota.pods,
                    persistentvolumeclaims: "10",
                    "services.loadbalancers": "2",
                },
            },
        }, { provider, dependsOn: [ns] });

        // Default resource limits
        new k8s.core.v1.LimitRange(`limits-${tenant.name}`, {
            metadata: {
                name: "default-limits",
                namespace: `tenant-${tenant.name}`,
            },
            spec: {
                limits: [{
                    type: "Container",
                    defaultRequest: { cpu: "100m", memory: "128Mi" },
                    default: { cpu: "500m", memory: "512Mi" },
                }],
            },
        }, { provider, dependsOn: [ns] });

        // Network policy: isolate tenants from each other
        new k8s.networking.v1.NetworkPolicy(`netpol-${tenant.name}`, {
            metadata: {
                name: "tenant-isolation",
                namespace: `tenant-${tenant.name}`,
            },
            spec: {
                podSelector: {},
                policyTypes: ["Ingress", "Egress"],
                ingress: [{
                    from: [
                        { namespaceSelector: { matchLabels: { "strand-tenant": tenant.name } } },
                        { namespaceSelector: { matchLabels: { "strand-component": "system" } } },
                    ],
                }],
                egress: [
                    {
                        to: [
                            { namespaceSelector: { matchLabels: { "strand-tenant": tenant.name } } },
                            { namespaceSelector: { matchLabels: { "strand-component": "system" } } },
                        ],
                    },
                    {
                        // Allow DNS resolution
                        ports: [
                            { port: 53, protocol: "UDP" },
                            { port: 53, protocol: "TCP" },
                        ],
                    },
                ],
            },
        }, { provider, dependsOn: [ns] });

        return ns;
    });

    return { systemNs, tenantNamespaces };
}
