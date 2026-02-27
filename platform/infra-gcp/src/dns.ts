import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface DnsResult {
    ingressIp: gcp.compute.GlobalAddress;
    zone?: gcp.dns.ManagedZone;
}

export function setupDns(config: GcpPlatformConfig): DnsResult {
    // Global static IP for GKE Ingress
    const ingressIp = new gcp.compute.GlobalAddress(resourceName("ingress-ip"), {
        name: resourceName("ingress-ip"),
        project: config.gcpProject,
    });

    // Cloud DNS zone (optional -- users may prefer external DNS like Cloudflare)
    // Only create in prod to avoid cluttering dev/staging with DNS zones
    let zone: gcp.dns.ManagedZone | undefined;

    if (config.environment === "prod") {
        zone = new gcp.dns.ManagedZone(resourceName("dns-zone"), {
            name: resourceName("dns-zone"),
            project: config.gcpProject,
            dnsName: `${config.domain}.`,
            description: `Strand Protocol ${config.environment} DNS zone`,
            labels: standardLabels(),
        });

        // A records for API and inference endpoints
        new gcp.dns.RecordSet(resourceName("api-record"), {
            name: `api.${config.domain}.`,
            project: config.gcpProject,
            managedZone: zone.name,
            type: "A",
            ttl: 300,
            rrdatas: [ingressIp.address],
        });

        new gcp.dns.RecordSet(resourceName("inference-record"), {
            name: `inference.${config.domain}.`,
            project: config.gcpProject,
            managedZone: zone.name,
            type: "A",
            ttl: 300,
            rrdatas: [ingressIp.address],
        });
    }

    return { ingressIp, zone };
}
