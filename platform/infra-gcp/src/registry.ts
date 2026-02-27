import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface RegistryResult {
    repository: gcp.artifactregistry.Repository;
    repositoryUrl: pulumi.Output<string>;
}

export function createRegistry(config: GcpPlatformConfig): RegistryResult {
    const repository = new gcp.artifactregistry.Repository(resourceName("images"), {
        repositoryId: resourceName("images"),
        project: config.gcpProject,
        location: config.gcpRegion,
        format: "DOCKER",
        description: "Strand Protocol container images",
        labels: standardLabels(),
        cleanupPolicies: [
            {
                id: "keep-tagged",
                action: "KEEP",
                condition: {
                    tagState: "TAGGED",
                },
                mostRecentVersions: {
                    keepCount: 10,
                },
            },
            {
                id: "delete-untagged",
                action: "DELETE",
                condition: {
                    tagState: "UNTAGGED",
                    olderThan: "604800s", // 7 days
                },
            },
        ],
    });

    const repositoryUrl = pulumi.interpolate`${config.gcpRegion}-docker.pkg.dev/${config.gcpProject}/${repository.repositoryId}`;

    return { repository, repositoryUrl };
}
