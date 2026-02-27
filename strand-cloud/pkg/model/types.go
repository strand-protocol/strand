// Package model defines the core data types for the Strand Cloud control plane.
package model

import "time"

// Node represents a Strand-enabled network node registered with the control plane.
type Node struct {
	ID              string      `json:"id"`
	Address         string      `json:"address"`
	SAD             []byte      `json:"sad,omitempty"`
	Status          string      `json:"status"`
	LastSeen        time.Time   `json:"last_seen"`
	FirmwareVersion string      `json:"firmware_version"`
	Metrics         NodeMetrics `json:"metrics"`
}

// NodeMetrics contains operational metrics for a node.
type NodeMetrics struct {
	Connections int           `json:"connections"`
	BytesSent   uint64        `json:"bytes_sent"`
	BytesRecv   uint64        `json:"bytes_recv"`
	AvgLatency  time.Duration `json:"avg_latency"`
}

// Route represents a network route entry managed by the control plane.
type Route struct {
	ID        string        `json:"id"`
	SAD       []byte        `json:"sad,omitempty"`
	Endpoints []Endpoint    `json:"endpoints"`
	TTL       time.Duration `json:"ttl"`
	CreatedAt time.Time     `json:"created_at"`
}

// Endpoint represents a single endpoint target within a route.
type Endpoint struct {
	NodeID  string  `json:"node_id"`
	Address string  `json:"address"`
	Weight  float64 `json:"weight"`
}

// MIC is a Machine Identity Certificate used for node authentication within the
// Strand trust framework (StrandTrust). It binds a node to a set of claims via
// an Ed25519 signature issued by the control plane CA.
type MIC struct {
	ID           string    `json:"id"`
	NodeID       string    `json:"node_id"`
	ModelHash    [32]byte  `json:"model_hash"`
	Capabilities []string  `json:"capabilities"`
	Signature    []byte    `json:"signature,omitempty"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidUntil   time.Time `json:"valid_until"`
	Revoked      bool      `json:"revoked"`
}

// FirmwareImage represents a firmware binary available for deployment to nodes.
type FirmwareImage struct {
	ID        string    `json:"id"`
	Version   string    `json:"version"`
	Platform  string    `json:"platform"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

// Tenant represents a multi-tenant organisation in the platform.
type Tenant struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Slug             string            `json:"slug"`
	Plan             string            `json:"plan"`
	Status           string            `json:"status"`
	MaxClusters      int               `json:"max_clusters"`
	MaxNodes         int               `json:"max_nodes"`
	MaxMICsMonth     int               `json:"max_mics_month"`
	TrafficGBIncl    float64           `json:"traffic_gb_included"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// Cluster represents a managed cluster owned by a tenant.
type Cluster struct {
	ID                   string            `json:"id"`
	TenantID             string            `json:"tenant_id"`
	Name                 string            `json:"name"`
	Region               string            `json:"region"`
	Status               string            `json:"status"`
	ControlPlaneEndpoint string            `json:"control_plane_endpoint,omitempty"`
	NodeCount            int               `json:"node_count"`
	Config               map[string]string `json:"config,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// AuditEntry represents a single audit log event.
type AuditEntry struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	ActorID      string            `json:"actor_id"`
	ActorType    string            `json:"actor_type"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}
