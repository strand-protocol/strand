-- 000002_create_users.up.sql
-- User accounts linked to Ory Kratos identities and scoped to tenants.

CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kratos_identity_id  TEXT UNIQUE,
    email               TEXT NOT NULL,
    display_name        TEXT NOT NULL DEFAULT '',
    role                TEXT NOT NULL DEFAULT 'viewer'
                            CHECK (role IN ('viewer', 'operator', 'admin', 'owner')),
    status              TEXT NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'invited', 'disabled')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_users_kratos ON users(kratos_identity_id) WHERE kratos_identity_id IS NOT NULL;
CREATE INDEX idx_users_email ON users(email);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_tenant_isolation ON users
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
