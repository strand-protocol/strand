import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import * as fs from "fs";
import * as path from "path";
import {
    PlatformConfig,
    BMC_API_BASE,
    resourceName,
    standardTags,
} from "./config";
import { NetworkResult } from "./network";

// ---------------------------------------------------------------------------
// Bare-metal server provisioning via PhoenixNAP BMC REST API
// ---------------------------------------------------------------------------

export interface ServerInfo {
    id: pulumi.Output<string>;
    publicIp: pulumi.Output<string>;
    privateIp: pulumi.Output<string>;
    hostname: string;
    role: "control-plane" | "worker" | "database";
}

export interface ServersResult {
    controlPlane: ServerInfo[];
    workers: ServerInfo[];
    databases: ServerInfo[];
    all: ServerInfo[];
}

/**
 * Read a cloud-init template from the cloud-init/ directory and perform
 * basic variable substitution.
 */
function loadCloudInit(
    templateName: string,
    vars: Record<string, string>,
): string {
    const tplPath = path.join(__dirname, "..", "cloud-init", templateName);
    let content = fs.readFileSync(tplPath, "utf-8");
    for (const [key, value] of Object.entries(vars)) {
        content = content.replace(new RegExp(`\\$\\{${key}\\}`, "g"), value);
    }
    return content;
}

/**
 * Provision a single bare-metal server via the BMC API.
 *
 * We use a Pulumi Command resource that shells out to curl.  The create
 * command POSTs to /servers, the delete command DELETEs by server id.
 *
 * The server response JSON is parsed with jq to extract the fields we need.
 */
function provisionServer(
    config: PlatformConfig,
    net: NetworkResult,
    opts: {
        hostname: string;
        serverType: string;
        role: "control-plane" | "worker" | "database";
        cloudInit: string;
        index: number;
    },
    parent?: pulumi.Resource,
): { cmd: command.local.Command; info: ServerInfo } {
    const name = resourceName(opts.role, opts.index);
    const tags = standardTags({ "strand:role": opts.role });
    const tagsList = Object.entries(tags)
        .map(([k, v]) => `{"name":"${k}","value":"${v}"}`)
        .join(",");

    // Base64-encode the cloud-init payload so we can safely embed it in JSON.
    const cloudInitB64 = Buffer.from(opts.cloudInit).toString("base64");

    const body = pulumi.interpolate`{
        "hostname": "${opts.hostname}",
        "os": "ubuntu/jammy",
        "type": "${opts.serverType}",
        "location": "${config.region}",
        "installDefaultSshKeys": false,
        "sshKeys": ["${config.sshKeyId}"],
        "networkType": "PRIVATE_AND_PUBLIC",
        "networkConfiguration": {
            "privateNetworkConfiguration": {
                "configurationType": "USER_DEFINED",
                "privateNetworks": [{
                    "id": "${net.privateNetworkId}"
                }]
            }
        },
        "tags": [${tagsList}],
        "cloudInit": {
            "userData": "${cloudInitB64}"
        },
        "description": "Strand ${opts.role} node (${config.environment})"
    }`;

    const cmd = new command.local.Command(name, {
        create: pulumi.interpolate`curl -sf -X POST ${BMC_API_BASE}/servers \
            -H "Authorization: Bearer ${net.authToken}" \
            -H "Content-Type: application/json" \
            -d '${body}'`,
        delete: pulumi.interpolate`SERVER_ID=$(echo '${cmd ? "" : ""}' | jq -r '.id' 2>/dev/null || true) && \
            [ -n "$SERVER_ID" ] && \
            curl -sf -X DELETE ${BMC_API_BASE}/servers/$SERVER_ID \
            -H "Authorization: Bearer ${net.authToken}" || true`,
    }, { parent });

    const info: ServerInfo = {
        id: cmd.stdout.apply(s => {
            try { return JSON.parse(s).id; } catch { return ""; }
        }),
        publicIp: cmd.stdout.apply(s => {
            try {
                const parsed = JSON.parse(s);
                const pub = parsed.publicIpAddresses ?? [];
                return pub.length > 0 ? pub[0] : "";
            } catch { return ""; }
        }),
        privateIp: cmd.stdout.apply(s => {
            try {
                const parsed = JSON.parse(s);
                const priv = parsed.privateIpAddresses ?? [];
                return priv.length > 0 ? priv[0] : "";
            } catch { return ""; }
        }),
        hostname: opts.hostname,
        role: opts.role,
    };

    return { cmd, info };
}

/**
 * Provision all servers for the platform based on the environment config.
 */
export function provisionServers(
    config: PlatformConfig,
    net: NetworkResult,
): ServersResult {
    const controlPlane: ServerInfo[] = [];
    const workers: ServerInfo[] = [];
    const databases: ServerInfo[] = [];

    // --- Control Plane nodes ---
    for (let i = 0; i < config.controlPlane.count; i++) {
        const hostname = `strand-cp-${config.environment}-${i}`;
        const cloudInit = loadCloudInit("control-plane.yaml", {
            HOSTNAME: hostname,
            ENVIRONMENT: config.environment,
            NODE_INDEX: i.toString(),
            ROLE: "control-plane",
        });
        const { info } = provisionServer(config, net, {
            hostname,
            serverType: config.controlPlane.type,
            role: "control-plane",
            cloudInit,
            index: i,
        });
        controlPlane.push(info);
    }

    // --- Worker nodes ---
    for (let i = 0; i < config.workers.count; i++) {
        const hostname = `strand-wk-${config.environment}-${i}`;
        const cloudInit = loadCloudInit("worker.yaml", {
            HOSTNAME: hostname,
            ENVIRONMENT: config.environment,
            NODE_INDEX: i.toString(),
            ROLE: "worker",
        });
        const { info } = provisionServer(config, net, {
            hostname,
            serverType: config.workers.type,
            role: "worker",
            cloudInit,
            index: i,
        });
        workers.push(info);
    }

    // --- Database nodes (only if not co-located on control plane) ---
    if (!config.database.colocated && config.database.count > 0) {
        for (let i = 0; i < config.database.count; i++) {
            const hostname = `strand-db-${config.environment}-${i}`;
            const cloudInit = loadCloudInit("database.yaml", {
                HOSTNAME: hostname,
                ENVIRONMENT: config.environment,
                NODE_INDEX: i.toString(),
                ROLE: "database",
            });
            const { info } = provisionServer(config, net, {
                hostname,
                serverType: config.database.type,
                role: "database",
                cloudInit,
                index: i,
            });
            databases.push(info);
        }
    }

    const all = [...controlPlane, ...workers, ...databases];

    return { controlPlane, workers, databases, all };
}
