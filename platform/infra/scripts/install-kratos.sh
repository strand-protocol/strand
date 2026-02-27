#!/usr/bin/env bash
# install-kratos.sh -- Install and configure Ory Kratos for Strand identity management
#
# Expected environment variables:
#   STRAND_ENV       - Environment (dev/staging/prod)
#   STRAND_DB_HOST   - PostgreSQL host (default: 127.0.0.1)
#   STRAND_DOMAIN    - Domain for URLs (e.g. dev.strand-protocol.net)
set -euo pipefail

KRATOS_VERSION="1.1.0"
STRAND_DB_HOST="${STRAND_DB_HOST:-127.0.0.1}"
STRAND_DOMAIN="${STRAND_DOMAIN:-localhost}"

echo "==> Installing Ory Kratos v${KRATOS_VERSION}..."

# Detect architecture
ARCH=$(uname -m)
case "${ARCH}" in
    x86_64)  KRATOS_ARCH="linux_64bit" ;;
    aarch64) KRATOS_ARCH="linux_arm64" ;;
    *)       echo "Unsupported architecture: ${ARCH}"; exit 1 ;;
esac

# Download Kratos binary
KRATOS_URL="https://github.com/ory/kratos/releases/download/v${KRATOS_VERSION}/kratos_${KRATOS_VERSION}-${KRATOS_ARCH}.tar.gz"
mkdir -p /tmp/kratos-install
cd /tmp/kratos-install

curl -fsSL "${KRATOS_URL}" -o kratos.tar.gz
tar xzf kratos.tar.gz
mv kratos /usr/local/bin/kratos
chmod +x /usr/local/bin/kratos
cd /
rm -rf /tmp/kratos-install

# Verify installation
kratos version

# Create Kratos user and directories
useradd --system --no-create-home --shell /usr/sbin/nologin kratos 2>/dev/null || true
mkdir -p /etc/kratos
mkdir -p /var/lib/kratos
mkdir -p /var/log/strand/kratos
chown kratos:kratos /var/lib/kratos /var/log/strand/kratos

# Create the Kratos database in PostgreSQL
su - postgres -c "psql" <<SQL 2>/dev/null || true
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_database WHERE datname = 'kratos') THEN
        CREATE DATABASE kratos OWNER strand;
    END IF;
END
\$\$;
SQL

# Determine cookie domain and base URLs
case "${STRAND_ENV}" in
    dev)
        PUBLIC_URL="http://localhost:4433"
        ADMIN_URL="http://localhost:4434"
        COOKIE_DOMAIN="localhost"
        ;;
    *)
        PUBLIC_URL="https://auth.${STRAND_DOMAIN}"
        ADMIN_URL="http://127.0.0.1:4434"
        COOKIE_DOMAIN="${STRAND_DOMAIN}"
        ;;
esac

# Generate Kratos configuration
cat > /etc/kratos/kratos.yml <<KRATOSCONF
version: v1.1.0

dsn: postgres://strand:strand_changeme_on_first_boot@${STRAND_DB_HOST}:5432/kratos?sslmode=disable&max_conns=20&max_idle_conns=4

serve:
  public:
    base_url: ${PUBLIC_URL}
    host: 0.0.0.0
    port: 4433
    cors:
      enabled: true
      allowed_origins:
        - https://*.${STRAND_DOMAIN}
      allowed_methods:
        - GET
        - POST
        - PUT
        - PATCH
        - DELETE
      allowed_headers:
        - Authorization
        - Content-Type
        - X-Session-Token
      exposed_headers:
        - Content-Type
  admin:
    base_url: ${ADMIN_URL}
    host: 127.0.0.1
    port: 4434

log:
  level: info
  format: json
  leak_sensitive_values: false

secrets:
  cookie:
    - CHANGEME-GENERATE-A-REAL-SECRET-ON-FIRST-BOOT
  cipher:
    - CHANGEME-32-CHAR-CIPHER-SECRET!!

hashers:
  argon2:
    parallelism: 1
    memory: 131072
    iterations: 2
    salt_length: 16
    key_length: 16

identity:
  default_schema_id: strand_operator
  schemas:
    - id: strand_operator
      url: file:///etc/kratos/schemas/operator.schema.json

selfservice:
  default_browser_return_url: https://${STRAND_DOMAIN}/
  allowed_return_urls:
    - https://${STRAND_DOMAIN}

  methods:
    password:
      enabled: true
      config:
        haveibeenpwned_enabled: true
        min_password_length: 12
    totp:
      enabled: true
      config:
        issuer: StrandProtocol

  flows:
    login:
      ui_url: https://${STRAND_DOMAIN}/auth/login
      lifespan: 10m
      after:
        password:
          hooks:
            - hook: require_verified_address

    registration:
      ui_url: https://${STRAND_DOMAIN}/auth/registration
      lifespan: 10m
      after:
        password:
          hooks:
            - hook: session

    verification:
      enabled: true
      ui_url: https://${STRAND_DOMAIN}/auth/verification
      lifespan: 1h

    recovery:
      enabled: true
      ui_url: https://${STRAND_DOMAIN}/auth/recovery
      lifespan: 1h

    settings:
      ui_url: https://${STRAND_DOMAIN}/auth/settings
      privileged_session_max_age: 15m

    logout:
      after:
        default_browser_return_url: https://${STRAND_DOMAIN}/

session:
  lifespan: 24h
  cookie:
    domain: ${COOKIE_DOMAIN}
    same_site: Lax
KRATOSCONF

# Create identity schema for Strand operators
mkdir -p /etc/kratos/schemas
cat > /etc/kratos/schemas/operator.schema.json <<'SCHEMA'
{
  "$id": "https://schemas.strand-protocol.net/operator.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Strand Protocol Operator",
  "type": "object",
  "properties": {
    "traits": {
      "type": "object",
      "properties": {
        "email": {
          "type": "string",
          "format": "email",
          "title": "Email",
          "ory.sh/kratos": {
            "credentials": {
              "password": {
                "identifier": true
              }
            },
            "verification": {
              "via": "email"
            },
            "recovery": {
              "via": "email"
            }
          }
        },
        "name": {
          "type": "object",
          "properties": {
            "first": {
              "type": "string",
              "title": "First Name",
              "minLength": 1
            },
            "last": {
              "type": "string",
              "title": "Last Name",
              "minLength": 1
            }
          },
          "required": ["first", "last"]
        },
        "organization": {
          "type": "string",
          "title": "Organization"
        },
        "role": {
          "type": "string",
          "title": "Role",
          "enum": ["admin", "operator", "viewer"],
          "default": "viewer"
        }
      },
      "required": ["email", "name"],
      "additionalProperties": false
    }
  }
}
SCHEMA

# Run Kratos migrations
echo "==> Running Kratos database migrations..."
kratos migrate sql -e --yes --config /etc/kratos/kratos.yml 2>/dev/null || true

# Create systemd service
cat > /etc/systemd/system/kratos.service <<'SVCEOF'
[Unit]
Description=Ory Kratos Identity Server
Documentation=https://www.ory.sh/kratos/docs/
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=kratos
Group=kratos
ExecStart=/usr/local/bin/kratos serve --config /etc/kratos/kratos.yml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

StandardOutput=append:/var/log/strand/kratos/kratos.log
StandardError=append:/var/log/strand/kratos/kratos-error.log

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/kratos /var/log/strand/kratos
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SVCEOF

# Start Kratos
systemctl daemon-reload
systemctl enable kratos
systemctl start kratos

echo "==> Ory Kratos v${KRATOS_VERSION} installation complete."
echo "    Public API: ${PUBLIC_URL}"
echo "    Admin API: ${ADMIN_URL}"
echo "    Config: /etc/kratos/kratos.yml"
