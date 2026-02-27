package api

import "time"

// NodeInfo represents a Nexus network node.
type NodeInfo struct {
	ID           string    `json:"id" yaml:"id"`
	Address      string    `json:"address" yaml:"address"`
	Status       string    `json:"status" yaml:"status"`
	LastSeen     time.Time `json:"last_seen" yaml:"last_seen"`
	Firmware     string    `json:"firmware" yaml:"firmware"`
	Capabilities []string  `json:"capabilities" yaml:"capabilities"`
}

// RouteInfo represents a SAD routing entry.
type RouteInfo struct {
	SAD       string   `json:"sad" yaml:"sad"`
	Endpoints []string `json:"endpoints" yaml:"endpoints"`
	Weight    int      `json:"weight" yaml:"weight"`
	TTL       int      `json:"ttl" yaml:"ttl"`
}

// MICInfo represents a Model Identity Certificate.
type MICInfo struct {
	NodeID     string    `json:"node_id" yaml:"node_id"`
	ModelHash  string    `json:"model_hash" yaml:"model_hash"`
	ValidUntil time.Time `json:"valid_until" yaml:"valid_until"`
	Issuer     string    `json:"issuer" yaml:"issuer"`
	Status     string    `json:"status" yaml:"status"`
}

// FirmwareInfo represents a firmware image.
type FirmwareInfo struct {
	ID       string `json:"id" yaml:"id"`
	Version  string `json:"version" yaml:"version"`
	Platform string `json:"platform" yaml:"platform"`
	Size     int64  `json:"size" yaml:"size"`
	Checksum string `json:"checksum" yaml:"checksum"`
}

// DiagnoseResult represents the result of a diagnostic operation.
type DiagnoseResult struct {
	Target  string  `json:"target" yaml:"target"`
	Latency float64 `json:"latency" yaml:"latency"`
	Hops    int     `json:"hops" yaml:"hops"`
	Status  string  `json:"status" yaml:"status"`
}

// MetricsData represents metrics for a node.
type MetricsData struct {
	NodeID      string  `json:"node_id" yaml:"node_id"`
	Connections int     `json:"connections" yaml:"connections"`
	BytesSent   int64   `json:"bytes_sent" yaml:"bytes_sent"`
	BytesRecv   int64   `json:"bytes_recv" yaml:"bytes_recv"`
	Latency     float64 `json:"latency" yaml:"latency"`
}
