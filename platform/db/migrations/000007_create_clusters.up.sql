-- 000007_create_clusters.up.sql
-- Customer-managed clusters deployed on bare metal or cloud.

CREATE TABLE clusters (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    region                  TEXT NOT NULL DEFAULT 'PHX',
    status                  TEXT NOT NULL DEFAULT 'provisioning'
                                CHECK (status IN ('provisioning', 'active', 'degraded',
                                                  'suspended', 'deprovisioning')),
    control_plane_endpoint  TEXT,
    node_count              INT NOT NULL DEFAULT 0,
    config                  JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_clusters_tenant ON clusters(tenant_id);
CREATE INDEX idx_clusters_status ON clusters(status);

ALTER TABLE clusters ENABLE ROW LEVEL SECURITY;

CREATE POLICY cluster_tenant_isolation ON clusters
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE TRIGGER clusters_updated_at
    BEFORE UPDATE ON clusters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
