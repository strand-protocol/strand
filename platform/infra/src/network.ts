import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import { PlatformConfig, BMC_API_BASE, BMC_AUTH_URL, resourceName } from "./config";

// ---------------------------------------------------------------------------
// Private network setup on PhoenixNAP BMC
// ---------------------------------------------------------------------------

export interface NetworkResult {
    privateNetworkId: pulumi.Output<string>;
    authToken: pulumi.Output<string>;
}

/**
 * Obtain a bearer token from the PhoenixNAP OAuth2 endpoint.
 * The token is short-lived and used for all subsequent BMC API calls.
 */
function getAuthToken(
    config: PlatformConfig,
    parent?: pulumi.Resource,
): command.local.Command {
    return new command.local.Command(resourceName("bmc-auth"), {
        create: pulumi.interpolate`curl -s -X POST ${BMC_AUTH_URL} \
            -H "Content-Type: application/x-www-form-urlencoded" \
            -d "grant_type=client_credentials" \
            -d "client_id=${config.bmcClientId}" \
            -d "client_secret=${config.bmcClientSecret}" \
            | jq -r '.access_token'`,
        // Re-run on every up to keep the token fresh.
        triggers: [Date.now().toString()],
    }, { parent });
}

/**
 * Create a private network inside the specified location.  Uses the BMC
 * REST API via curl because there is no first-party Pulumi provider for
 * PhoenixNAP private networks.
 */
function createPrivateNetwork(
    config: PlatformConfig,
    token: pulumi.Output<string>,
    parent?: pulumi.Resource,
): command.local.Command {
    const name = resourceName("private-net");
    const body = pulumi.interpolate`{
        "name": "${name}",
        "location": "${config.region}",
        "locationDefault": false,
        "cidr": "${config.network.cidr}",
        "vlanId": ${config.network.vlanId},
        "description": "Strand Protocol private network (${config.environment})"
    }`;

    return new command.local.Command(name, {
        create: pulumi.interpolate`curl -s -X POST ${BMC_API_BASE}/private-networks \
            -H "Authorization: Bearer ${token}" \
            -H "Content-Type: application/json" \
            -d '${body}' \
            | jq -r '.id'`,
        delete: pulumi.interpolate`NETWORK_ID=$(curl -s ${BMC_API_BASE}/private-networks \
            -H "Authorization: Bearer ${token}" \
            | jq -r '.[] | select(.name=="${name}") | .id') && \
            [ -n "$NETWORK_ID" ] && \
            curl -s -X DELETE ${BMC_API_BASE}/private-networks/$NETWORK_ID \
            -H "Authorization: Bearer ${token}" || true`,
    }, { parent });
}

/**
 * Provision the private network and return identifiers needed by downstream
 * resources (servers, storage, etc.).
 */
export function setupNetwork(config: PlatformConfig): NetworkResult {
    const authCmd = getAuthToken(config);
    const token = authCmd.stdout;

    const netCmd = createPrivateNetwork(config, token);
    const privateNetworkId = netCmd.stdout;

    return { privateNetworkId, authToken: token };
}
