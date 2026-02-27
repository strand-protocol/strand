#!/usr/bin/env bash
# install-postgres.sh -- Install and configure PostgreSQL 16 for Strand Protocol
#
# Expected environment variables:
#   STRAND_ENV       - Environment (dev/staging/prod)
#   STRAND_DB_DISK   - Data directory mount point (e.g. /data/strand)
#   STRAND_DB_NAME   - Database name (default: strand)
#   STRAND_DB_USER   - Database user (default: strand)
set -euo pipefail

STRAND_DB_NAME="${STRAND_DB_NAME:-strand}"
STRAND_DB_USER="${STRAND_DB_USER:-strand}"
STRAND_DB_DISK="${STRAND_DB_DISK:-/data/strand}"
PG_DATA_DIR="${STRAND_DB_DISK}/postgresql/16/main"
PG_VERSION="16"

echo "==> Installing PostgreSQL ${PG_VERSION}..."

# Add PostgreSQL APT repository
if [ ! -f /etc/apt/sources.list.d/pgdg.list ]; then
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
        | gpg --dearmor -o /usr/share/keyrings/postgresql-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/postgresql-keyring.gpg] \
        http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
        > /etc/apt/sources.list.d/pgdg.list
fi

apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    postgresql-${PG_VERSION} \
    postgresql-contrib-${PG_VERSION} \
    prometheus-postgres-exporter

echo "==> Stopping PostgreSQL to reconfigure data directory..."
systemctl stop postgresql

# Move data directory to the block storage volume
if [ -d "${STRAND_DB_DISK}/postgresql" ]; then
    mkdir -p "${PG_DATA_DIR}"
    chown -R postgres:postgres "${STRAND_DB_DISK}/postgresql"

    # Only move if the default data directory exists and target is empty
    if [ -d /var/lib/postgresql/${PG_VERSION}/main ] && [ ! -f "${PG_DATA_DIR}/PG_VERSION" ]; then
        rsync -av /var/lib/postgresql/${PG_VERSION}/main/ "${PG_DATA_DIR}/"
        chown -R postgres:postgres "${PG_DATA_DIR}"
    fi
fi

# Configure PostgreSQL
PG_CONF="/etc/postgresql/${PG_VERSION}/main/postgresql.conf"
PG_HBA="/etc/postgresql/${PG_VERSION}/main/pg_hba.conf"

cat > "${PG_CONF}" <<PGCONF
# Strand Protocol PostgreSQL Configuration
# Environment: ${STRAND_ENV}

# Connection
listen_addresses = '*'
port = 5432
max_connections = 200

# Data directory
data_directory = '${PG_DATA_DIR}'

# Memory -- tuned per environment
PGCONF

case "${STRAND_ENV}" in
    dev)
        cat >> "${PG_CONF}" <<PGMEM
shared_buffers = 256MB
effective_cache_size = 768MB
work_mem = 4MB
maintenance_work_mem = 64MB
PGMEM
        ;;
    staging)
        cat >> "${PG_CONF}" <<PGMEM
shared_buffers = 2GB
effective_cache_size = 6GB
work_mem = 16MB
maintenance_work_mem = 512MB
PGMEM
        ;;
    prod)
        cat >> "${PG_CONF}" <<PGMEM
shared_buffers = 8GB
effective_cache_size = 24GB
work_mem = 32MB
maintenance_work_mem = 1GB
huge_pages = try
PGMEM
        ;;
esac

cat >> "${PG_CONF}" <<PGCONF

# WAL
wal_level = replica
max_wal_size = 2GB
min_wal_size = 256MB
wal_compression = lz4
wal_buffers = 64MB

# Checkpoints
checkpoint_completion_target = 0.9
checkpoint_timeout = 15min

# Query planner
random_page_cost = 1.1
effective_io_concurrency = 200
default_statistics_target = 100

# Logging
log_destination = 'stderr'
logging_collector = on
log_directory = '/var/log/strand/postgresql'
log_filename = 'postgresql-%Y-%m-%d.log'
log_rotation_age = 1d
log_rotation_size = 100MB
log_min_duration_statement = 500
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
log_temp_files = 0
log_timezone = 'UTC'

# Locale
timezone = 'UTC'
lc_messages = 'en_US.UTF-8'

# SSL -- require for non-localhost connections
ssl = on
ssl_cert_file = '/etc/ssl/certs/ssl-cert-snakeoil.pem'
ssl_key_file = '/etc/ssl/private/ssl-cert-snakeoil.key'
PGCONF

# Configure pg_hba.conf
cat > "${PG_HBA}" <<PGHBA
# Strand Protocol PostgreSQL HBA Configuration
# TYPE    DATABASE    USER        ADDRESS             METHOD

# Local connections
local   all         postgres                        peer
local   all         all                             peer

# Loopback (IPv4/IPv6)
host    all         all         127.0.0.1/32        scram-sha-256
host    all         all         ::1/128             scram-sha-256

# Private network -- allow strand user with password auth
host    ${STRAND_DB_NAME}  ${STRAND_DB_USER}  10.0.0.0/8      scram-sha-256

# Replication (staging/prod)
host    replication all         10.0.0.0/8          scram-sha-256
PGHBA

# Create log directory
mkdir -p /var/log/strand/postgresql
chown postgres:postgres /var/log/strand/postgresql

# Start PostgreSQL
echo "==> Starting PostgreSQL..."
systemctl start postgresql
systemctl enable postgresql

# Wait for PostgreSQL to be ready
for i in $(seq 1 30); do
    if su - postgres -c "pg_isready" > /dev/null 2>&1; then
        break
    fi
    sleep 2
done

# Create database and user
echo "==> Creating Strand database and user..."
su - postgres -c "psql" <<SQL
-- Create the strand user (password will be set via Pulumi secret injection)
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${STRAND_DB_USER}') THEN
        CREATE ROLE ${STRAND_DB_USER} WITH LOGIN PASSWORD 'strand_changeme_on_first_boot';
    END IF;
END
\$\$;

-- Create the strand database
SELECT 'CREATE DATABASE ${STRAND_DB_NAME} OWNER ${STRAND_DB_USER}'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${STRAND_DB_NAME}')
\gexec

-- Connect to strand database and create schema
\c ${STRAND_DB_NAME}

-- Nodes table
CREATE TABLE IF NOT EXISTS nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id         BYTEA NOT NULL UNIQUE,
    hostname        TEXT NOT NULL,
    public_key      BYTEA NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    region          TEXT,
    capabilities    JSONB DEFAULT '[]'::jsonb,
    last_heartbeat  TIMESTAMPTZ,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_region ON nodes(region);
CREATE INDEX IF NOT EXISTS idx_nodes_last_heartbeat ON nodes(last_heartbeat);

-- Routes table
CREATE TABLE IF NOT EXISTS routes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sad_descriptor  BYTEA NOT NULL,
    target_node_id  BYTEA NOT NULL REFERENCES nodes(node_id),
    priority        INTEGER NOT NULL DEFAULT 0,
    weight          REAL NOT NULL DEFAULT 1.0,
    ttl_seconds     INTEGER NOT NULL DEFAULT 300,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_routes_target ON routes(target_node_id);
CREATE INDEX IF NOT EXISTS idx_routes_expires ON routes(expires_at);

-- Firmware images table
CREATE TABLE IF NOT EXISTS firmware_images (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    version         TEXT NOT NULL,
    checksum_sha256 TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    storage_url     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'uploaded',
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, version)
);

-- MIC certificates table
CREATE TABLE IF NOT EXISTS certificates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial_number   BYTEA NOT NULL UNIQUE,
    node_id         BYTEA NOT NULL,
    public_key      BYTEA NOT NULL,
    issuer_node_id  BYTEA,
    not_before      TIMESTAMPTZ NOT NULL,
    not_after       TIMESTAMPTZ NOT NULL,
    revoked         BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at      TIMESTAMPTZ,
    claims          JSONB DEFAULT '[]'::jsonb,
    raw_mic         BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_certs_node ON certificates(node_id);
CREATE INDEX IF NOT EXISTS idx_certs_revoked ON certificates(revoked) WHERE revoked = TRUE;
CREATE INDEX IF NOT EXISTS idx_certs_expiry ON certificates(not_after);

-- Tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    api_key_hash    TEXT,
    status          TEXT NOT NULL DEFAULT 'active',
    quotas          JSONB DEFAULT '{}'::jsonb,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE ${STRAND_DB_NAME} TO ${STRAND_DB_USER};
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${STRAND_DB_USER};
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${STRAND_DB_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO ${STRAND_DB_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO ${STRAND_DB_USER};
SQL

# Configure postgres_exporter for Prometheus
cat > /etc/default/prometheus-postgres-exporter <<PGEXP
DATA_SOURCE_NAME="postgresql://postgres@localhost:5432/strand?sslmode=disable"
PGEXP

systemctl restart prometheus-postgres-exporter 2>/dev/null || true
systemctl enable prometheus-postgres-exporter 2>/dev/null || true

echo "==> PostgreSQL ${PG_VERSION} installation complete."
echo "    Database: ${STRAND_DB_NAME}"
echo "    User: ${STRAND_DB_USER}"
echo "    Data dir: ${PG_DATA_DIR}"
