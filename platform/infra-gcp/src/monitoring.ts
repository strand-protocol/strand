import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface MonitoringResult {
    notificationChannel: gcp.monitoring.NotificationChannel;
    gpuHighAlert: gcp.monitoring.AlertPolicy;
    gpuLowAlert: gcp.monitoring.AlertPolicy;
    unhealthyAlert: gcp.monitoring.AlertPolicy;
}

export function setupMonitoring(config: GcpPlatformConfig): MonitoringResult {
    // Email notification channel
    const notificationChannel = new gcp.monitoring.NotificationChannel(resourceName("email-channel"), {
        displayName: `Strand ${config.environment} alerts`,
        project: config.gcpProject,
        type: "email",
        labels: {
            email_address: config.monitoring.alertNotificationChannel,
        },
    });

    // Alert: GPU utilization > 90% for 5 min (need to scale up)
    const gpuHighAlert = new gcp.monitoring.AlertPolicy(resourceName("gpu-high"), {
        displayName: `[${config.environment}] GPU utilization high (>90%)`,
        project: config.gcpProject,
        combiner: "OR",
        conditions: [{
            displayName: "GPU utilization > 90%",
            conditionThreshold: {
                filter: `resource.type = "k8s_node" AND metric.type = "kubernetes.io/node/accelerator/duty_cycle"`,
                comparison: "COMPARISON_GT",
                thresholdValue: 90,
                duration: "300s",
                aggregations: [{
                    alignmentPeriod: "60s",
                    perSeriesAligner: "ALIGN_MEAN",
                }],
            },
        }],
        notificationChannels: [notificationChannel.id],
        alertStrategy: {
            autoClose: "1800s",
        },
    });

    // Alert: GPU utilization < 10% for 30 min (wasted spend)
    const gpuLowAlert = new gcp.monitoring.AlertPolicy(resourceName("gpu-low"), {
        displayName: `[${config.environment}] GPU utilization low (<10%)`,
        project: config.gcpProject,
        combiner: "OR",
        conditions: [{
            displayName: "GPU utilization < 10%",
            conditionThreshold: {
                filter: `resource.type = "k8s_node" AND metric.type = "kubernetes.io/node/accelerator/duty_cycle"`,
                comparison: "COMPARISON_LT",
                thresholdValue: 10,
                duration: "1800s",
                aggregations: [{
                    alignmentPeriod: "60s",
                    perSeriesAligner: "ALIGN_MEAN",
                }],
            },
        }],
        notificationChannels: [notificationChannel.id],
        alertStrategy: {
            autoClose: "1800s",
        },
    });

    // Alert: strand-cloud pod unhealthy for > 2 min
    const unhealthyAlert = new gcp.monitoring.AlertPolicy(resourceName("unhealthy"), {
        displayName: `[${config.environment}] strand-cloud unhealthy`,
        project: config.gcpProject,
        combiner: "OR",
        conditions: [{
            displayName: "strand-cloud pod not ready",
            conditionThreshold: {
                filter: `resource.type = "k8s_container" AND resource.labels.container_name = "strand-cloud" AND metric.type = "kubernetes.io/container/restart_count"`,
                comparison: "COMPARISON_GT",
                thresholdValue: 3,
                duration: "120s",
                aggregations: [{
                    alignmentPeriod: "60s",
                    perSeriesAligner: "ALIGN_RATE",
                }],
            },
        }],
        notificationChannels: [notificationChannel.id],
        alertStrategy: {
            autoClose: "3600s",
        },
    });

    return { notificationChannel, gpuHighAlert, gpuLowAlert, unhealthyAlert };
}
