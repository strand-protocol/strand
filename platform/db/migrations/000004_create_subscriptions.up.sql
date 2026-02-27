-- 000004_create_subscriptions.up.sql
-- Subscription and billing state per tenant.

CREATE TABLE subscriptions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plan                    TEXT NOT NULL CHECK (plan IN ('free', 'starter', 'pro', 'enterprise')),
    billing_cycle           TEXT NOT NULL DEFAULT 'monthly'
                                CHECK (billing_cycle IN ('monthly', 'annual')),
    base_price_cents        INT NOT NULL DEFAULT 0,
    mic_price_cents         INT NOT NULL DEFAULT 300,
    traffic_price_cents     INT NOT NULL DEFAULT 8,
    status                  TEXT NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'past_due', 'cancelled', 'trialing')),
    trial_ends_at           TIMESTAMPTZ,
    current_period_start    TIMESTAMPTZ NOT NULL DEFAULT now(),
    current_period_end      TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '1 month'),
    stripe_subscription_id  TEXT,
    stripe_customer_id      TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Only one active subscription per tenant
CREATE UNIQUE INDEX idx_subscriptions_tenant_active
    ON subscriptions(tenant_id) WHERE status IN ('active', 'trialing');

CREATE TRIGGER subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
