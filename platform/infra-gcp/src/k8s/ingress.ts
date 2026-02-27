import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import * as k8s from "@pulumi/kubernetes";
import { GcpPlatformConfig, resourceName } from "../config";
import { DnsResult } from "../dns";

export function setupIngress(
    config: GcpPlatformConfig,
    provider: k8s.Provider,
    dns: DnsResult,
) {
    const ns = "strand-system";

    // Managed SSL certificate for HTTPS
    const managedCert = new gcp.compute.ManagedSslCertificate(resourceName("ssl-cert"), {
        name: resourceName("ssl-cert"),
        project: config.gcpProject,
        managed: {
            domains: [
                `api.${config.domain}`,
                `inference.${config.domain}`,
            ],
        },
    });

    // BackendConfig for strand-cloud health checks
    new k8s.apiextensions.CustomResource("strand-cloud-backend-config", {
        apiVersion: "cloud.google.com/v1",
        kind: "BackendConfig",
        metadata: { name: "strand-cloud-backend", namespace: ns },
        spec: {
            healthCheck: {
                checkIntervalSec: 15,
                port: 8080,
                type: "HTTP",
                requestPath: "/healthz",
            },
            timeoutSec: 30,
        },
    }, { provider });

    // BackendConfig for httpbridge health checks
    new k8s.apiextensions.CustomResource("httpbridge-backend-config", {
        apiVersion: "cloud.google.com/v1",
        kind: "BackendConfig",
        metadata: { name: "httpbridge-backend", namespace: ns },
        spec: {
            healthCheck: {
                checkIntervalSec: 15,
                port: 9001,
                type: "HTTP",
                requestPath: "/healthz",
            },
            timeoutSec: 60,
        },
    }, { provider });

    // GKE Ingress
    const ingress = new k8s.networking.v1.Ingress("strand-ingress", {
        metadata: {
            name: "strand-ingress",
            namespace: ns,
            annotations: {
                "kubernetes.io/ingress.class": "gce",
                "kubernetes.io/ingress.global-static-ip-name": dns.ingressIp.name,
                "networking.gke.io/managed-certificates": managedCert.name,
                "kubernetes.io/ingress.allow-http": "false",
            },
        },
        spec: {
            rules: [
                {
                    host: `api.${config.domain}`,
                    http: {
                        paths: [{
                            path: "/*",
                            pathType: "ImplementationSpecific",
                            backend: {
                                service: { name: "strand-cloud", port: { number: 8080 } },
                            },
                        }],
                    },
                },
                {
                    host: `inference.${config.domain}`,
                    http: {
                        paths: [{
                            path: "/*",
                            pathType: "ImplementationSpecific",
                            backend: {
                                service: { name: "strandapi-httpbridge", port: { number: 9001 } },
                            },
                        }],
                    },
                },
            ],
        },
    }, { provider });

    return { managedCert, ingress };
}
