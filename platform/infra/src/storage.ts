import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import {
    PlatformConfig,
    BMC_API_BASE,
    resourceName,
    standardTags,
} from "./config";
import { NetworkResult } from "./network";
import { ServerInfo } from "./servers";

// ---------------------------------------------------------------------------
// Block storage volumes for database nodes
// ---------------------------------------------------------------------------

export interface StorageVolume {
    id: pulumi.Output<string>;
    name: string;
    sizeGb: number;
    attachedTo: string;
}

export interface StorageResult {
    volumes: StorageVolume[];
}

/**
 * Create a block storage volume and attach it to the given server.
 *
 * PhoenixNAP BMC exposes storage volumes through their /storage/volumes API.
 * We create the volume and then attach it to the server in a single chained
 * curl call.
 */
function createAndAttachVolume(
    config: PlatformConfig,
    net: NetworkResult,
    server: ServerInfo,
    index: number,
    parent?: pulumi.Resource,
): StorageVolume {
    const volumeName = resourceName(`db-vol`, index);
    const tags = standardTags({ "strand:role": "database-storage" });
    const tagsList = Object.entries(tags)
        .map(([k, v]) => `{"name":"${k}","value":"${v}"}`)
        .join(",");

    const createBody = pulumi.interpolate`{
        "name": "${volumeName}",
        "capacityInGb": ${config.storage.dbDiskSizeGb},
        "description": "Strand database storage for ${server.hostname}",
        "pathSuffix": "/data/strand",
        "tags": [${tagsList}]
    }`;

    const volumeCmd = new command.local.Command(volumeName, {
        create: pulumi.interpolate`VOLUME_ID=$(curl -sf -X POST "${BMC_API_BASE}/storage/volumes" \
            -H "Authorization: Bearer ${net.authToken}" \
            -H "Content-Type: application/json" \
            -d '${createBody}' | jq -r '.id') && \
            curl -sf -X POST "${BMC_API_BASE}/servers/${server.id}/storage/volumes" \
            -H "Authorization: Bearer ${net.authToken}" \
            -H "Content-Type: application/json" \
            -d "{\"volumeId\": \"$VOLUME_ID\"}" && \
            echo "$VOLUME_ID"`,
        delete: pulumi.interpolate`VOLUME_ID=$(curl -s "${BMC_API_BASE}/storage/volumes" \
            -H "Authorization: Bearer ${net.authToken}" \
            | jq -r '.[] | select(.name=="${volumeName}") | .id') && \
            [ -n "$VOLUME_ID" ] && \
            curl -sf -X DELETE "${BMC_API_BASE}/storage/volumes/$VOLUME_ID" \
            -H "Authorization: Bearer ${net.authToken}" || true`,
    }, { parent });

    return {
        id: volumeCmd.stdout.apply(s => s.trim()),
        name: volumeName,
        sizeGb: config.storage.dbDiskSizeGb,
        attachedTo: server.hostname,
    };
}

/**
 * Provision block storage volumes for all database nodes.
 *
 * In dev (co-located), we attach the volume to the first control-plane node.
 * In staging/prod, each dedicated database node gets its own volume.
 */
export function provisionStorage(
    config: PlatformConfig,
    net: NetworkResult,
    dbServers: ServerInfo[],
    cpServers: ServerInfo[],
): StorageResult {
    const volumes: StorageVolume[] = [];

    if (config.database.colocated) {
        // Dev mode: attach a single volume to the first control-plane node
        if (cpServers.length > 0) {
            volumes.push(
                createAndAttachVolume(config, net, cpServers[0], 0),
            );
        }
    } else {
        // Staging/prod: one volume per database node
        for (let i = 0; i < dbServers.length; i++) {
            volumes.push(
                createAndAttachVolume(config, net, dbServers[i], i),
            );
        }
    }

    return { volumes };
}
