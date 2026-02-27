import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface BillingResult {
    usageDataset: gcp.bigquery.Dataset;
    exportDataset: gcp.bigquery.Dataset;
    gpuHoursView: gcp.bigquery.Table;
    cpuHoursView: gcp.bigquery.Table;
    monthlySummaryView: gcp.bigquery.Table;
}

export function setupBilling(config: GcpPlatformConfig): BillingResult {
    const labels = standardLabels();

    // BigQuery dataset for GKE resource usage metering
    // GKE writes pod-level CPU/memory/GPU usage per namespace to this dataset
    const usageDataset = new gcp.bigquery.Dataset(resourceName("gke-usage"), {
        datasetId: config.billing.bigQueryDatasetId.replace(/-/g, "_"),
        project: config.gcpProject,
        location: config.gcpRegion,
        description: "GKE resource usage metering for Strand Protocol tenant chargeback",
        defaultTableExpirationMs: 7776000000, // 90 days
        labels,
    });

    // BigQuery dataset for GCP billing export
    const exportDataset = new gcp.bigquery.Dataset(resourceName("billing-export"), {
        datasetId: config.billing.billingExportDatasetId.replace(/-/g, "_"),
        project: config.gcpProject,
        location: config.gcpRegion,
        description: "GCP detailed billing export for Strand Protocol",
        labels,
    });

    // View: GPU hours by namespace (tenant)
    const gpuHoursView = new gcp.bigquery.Table(resourceName("gpu-hours-view"), {
        tableId: "gpu_hours_by_namespace",
        project: config.gcpProject,
        datasetId: usageDataset.datasetId,
        deletionProtection: false,
        view: {
            query: pulumi.interpolate`
SELECT
    namespace,
    SUM(
        TIMESTAMP_DIFF(end_time, start_time, SECOND) / 3600.0
        * usage.amount
    ) AS gpu_hours,
    DATE(start_time) AS date
FROM \`${config.gcpProject}.${usageDataset.datasetId}.gke_cluster_resource_usage\`
WHERE resource_name = 'nvidia.com/gpu'
GROUP BY namespace, date
ORDER BY date DESC, gpu_hours DESC`,
            useLegacySql: false,
        },
        labels,
    });

    // View: CPU hours by namespace (tenant)
    const cpuHoursView = new gcp.bigquery.Table(resourceName("cpu-hours-view"), {
        tableId: "cpu_hours_by_namespace",
        project: config.gcpProject,
        datasetId: usageDataset.datasetId,
        deletionProtection: false,
        view: {
            query: pulumi.interpolate`
SELECT
    namespace,
    SUM(
        TIMESTAMP_DIFF(end_time, start_time, SECOND) / 3600.0
        * usage.amount
    ) AS cpu_hours,
    SUM(
        TIMESTAMP_DIFF(end_time, start_time, SECOND) / 3600.0
        * SAFE_CAST(labels.value AS FLOAT64)
    ) AS memory_gb_hours,
    DATE(start_time) AS date
FROM \`${config.gcpProject}.${usageDataset.datasetId}.gke_cluster_resource_usage\`
WHERE resource_name = 'cpu'
GROUP BY namespace, date
ORDER BY date DESC, cpu_hours DESC`,
            useLegacySql: false,
        },
        labels,
    });

    // View: Monthly tenant cost summary
    const monthlySummaryView = new gcp.bigquery.Table(resourceName("monthly-summary-view"), {
        tableId: "monthly_tenant_cost_summary",
        project: config.gcpProject,
        datasetId: usageDataset.datasetId,
        deletionProtection: false,
        view: {
            query: pulumi.interpolate`
SELECT
    gpu.namespace AS tenant_namespace,
    FORMAT_DATE('%Y-%m', gpu.date) AS month,
    COALESCE(SUM(gpu.gpu_hours), 0) AS total_gpu_hours,
    COALESCE(SUM(cpu.cpu_hours), 0) AS total_cpu_hours,
    COALESCE(SUM(cpu.memory_gb_hours), 0) AS total_memory_gb_hours,
    -- Cost estimates (configurable rates)
    COALESCE(SUM(gpu.gpu_hours), 0) * 0.70 AS gpu_cost_usd,
    COALESCE(SUM(cpu.cpu_hours), 0) * 0.034 AS cpu_cost_usd,
    COALESCE(SUM(cpu.memory_gb_hours), 0) * 0.004 AS memory_cost_usd,
    (COALESCE(SUM(gpu.gpu_hours), 0) * 0.70
     + COALESCE(SUM(cpu.cpu_hours), 0) * 0.034
     + COALESCE(SUM(cpu.memory_gb_hours), 0) * 0.004) AS total_cost_usd
FROM \`${config.gcpProject}.${usageDataset.datasetId}.gpu_hours_by_namespace\` gpu
LEFT JOIN \`${config.gcpProject}.${usageDataset.datasetId}.cpu_hours_by_namespace\` cpu
    ON gpu.namespace = cpu.namespace AND gpu.date = cpu.date
GROUP BY tenant_namespace, month
ORDER BY month DESC, total_cost_usd DESC`,
            useLegacySql: false,
        },
        labels,
    });

    // Budget alerts
    if (config.billing.billingAccountId) {
        new gcp.billing.Budget(resourceName("budget"), {
            billingAccount: config.billing.billingAccountId,
            displayName: `Strand ${config.environment} budget`,
            amount: {
                specifiedAmount: {
                    currencyCode: "USD",
                    units: config.environment === "prod" ? "10000" : "1000",
                },
            },
            thresholdRules: [
                { thresholdPercent: 0.5 },
                { thresholdPercent: 0.8 },
                { thresholdPercent: 1.0 },
                { thresholdPercent: 1.2, spendBasis: "FORECASTED_SPEND" },
            ],
            budgetFilter: {
                projects: [`projects/${config.gcpProject}`],
            },
        });
    }

    return { usageDataset, exportDataset, gpuHoursView, cpuHoursView, monthlySummaryView };
}
