-- dev_seed.sql
-- Seed data for local development. Run after migrations.
-- Usage: psql -d strand -f dev_seed.sql

-- Bypass RLS for seeding
SET app.current_tenant_id = '00000000-0000-0000-0000-000000000000';

-- Demo tenant: Acme AI
INSERT INTO tenants (id, name, slug, plan, status, max_clusters, max_nodes, max_mics_month, traffic_gb_included)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'Acme AI',
    'acme-ai',
    'pro',
    'active',
    3, 50, 10000, 100.0
) ON CONFLICT (slug) DO NOTHING;

-- Demo tenant: Free Tier User
INSERT INTO tenants (id, name, slug, plan, status, max_clusters, max_nodes, max_mics_month, traffic_gb_included)
VALUES (
    'a0000000-0000-0000-0000-000000000002',
    'Dev Sandbox',
    'dev-sandbox',
    'free',
    'active',
    1, 3, 100, 1.0
) ON CONFLICT (slug) DO NOTHING;

-- Set tenant context for RLS
SET app.current_tenant_id = 'a0000000-0000-0000-0000-000000000001';

-- Admin user for Acme AI
INSERT INTO users (id, tenant_id, email, display_name, role, status)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'admin@acme-ai.com',
    'Alice Admin',
    'owner',
    'active'
) ON CONFLICT (tenant_id, email) DO NOTHING;

-- Operator user
INSERT INTO users (id, tenant_id, email, display_name, role, status)
VALUES (
    'b0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'ops@acme-ai.com',
    'Bob Operator',
    'operator',
    'active'
) ON CONFLICT (tenant_id, email) DO NOTHING;

-- Subscription for Acme AI
INSERT INTO subscriptions (id, tenant_id, plan, billing_cycle, base_price_cents, mic_price_cents, traffic_price_cents, status)
VALUES (
    'c0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'pro',
    'monthly',
    500000,  -- $5,000/mo
    250,     -- $2.50/MIC overage
    5,       -- $0.05/GB overage
    'active'
) ON CONFLICT DO NOTHING;

-- Cluster for Acme AI
INSERT INTO clusters (id, tenant_id, name, region, status, node_count)
VALUES (
    'd0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'us-west-inference',
    'PHX',
    'active',
    5
) ON CONFLICT (tenant_id, name) DO NOTHING;

-- Usage record for current month
INSERT INTO usage_records (tenant_id, period_start, period_end, mics_issued, traffic_bytes, node_hours)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    date_trunc('month', now())::date,
    (date_trunc('month', now()) + INTERVAL '1 month')::date,
    2847,
    53687091200,  -- ~50 GB
    3600.0
) ON CONFLICT DO NOTHING;

-- Free tier subscription
SET app.current_tenant_id = 'a0000000-0000-0000-0000-000000000002';

INSERT INTO users (id, tenant_id, email, display_name, role, status)
VALUES (
    'b0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000002',
    'dev@sandbox.io',
    'Charlie Dev',
    'owner',
    'active'
) ON CONFLICT (tenant_id, email) DO NOTHING;

INSERT INTO subscriptions (id, tenant_id, plan, billing_cycle, base_price_cents, status)
VALUES (
    'c0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000002',
    'free',
    'monthly',
    0,
    'active'
) ON CONFLICT DO NOTHING;

RESET app.current_tenant_id;
