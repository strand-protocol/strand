import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import { PlatformConfig, resourceName } from "./config";
import { ServerInfo } from "./servers";

// ---------------------------------------------------------------------------
// DNS record management
//
// Uses the PhoenixNAP BMC tagging + external DNS provider (Cloudflare-style
// API calls via curl).  Adjust the API calls below if you use a different
// DNS provider.
//
// Required environment variables for DNS operations:
//   CLOUDFLARE_API_TOKEN  -- bearer token with DNS edit permission
//   CLOUDFLARE_ZONE_ID    -- zone identifier for the domain
// ---------------------------------------------------------------------------

const CF_API = "https://api.cloudflare.com/client/v4";

export interface DnsResult {
    records: { name: string; ip: pulumi.Output<string> }[];
}

/**
 * Create an A record pointing a subdomain to a server's public IP.
 */
function createARecord(
    name: string,
    subdomain: string,
    ip: pulumi.Output<string>,
    config: PlatformConfig,
    parent?: pulumi.Resource,
): command.local.Command {
    const fqdn = `${subdomain}.${config.domain}`;

    return new command.local.Command(resourceName(`dns-${name}`), {
        create: pulumi.interpolate`curl -sf -X POST \
            "${CF_API}/zones/$CLOUDFLARE_ZONE_ID/dns_records" \
            -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
            -H "Content-Type: application/json" \
            -d '{
                "type": "A",
                "name": "${fqdn}",
                "content": "${ip}",
                "ttl": 300,
                "proxied": false,
                "comment": "Strand ${config.environment} - ${name}"
            }' | jq -r '.result.id'`,
        delete: pulumi.interpolate`RECORD_ID=$(curl -sf \
            "${CF_API}/zones/$CLOUDFLARE_ZONE_ID/dns_records?name=${fqdn}&type=A" \
            -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
            | jq -r '.result[0].id // empty') && \
            [ -n "$RECORD_ID" ] && \
            curl -sf -X DELETE \
            "${CF_API}/zones/$CLOUDFLARE_ZONE_ID/dns_records/$RECORD_ID" \
            -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" || true`,
    }, { parent });
}

/**
 * Set up DNS records for the entire platform.
 *
 * Records created:
 *   api.<domain>       -> first control-plane node (or load-balanced VIP in prod)
 *   grafana.<domain>   -> monitoring node (first CP)
 *   cp-N.<domain>      -> each control-plane node
 *   wk-N.<domain>      -> each worker node
 *   db-N.<domain>      -> each database node
 */
export function setupDns(
    config: PlatformConfig,
    cpServers: ServerInfo[],
    workerServers: ServerInfo[],
    dbServers: ServerInfo[],
): DnsResult {
    const records: { name: string; ip: pulumi.Output<string> }[] = [];

    // API endpoint points to first control-plane node
    if (cpServers.length > 0) {
        createARecord("api", "api", cpServers[0].publicIp, config);
        records.push({ name: `api.${config.domain}`, ip: cpServers[0].publicIp });
    }

    // Grafana dashboard
    if (config.monitoring.enabled && cpServers.length > 0) {
        createARecord("grafana", "grafana", cpServers[0].publicIp, config);
        records.push({ name: `grafana.${config.domain}`, ip: cpServers[0].publicIp });
    }

    // Individual control-plane nodes
    for (let i = 0; i < cpServers.length; i++) {
        const sub = `cp-${i}`;
        createARecord(`cp-${i}`, sub, cpServers[i].publicIp, config);
        records.push({ name: `${sub}.${config.domain}`, ip: cpServers[i].publicIp });
    }

    // Individual worker nodes
    for (let i = 0; i < workerServers.length; i++) {
        const sub = `wk-${i}`;
        createARecord(`wk-${i}`, sub, workerServers[i].publicIp, config);
        records.push({ name: `${sub}.${config.domain}`, ip: workerServers[i].publicIp });
    }

    // Individual database nodes
    for (let i = 0; i < dbServers.length; i++) {
        const sub = `db-${i}`;
        createARecord(`db-${i}`, sub, dbServers[i].publicIp, config);
        records.push({ name: `${sub}.${config.domain}`, ip: dbServers[i].publicIp });
    }

    return { records };
}
