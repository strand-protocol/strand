import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName } from "./config";

export interface IamResult {
    nodeServiceAccount: gcp.serviceaccount.Account;
    strandCloudSa: gcp.serviceaccount.Account;
    inferenceSa: gcp.serviceaccount.Account;
    billingSa: gcp.serviceaccount.Account;
}

export function setupIam(config: GcpPlatformConfig): IamResult {
    // GKE node service account (shared by all node pools)
    const nodeServiceAccount = new gcp.serviceaccount.Account(resourceName("gke-node"), {
        accountId: resourceName("gke-node"),
        displayName: "Strand GKE Node SA",
        project: config.gcpProject,
    });

    const nodeRoles = [
        "roles/logging.logWriter",
        "roles/monitoring.metricWriter",
        "roles/monitoring.viewer",
        "roles/stackdriver.resourceMetadata.writer",
        "roles/artifactregistry.reader",
    ];

    nodeRoles.forEach((role, i) => {
        new gcp.projects.IAMMember(`node-role-${i}`, {
            project: config.gcpProject,
            role,
            member: nodeServiceAccount.email.apply(e => `serviceAccount:${e}`),
        });
    });

    // strand-cloud Workload Identity SA
    const strandCloudSa = new gcp.serviceaccount.Account(resourceName("cloud-wi"), {
        accountId: resourceName("cloud-wi"),
        displayName: "Strand Cloud Workload Identity SA",
        project: config.gcpProject,
    });

    new gcp.projects.IAMMember("cloud-wi-monitoring", {
        project: config.gcpProject,
        role: "roles/monitoring.metricWriter",
        member: strandCloudSa.email.apply(e => `serviceAccount:${e}`),
    });

    // Workload Identity binding: K8s SA strand-cloud in strand-system -> GCP SA
    new gcp.serviceaccount.IAMMember("cloud-wi-binding", {
        serviceAccountId: strandCloudSa.name,
        role: "roles/iam.workloadIdentityUser",
        member: strandCloudSa.email.apply(() =>
            `serviceAccount:${config.gcpProject}.svc.id.goog[strand-system/strand-cloud]`,
        ),
    });

    // Inference Workload Identity SA
    const inferenceSa = new gcp.serviceaccount.Account(resourceName("inference-wi"), {
        accountId: resourceName("inference-wi"),
        displayName: "Strand Inference Workload Identity SA",
        project: config.gcpProject,
    });

    new gcp.projects.IAMMember("inference-wi-monitoring", {
        project: config.gcpProject,
        role: "roles/monitoring.metricWriter",
        member: inferenceSa.email.apply(e => `serviceAccount:${e}`),
    });

    new gcp.serviceaccount.IAMMember("inference-wi-binding", {
        serviceAccountId: inferenceSa.name,
        role: "roles/iam.workloadIdentityUser",
        member: inferenceSa.email.apply(() =>
            `serviceAccount:${config.gcpProject}.svc.id.goog[strand-system/strandapi-inference]`,
        ),
    });

    // Billing SA for BigQuery access
    const billingSa = new gcp.serviceaccount.Account(resourceName("billing"), {
        accountId: resourceName("billing"),
        displayName: "Strand Billing SA",
        project: config.gcpProject,
    });

    const billingRoles = [
        "roles/bigquery.dataEditor",
        "roles/bigquery.jobUser",
    ];

    billingRoles.forEach((role, i) => {
        new gcp.projects.IAMMember(`billing-role-${i}`, {
            project: config.gcpProject,
            role,
            member: billingSa.email.apply(e => `serviceAccount:${e}`),
        });
    });

    return { nodeServiceAccount, strandCloudSa, inferenceSa, billingSa };
}
