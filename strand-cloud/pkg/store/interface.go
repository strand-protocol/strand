// Package store defines the persistence interfaces for the Strand Cloud control
// plane. Implementations include an in-memory store (for dev/testing) and an
// etcd-backed store (for production).
package store

import "github.com/strand-protocol/strand/strand-cloud/pkg/model"

// NodeStore provides CRUD operations for Node records.
type NodeStore interface {
	List() ([]model.Node, error)
	Get(id string) (*model.Node, error)
	Create(node *model.Node) error
	Update(node *model.Node) error
	Delete(id string) error
}

// RouteStore provides CRUD operations for Route records.
type RouteStore interface {
	List() ([]model.Route, error)
	Get(id string) (*model.Route, error)
	Create(route *model.Route) error
	Update(route *model.Route) error
	Delete(id string) error
}

// MICStore provides CRUD operations for MIC records, plus a Revoke shorthand.
type MICStore interface {
	List() ([]model.MIC, error)
	Get(id string) (*model.MIC, error)
	Create(mic *model.MIC) error
	Update(mic *model.MIC) error
	Delete(id string) error
	Revoke(id string) error
}

// FirmwareStore provides CRUD operations for FirmwareImage records.
type FirmwareStore interface {
	List() ([]model.FirmwareImage, error)
	Get(id string) (*model.FirmwareImage, error)
	Create(fw *model.FirmwareImage) error
	Update(fw *model.FirmwareImage) error
	Delete(id string) error
}

// TenantStore provides CRUD operations for Tenant records.
type TenantStore interface {
	List() ([]model.Tenant, error)
	Get(id string) (*model.Tenant, error)
	GetBySlug(slug string) (*model.Tenant, error)
	Create(tenant *model.Tenant) error
	Update(tenant *model.Tenant) error
	Delete(id string) error
}

// ClusterStore provides CRUD operations for Cluster records.
type ClusterStore interface {
	List(tenantID string) ([]model.Cluster, error)
	Get(id string) (*model.Cluster, error)
	Create(cluster *model.Cluster) error
	Update(cluster *model.Cluster) error
	Delete(id string) error
}

// AuditLogStore provides append and query operations for audit log entries.
type AuditLogStore interface {
	Append(entry *model.AuditEntry) error
	List(tenantID string, limit int) ([]model.AuditEntry, error)
}

// Store aggregates all sub-stores into a single handle.
type Store interface {
	Nodes() NodeStore
	Routes() RouteStore
	MICs() MICStore
	Firmware() FirmwareStore
	Tenants() TenantStore
	Clusters() ClusterStore
	AuditLog() AuditLogStore
}
