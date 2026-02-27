// Package model defines the core data types for the Nexus Cloud control plane.
package model

import "time"

// Node represents a Nexus-enabled network node registered with the control plane.
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
// Nexus trust framework (NexTrust). It binds a node to a set of claims via
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
