import * as pulumi from "@pulumi/pulumi";
import * as command from "@pulumi/command";
import * as fs from "fs";
import * as path from "path";
import { PlatformConfig, resourceName } from "./config";
import { ServerInfo } from "./servers";

// ---------------------------------------------------------------------------
// Post-provision SSH commands -- runs install scripts on each node after
// the bare-metal servers have booted and cloud-init has completed.
// ---------------------------------------------------------------------------

/** Shared SSH options for all remote commands. */
const SSH_OPTS = "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30";

/**
 * Read a shell script from scripts/ and return its content.
 */
function loadScript(scriptName: string): string {
    const scriptPath = path.join(__dirname, "..", "scripts", scriptName);
    return fs.readFileSync(scriptPath, "utf-8");
}

/**
 * Execute a script on a remote server over SSH. Waits for cloud-init to
 * finish before running.
 */
function remoteExec(
    name: string,
    server: ServerInfo,
    script: string,
    env: Record<string, pulumi.Input<string>>,
    dependsOn?: pulumi.Resource[],
): command.local.Command {
    // Build environment variable exports for the remote shell
    const envExports = Object.entries(env)
        .map(([k, v]) => pulumi.interpolate`export ${k}='${v}'`)
        .reduce(
            (acc, line) => pulumi.interpolate`${acc}\n${line}`,
            pulumi.interpolate``,
        );

    const fullScript = pulumi.interpolate`#!/usr/bin/env bash
set -euo pipefail
${envExports}

# Wait for cloud-init to finish (max 10 minutes)
timeout 600 bash -c 'while [ ! -f /var/lib/cloud/instance/boot-finished ]; do sleep 5; done' || true

${script}
`;

    return new command.local.Command(name, {
        create: pulumi.interpolate`ssh ${SSH_OPTS} root@${server.publicIp} 'bash -s' <<'STRAND_REMOTE_SCRIPT'
${fullScript}
STRAND_REMOTE_SCRIPT`,
    }, { dependsOn });
}

/**
 * Provision the control-plane nodes with all required services.
 */
function provisionControlPlane(
    config: PlatformConfig,
    servers: ServerInfo[],
): command.local.Command[] {
    const commands: command.local.Command[] = [];

    const installStrandCloud = loadScript("install-strand-cloud.sh");
    const installMonitoring = loadScript("install-monitoring.sh");

    // On dev, also install database software on CP node
    const installPostgres = loadScript("install-postgres.sh");
    const installClickhouse = loadScript("install-clickhouse.sh");

    for (let i = 0; i < servers.length; i++) {
        const server = servers[i];

        // Install strand-cloud service
        const strandCloudCmd = remoteExec(
            resourceName("provision-strand-cloud", i),
            server,
            installStrandCloud,
            {
                STRAND_ENV: config.environment,
                STRAND_DOMAIN: config.domain,
                STRAND_NODE_ROLE: "control-plane",
                STRAND_NODE_INDEX: i.toString(),
                STRAND_CP_COUNT: config.controlPlane.count.toString(),
            },
        );
        commands.push(strandCloudCmd);

        // If co-located database (dev mode), install DB software on CP
        if (config.database.colocated) {
            const pgCmd = remoteExec(
                resourceName("provision-postgres-colocated", i),
                server,
                installPostgres,
                {
                    STRAND_ENV: config.environment,
                    STRAND_DB_DISK: "/data/strand",
                    STRAND_DB_NAME: "strand",
                    STRAND_DB_USER: "strand",
                },
                [strandCloudCmd],
            );
            commands.push(pgCmd);

            const chCmd = remoteExec(
                resourceName("provision-clickhouse-colocated", i),
                server,
                installClickhouse,
                {
                    STRAND_ENV: config.environment,
                    STRAND_CH_DATA_DIR: "/data/strand/clickhouse",
                },
                [pgCmd],
            );
            commands.push(chCmd);
        }

        // Install monitoring on first CP node
        if (i === 0 && config.monitoring.enabled) {
            const monCmd = remoteExec(
                resourceName("provision-monitoring", i),
                server,
                installMonitoring,
                {
                    STRAND_ENV: config.environment,
                    STRAND_DOMAIN: config.domain,
                    GRAFANA_ADMIN_PASSWORD: config.monitoring.grafanaAdminPassword,
                },
                [strandCloudCmd],
            );
            commands.push(monCmd);
        }
    }

    return commands;
}

/**
 * Provision worker nodes with the Strand agent.
 */
function provisionWorkers(
    config: PlatformConfig,
    servers: ServerInfo[],
    cpServers: ServerInfo[],
): command.local.Command[] {
    const commands: command.local.Command[] = [];
    const installStrandCloud = loadScript("install-strand-cloud.sh");

    for (let i = 0; i < servers.length; i++) {
        const server = servers[i];
        const cmd = remoteExec(
            resourceName("provision-worker", i),
            server,
            installStrandCloud,
            {
                STRAND_ENV: config.environment,
                STRAND_DOMAIN: config.domain,
                STRAND_NODE_ROLE: "worker",
                STRAND_NODE_INDEX: i.toString(),
                STRAND_CP_ENDPOINT: cpServers.length > 0
                    ? cpServers[0].privateIp
                    : pulumi.output(""),
            },
        );
        commands.push(cmd);
    }

    return commands;
}

/**
 * Provision dedicated database nodes.
 */
function provisionDatabases(
    config: PlatformConfig,
    servers: ServerInfo[],
): command.local.Command[] {
    const commands: command.local.Command[] = [];

    const installPostgres = loadScript("install-postgres.sh");
    const installClickhouse = loadScript("install-clickhouse.sh");
    const installKratos = loadScript("install-kratos.sh");

    for (let i = 0; i < servers.length; i++) {
        const server = servers[i];

        const pgCmd = remoteExec(
            resourceName("provision-postgres", i),
            server,
            installPostgres,
            {
                STRAND_ENV: config.environment,
                STRAND_DB_DISK: "/data/strand",
                STRAND_DB_NAME: "strand",
                STRAND_DB_USER: "strand",
            },
        );
        commands.push(pgCmd);

        const chCmd = remoteExec(
            resourceName("provision-clickhouse", i),
            server,
            installClickhouse,
            {
                STRAND_ENV: config.environment,
                STRAND_CH_DATA_DIR: "/data/strand/clickhouse",
            },
            [pgCmd],
        );
        commands.push(chCmd);

        // Install Kratos on first database node only
        if (i === 0) {
            const kratosCmd = remoteExec(
                resourceName("provision-kratos", i),
                server,
                installKratos,
                {
                    STRAND_ENV: config.environment,
                    STRAND_DB_HOST: "127.0.0.1",
                    STRAND_DOMAIN: config.domain,
                },
                [pgCmd],
            );
            commands.push(kratosCmd);
        }
    }

    return commands;
}

/**
 * Main entry point: provision all nodes based on their role.
 */
export function provisionAll(
    config: PlatformConfig,
    cpServers: ServerInfo[],
    workerServers: ServerInfo[],
    dbServers: ServerInfo[],
): command.local.Command[] {
    const commands: command.local.Command[] = [];

    commands.push(...provisionControlPlane(config, cpServers));
    commands.push(...provisionWorkers(config, workerServers, cpServers));
    commands.push(...provisionDatabases(config, dbServers));

    return commands;
}
