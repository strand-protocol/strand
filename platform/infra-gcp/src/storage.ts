import * as gcp from "@pulumi/gcp";
import { GcpPlatformConfig, resourceName, standardLabels } from "./config";

export interface StorageResult {
    etcdBackupBucket: gcp.storage.Bucket;
    modelArtifactsBucket: gcp.storage.Bucket;
}

export function provisionStorage(config: GcpPlatformConfig): StorageResult {
    const labels = standardLabels();

    // GCS bucket for etcd backups
    const etcdBackupBucket = new gcp.storage.Bucket(resourceName("etcd-backups"), {
        name: resourceName("etcd-backups"),
        project: config.gcpProject,
        location: config.gcpRegion,
        uniformBucketLevelAccess: true,
        labels,
        lifecycleRules: [{
            condition: { age: 90 },
            action: { type: "Delete" },
        }],
    });

    // GCS bucket for model artifacts (NIM model cache, custom model weights)
    const modelArtifactsBucket = new gcp.storage.Bucket(resourceName("models"), {
        name: resourceName("models"),
        project: config.gcpProject,
        location: config.gcpRegion,
        uniformBucketLevelAccess: true,
        labels,
        lifecycleRules: [{
            condition: {
                age: 365,
                withState: "ARCHIVED",
            },
            action: { type: "Delete" },
        }],
    });

    return { etcdBackupBucket, modelArtifactsBucket };
}
