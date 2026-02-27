import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface NetworkResult {
    vpc: gcp.compute.Network;
    subnet: gcp.compute.Subnetwork;
    router: gcp.compute.Router;
    nat: gcp.compute.RouterNat;
    podRangeName: string;
    serviceRangeName: string;
}

export function setupNetwork(config: GcpPlatformConfig): NetworkResult {
    const labels = standardLabels();
    const podRangeName = "gke-pods";
    const serviceRangeName = "gke-services";

    // Custom-mode VPC
    const vpc = new gcp.compute.Network(resourceName("vpc"), {
        name: resourceName("vpc"),
        project: config.gcpProject,
        autoCreateSubnetworks: false,
    });

    // Primary subnet with secondary ranges for GKE pods and services
    const subnet = new gcp.compute.Subnetwork(resourceName("gke-subnet"), {
        name: resourceName("gke-subnet"),
        project: config.gcpProject,
        region: config.gcpRegion,
        network: vpc.id,
        ipCidrRange: config.environment === "prod" ? "10.0.0.0/18" : "10.0.0.0/20",
        privateIpGoogleAccess: true,
        secondaryIpRanges: [
            { rangeName: podRangeName, ipCidrRange: config.network.podCidr },
            { rangeName: serviceRangeName, ipCidrRange: config.network.serviceCidr },
        ],
    });

    // Cloud Router for NAT
    const router = new gcp.compute.Router(resourceName("router"), {
        name: resourceName("router"),
        project: config.gcpProject,
        region: config.gcpRegion,
        network: vpc.id,
    });

    // Cloud NAT for private node egress
    const nat = new gcp.compute.RouterNat(resourceName("nat"), {
        name: resourceName("nat"),
        project: config.gcpProject,
        region: config.gcpRegion,
        router: router.name,
        natIpAllocateOption: "AUTO_ONLY",
        sourceSubnetworkIpRangesToNat: "ALL_SUBNETWORKS_ALL_IP_RANGES",
        logConfig: {
            enable: true,
            filter: "ERRORS_ONLY",
        },
    });

    // Allow internal traffic between GKE nodes
    new gcp.compute.Firewall(resourceName("allow-internal"), {
        name: resourceName("allow-internal"),
        project: config.gcpProject,
        network: vpc.id,
        allows: [
            { protocol: "tcp", ports: ["0-65535"] },
            { protocol: "udp", ports: ["0-65535"] },
            { protocol: "icmp" },
        ],
        sourceRanges: [subnet.ipCidrRange],
    });

    // Allow GCP health check probes
    new gcp.compute.Firewall(resourceName("allow-health-checks"), {
        name: resourceName("allow-health-checks"),
        project: config.gcpProject,
        network: vpc.id,
        allows: [
            { protocol: "tcp" },
        ],
        sourceRanges: ["130.211.0.0/22", "35.191.0.0/16"],
        targetTags: ["gke-node"],
    });

    // Allow IAP tunnel for SSH debugging
    new gcp.compute.Firewall(resourceName("allow-iap"), {
        name: resourceName("allow-iap"),
        project: config.gcpProject,
        network: vpc.id,
        allows: [
            { protocol: "tcp", ports: ["22"] },
        ],
        sourceRanges: ["35.235.240.0/20"],
    });

    return { vpc, subnet, router, nat, podRangeName, serviceRangeName };
}
