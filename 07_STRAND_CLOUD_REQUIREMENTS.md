# Nexus Cloud — Control Plane & Managed Infrastructure

## Module: `nexus-cloud/`

## Language: Go (1.22+) + Rust (for performance-critical data plane components)

---

## 1. Overview

Nexus Cloud is the managed control plane and orchestration layer for Nexus Protocol deployments. It provides fleet management, centralized configuration, multi-tenant isolation, observability, and a management API for enterprise and cloud deployments. Nexus Cloud is to the Nexus Protocol what a Kubernetes control plane is to container workloads — it turns a collection of Nexus-enabled nodes into a managed, observable, policy-driven network.

Nexus Cloud runs as a set of microservices deployable on Kubernetes or as a standalone binary for single-node development.

---

## 2. References

| Reference | Relevance |
|-----------|-----------|
| **Kubernetes Control Plane Architecture** | Primary architectural reference. Nexus Cloud follows the API server + controller + etcd pattern |
| **Istio Control Plane (istiod)** | Reference for service mesh control plane design, config distribution, and policy enforcement |
| **Envoy xDS Protocol** | Reference for dynamic configuration distribution to data plane nodes |
| **Cilium / eBPF Networking** | Reference for cloud-native network policy enforcement |
| **RFC 8040** (RESTCONF) | Reference for RESTful configuration protocol |
| **OpenTelemetry Specification** | Reference for observability data collection and export |
| **etcd v3 API** | Consistent key-value store for control plane state |
| **NATS / JetStream** | Reference for lightweight control plane messaging |

---

## 3. Architecture

### 3.1 Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Nexus Cloud Control Plane                  │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  API Server   │  │  Config Dist │  │  Fleet Controller    │  │
│  │  (REST/gRPC)  │  │  (xDS-like)  │  │  (reconciliation)    │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│         │                 │                      │               │
│  ┌──────▼─────────────────▼──────────────────────▼───────────┐  │
│  │                     State Store (etcd)                      │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  CA Service   │  │  Metrics     │  │  Tenant Manager      │  │
│  │  (NexTrust)   │  │  Aggregator  │  │  (isolation/billing)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
└──────────────────────────────┬──────────────────────────────────┘
                               │  Config push / Metrics pull
                               ▼
              ┌────────────────────────────────┐
              │      Nexus Data Plane Nodes     │
              │  (NexLink + NexRoute + NexStream │
              │   + NexTrust + NexAPI)           │
              └────────────────────────────────┘
```

### 3.2 Source Tree Structure

```
nexus-cloud/
├── go.mod
├── go.sum
├── cmd/
│   ├── nexus-apiserver/
│   │   └── main.go                    # API server binary
│   ├── nexus-controller/
│   │   └── main.go                    # Fleet controller binary
│   ├── nexus-ca/
│   │   └── main.go                    # NexTrust CA service binary
│   ├── nexus-metrics/
│   │   └── main.go                    # Metrics aggregator binary
│   └── nexus-allinone/
│       └── main.go                    # All-in-one binary for dev/single-node
├── pkg/
│   ├── apiserver/
│   │   ├── server.go                  # HTTP/gRPC API server
│   │   ├── handlers/
│   │   │   ├── nodes.go               # Node CRUD API
│   │   │   ├── routes.go              # Routing policy API
│   │   │   ├── trust.go               # MIC management API
│   │   │   ├── tenants.go             # Tenant management API
│   │   │   ├── firmware.go            # Firmware management API
│   │   │   └── config.go              # Configuration distribution API
│   │   ├── middleware/
│   │   │   ├── auth.go                # Authentication middleware
│   │   │   ├── rbac.go                # Role-based access control
│   │   │   ├── audit.go               # Audit logging
│   │   │   └── ratelimit.go           # API rate limiting
│   │   └── openapi/
│   │       └── spec.yaml              # OpenAPI 3.1 specification
│   ├── controller/
│   │   ├── controller.go              # Main reconciliation loop
│   │   ├── node_controller.go         # Node lifecycle management
│   │   ├── route_controller.go        # Route policy reconciliation
│   │   ├── firmware_controller.go     # Firmware rollout controller
│   │   ├── health_controller.go       # Fleet health monitoring
│   │   └── gc_controller.go           # Garbage collection (stale entries, expired MICs)
│   ├── configdist/
│   │   ├── distributor.go             # Configuration distribution engine
│   │   ├── xds.go                     # xDS-like protocol for config push
│   │   ├── snapshot.go                # Configuration snapshots
│   │   └── versioning.go              # Config version management
│   ├── ca/
│   │   ├── service.go                 # NexTrust CA as a service
│   │   ├── issuer.go                  # MIC issuance workflow
│   │   ├── revocation.go              # Revocation management
│   │   ├── transparency.go            # Transparency log service
│   │   └── hsm.go                     # Hardware Security Module integration
│   ├── tenant/
│   │   ├── manager.go                 # Multi-tenant isolation
│   │   ├── quota.go                   # Resource quotas per tenant
│   │   ├── billing.go                 # Usage tracking and billing integration
│   │   └── isolation.go               # Network namespace isolation
│   ├── metrics/
│   │   ├── aggregator.go              # Metrics collection and aggregation
│   │   ├── prometheus.go              # Prometheus exposition
│   │   ├── alerting.go                # Alert rule evaluation
│   │   └── storage.go                 # Metrics storage backend (pluggable)
│   ├── store/
│   │   ├── store.go                   # State store interface
│   │   ├── etcd.go                    # etcd backend
│   │   ├── memory.go                  # In-memory backend (testing/dev)
│   │   └── models/
│   │       ├── node.go                # Node data model
│   │       ├── route_policy.go        # Route policy data model
│   │       ├── tenant.go              # Tenant data model
│   │       ├── mic.go                 # MIC data model
│   │       └── firmware.go            # Firmware version data model
│   └── agent/
│       ├── agent.go                   # Node-side agent (runs on each Nexus node)
│       ├── reporter.go                # Metrics/health reporting to control plane
│       ├── config_receiver.go         # Config distribution receiver
│       └── firmware_agent.go          # Firmware update agent
├── deploy/
│   ├── kubernetes/
│   │   ├── namespace.yaml
│   │   ├── apiserver-deployment.yaml
│   │   ├── controller-deployment.yaml
│   │   ├── ca-deployment.yaml
│   │   ├── metrics-deployment.yaml
│   │   ├── etcd-statefulset.yaml
│   │   └── kustomization.yaml
│   ├── docker/
│   │   ├── Dockerfile.apiserver
│   │   ├── Dockerfile.controller
│   │   ├── Dockerfile.ca
│   │   ├── Dockerfile.metrics
│   │   └── Dockerfile.allinone
│   └── helm/
│       └── nexus-cloud/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── tests/
│   ├── apiserver_test.go
│   ├── controller_test.go
│   ├── ca_test.go
│   ├── tenant_test.go
│   ├── configdist_test.go
│   ├── integration/
│   │   ├── fleet_test.go             # Multi-node fleet management
│   │   ├── firmware_rollout_test.go   # Firmware upgrade orchestration
│   │   └── tenant_isolation_test.go   # Multi-tenant isolation verification
│   └── e2e/
│       └── smoke_test.go             # Full stack E2E in kind cluster
└── docs/
    ├── architecture.md
    ├── api-reference.md
    ├── deployment-guide.md
    └── multi-tenancy.md
```

---

## 4. Functional Requirements

### 4.1 API Server

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-API-001 | REST API with OpenAPI 3.1 specification for all management operations | P0 |
| NCC-API-002 | gRPC API for programmatic access (used by nexctl and node agents) | P0 |
| NCC-API-003 | Authentication: API key, mTLS (using NexTrust MICs), and OIDC/OAuth2 | P0 |
| NCC-API-004 | Role-based access control (RBAC): admin, operator, viewer roles per tenant | P0 |
| NCC-API-005 | Audit logging: all API calls logged with caller identity, action, and result | P0 |
| NCC-API-006 | Rate limiting: configurable per-tenant API rate limits | P1 |
| NCC-API-007 | API versioning: `v1` prefix, backward-compatible evolution policy | P0 |
| NCC-API-008 | Pagination for list endpoints (cursor-based) | P0 |
| NCC-API-009 | Watch/subscribe API for real-time change notifications (WebSocket or SSE) | P1 |

### 4.2 Fleet Controller

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-FC-001 | Reconciliation loop: continuously reconcile desired state (in etcd) with actual state (on nodes) | P0 |
| NCC-FC-002 | Node lifecycle: detect new nodes, mark unhealthy nodes, remove decommissioned nodes | P0 |
| NCC-FC-003 | Route policy enforcement: distribute routing policies to NexRoute instances on all nodes | P0 |
| NCC-FC-004 | Firmware rollout: orchestrate rolling firmware upgrades with configurable batch size, health checks, and automatic rollback on failure | P1 |
| NCC-FC-005 | Leader election: only one controller active at a time in HA deployments (etcd-based lease) | P0 |
| NCC-FC-006 | Garbage collection: clean up expired MICs, stale route entries, orphaned config | P1 |

### 4.3 Configuration Distribution

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-CD-001 | Push-based config distribution: when admin updates config, push to affected nodes within 5 seconds | P0 |
| NCC-CD-002 | Config versioning: every config change produces a new version number. Nodes report their current config version | P0 |
| NCC-CD-003 | Config snapshots: capture full fleet configuration at a point in time for rollback | P1 |
| NCC-CD-004 | Validation: validate config changes before distribution. Reject invalid configs | P0 |
| NCC-CD-005 | Gradual rollout: support canary config distribution (push to N% of nodes, verify, then full rollout) | P2 |

### 4.4 CA Service

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-CA-001 | MIC issuance API: accept CSR, validate, sign, return MIC | P0 |
| NCC-CA-002 | Automated MIC renewal: issue new MICs before expiry with configurable renewal window | P1 |
| NCC-CA-003 | Revocation: revoke MICs and distribute CRL to all nodes | P0 |
| NCC-CA-004 | HSM integration: store CA private keys in HSM (AWS CloudHSM, Azure Key Vault, PKCS#11) | P2 |
| NCC-CA-005 | Transparency log: append-only log of all issued MICs, queryable by serial, node ID, or issuer | P1 |

### 4.5 Multi-Tenancy

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-TN-001 | Tenant isolation: each tenant's nodes, routes, and MICs are invisible to other tenants | P0 |
| NCC-TN-002 | Resource quotas: configurable limits on nodes, connections, bandwidth per tenant | P0 |
| NCC-TN-003 | Usage tracking: track per-tenant traffic volume, connection count, API calls for billing | P1 |
| NCC-TN-004 | Tenant onboarding: API to create new tenant with initial admin user, CA, and default policies | P0 |

### 4.6 Metrics & Observability

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-OB-001 | Collect metrics from all nodes: throughput, latency, connection count, stream count, error rate | P0 |
| NCC-OB-002 | Prometheus exposition endpoint at `/metrics` | P0 |
| NCC-OB-003 | Pre-built Grafana dashboards for fleet-level and node-level monitoring | P1 |
| NCC-OB-004 | Alerting rules: configurable alerts for node down, high latency, firmware mismatch, MIC expiry | P1 |
| NCC-OB-005 | Distributed tracing: propagate trace IDs through NexAPI → NexStream → NexLink, export to OpenTelemetry | P1 |
| NCC-OB-006 | Log aggregation: collect structured logs from all nodes via agent reporting | P1 |

### 4.7 Node Agent

| ID | Requirement | Priority |
|----|-------------|----------|
| NCC-AG-001 | Runs on every Nexus node. Reports health, metrics, and config version to control plane | P0 |
| NCC-AG-002 | Receives config updates from Config Distribution service and applies them to local NexRoute/NexStream | P0 |
| NCC-AG-003 | Receives firmware update instructions and executes local firmware flash | P1 |
| NCC-AG-004 | Auto-registers with control plane on first boot using bootstrap token | P0 |
| NCC-AG-005 | Heartbeat: periodic health report (default: every 10 seconds). Node marked unhealthy after 3 missed heartbeats | P0 |

---

## 5. Data Models

### 5.1 Node

```go
type Node struct {
    NodeID          [16]byte          `json:"node_id"`
    Hostname        string            `json:"hostname"`
    Labels          map[string]string `json:"labels"`
    Addresses       []string          `json:"addresses"`
    Status          NodeStatus        `json:"status"`        // Online, Offline, Degraded, Draining
    NexLinkVersion  string            `json:"nexlink_version"`
    NexRouteVersion string            `json:"nexroute_version"`
    NICModel        string            `json:"nic_model"`
    FirmwareVersion string            `json:"firmware_version"`
    Capabilities    SAD               `json:"capabilities"`
    TrustLevel      uint8             `json:"trust_level"`
    TenantID        string            `json:"tenant_id"`
    ConfigVersion   uint64            `json:"config_version"`
    LastHeartbeat   time.Time         `json:"last_heartbeat"`
    CreatedAt       time.Time         `json:"created_at"`
    UpdatedAt       time.Time         `json:"updated_at"`
}
```

### 5.2 Route Policy

```go
type RoutePolicy struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    TenantID        string            `json:"tenant_id"`
    Priority        int               `json:"priority"`      // Lower = higher priority
    Match           RoutePolicyMatch  `json:"match"`         // Which traffic this policy applies to
    Action          RoutePolicyAction `json:"action"`        // What to do with matching traffic
    Enabled         bool              `json:"enabled"`
    CreatedAt       time.Time         `json:"created_at"`
}

type RoutePolicyMatch struct {
    SourceNodeIDs   [][16]byte        `json:"source_node_ids,omitempty"`
    DestCapabilities *SAD             `json:"dest_capabilities,omitempty"`
    StreamModes     []string          `json:"stream_modes,omitempty"`     // "RO", "RU", "BE", "PR"
    MessageTypes    []uint16          `json:"message_types,omitempty"`
}

type RoutePolicyAction struct {
    Type            string            `json:"type"`          // "allow", "deny", "ratelimit", "redirect"
    RateLimit       *RateLimitConfig  `json:"rate_limit,omitempty"`
    RedirectNodeID  *[16]byte         `json:"redirect_node_id,omitempty"`
    WeightOverride  map[string]float64 `json:"weight_override,omitempty"` // Override scoring weights
}
```

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NCC-NF-001 | API server request latency (CRUD operations) | p99 < 50ms |
| NCC-NF-002 | Config distribution latency (change → node receives) | < 5 seconds for 1000 nodes |
| NCC-NF-003 | Fleet size | Support 10,000+ nodes per control plane instance |
| NCC-NF-004 | API availability | 99.99% (HA deployment with 3+ replicas) |
| NCC-NF-005 | State store size | Support 100K+ objects (nodes + routes + MICs + policies) |
| NCC-NF-006 | Metrics collection interval | 10 seconds (configurable) |
| NCC-NF-007 | All-in-one binary startup time | < 3 seconds |

---

## 7. Build & Deployment

```bash
# Build all binaries
make build

# Build Docker images
make docker-build

# Deploy to Kubernetes (requires existing cluster with etcd)
kubectl apply -k deploy/kubernetes/

# Deploy via Helm
helm install nexus-cloud deploy/helm/nexus-cloud/ -f values.yaml

# Run all-in-one for development
go run ./cmd/nexus-allinone/ --store=memory --port=8080

# Run tests
go test ./... -v

# Run integration tests (requires etcd running)
go test ./tests/integration/ -v -tags=integration

# Run E2E tests (requires kind cluster)
go test ./tests/e2e/ -v -tags=e2e
```

---

## 8. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go standard library | 1.22+ | Core |
| `go.etcd.io/etcd/client/v3` | 3.5+ | etcd client for state store |
| `google.golang.org/grpc` | 1.62+ | gRPC server/client |
| `github.com/gorilla/mux` | 1.8+ | HTTP routing |
| `go.opentelemetry.io/otel` | 1.24+ | Distributed tracing |
| `github.com/prometheus/client_golang` | 1.18+ | Prometheus metrics |
| `go.uber.org/zap` | 1.27+ | Structured logging |
| `github.com/spf13/viper` | 1.18+ | Configuration |
| `k8s.io/client-go` | 0.29+ | Kubernetes client (for leader election, CRDs) |
| NexAPI client SDK | — | Communication with nodes |
| NexTrust Rust FFI | — | MIC signing/verification (via CGo to Rust) |
