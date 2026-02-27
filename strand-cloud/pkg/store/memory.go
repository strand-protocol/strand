package store

import (
	"fmt"
	"sync"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

// MemoryStore is an in-memory implementation of Store backed by maps and a
// read/write mutex. Suitable for development, testing, and single-node
// deployments.
type MemoryStore struct {
	nodes    *memoryNodeStore
	routes   *memoryRouteStore
	mics     *memoryMICStore
	firmware *memoryFirmwareStore
	tenants  *memoryTenantStore
	clusters *memoryClusterStore
	auditLog *memoryAuditLogStore
}

// NewMemoryStore returns a fully initialised MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:    &memoryNodeStore{data: make(map[string]model.Node)},
		routes:   &memoryRouteStore{data: make(map[string]model.Route)},
		mics:     &memoryMICStore{data: make(map[string]model.MIC)},
		firmware: &memoryFirmwareStore{data: make(map[string]model.FirmwareImage)},
		tenants:  &memoryTenantStore{data: make(map[string]model.Tenant), slugIdx: make(map[string]string)},
		clusters: &memoryClusterStore{data: make(map[string]model.Cluster)},
		auditLog: &memoryAuditLogStore{},
	}
}

func (m *MemoryStore) Nodes() NodeStore         { return m.nodes }
func (m *MemoryStore) Routes() RouteStore       { return m.routes }
func (m *MemoryStore) MICs() MICStore           { return m.mics }
func (m *MemoryStore) Firmware() FirmwareStore   { return m.firmware }
func (m *MemoryStore) Tenants() TenantStore     { return m.tenants }
func (m *MemoryStore) Clusters() ClusterStore   { return m.clusters }
func (m *MemoryStore) AuditLog() AuditLogStore  { return m.auditLog }

// ---------------------------------------------------------------------------
// Node store
// ---------------------------------------------------------------------------

type memoryNodeStore struct {
	mu   sync.RWMutex
	data map[string]model.Node
}

func (s *memoryNodeStore) List() ([]model.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Node, 0, len(s.data))
	for _, n := range s.data {
		out = append(out, n)
	}
	return out, nil
}

func (s *memoryNodeStore) Get(id string) (*model.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("node %q not found", id)
	}
	return &n, nil
}

func (s *memoryNodeStore) Create(node *model.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	s.data[node.ID] = *node
	return nil
}

func (s *memoryNodeStore) Update(node *model.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[node.ID]; !exists {
		return fmt.Errorf("node %q not found", node.ID)
	}
	s.data[node.ID] = *node
	return nil
}

func (s *memoryNodeStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("node %q not found", id)
	}
	delete(s.data, id)
	return nil
}

// ---------------------------------------------------------------------------
// Route store
// ---------------------------------------------------------------------------

type memoryRouteStore struct {
	mu   sync.RWMutex
	data map[string]model.Route
}

func (s *memoryRouteStore) List() ([]model.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Route, 0, len(s.data))
	for _, r := range s.data {
		out = append(out, r)
	}
	return out, nil
}

func (s *memoryRouteStore) Get(id string) (*model.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("route %q not found", id)
	}
	return &r, nil
}

func (s *memoryRouteStore) Create(route *model.Route) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[route.ID]; exists {
		return fmt.Errorf("route %q already exists", route.ID)
	}
	s.data[route.ID] = *route
	return nil
}

func (s *memoryRouteStore) Update(route *model.Route) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[route.ID]; !exists {
		return fmt.Errorf("route %q not found", route.ID)
	}
	s.data[route.ID] = *route
	return nil
}

func (s *memoryRouteStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("route %q not found", id)
	}
	delete(s.data, id)
	return nil
}

// ---------------------------------------------------------------------------
// MIC store
// ---------------------------------------------------------------------------

type memoryMICStore struct {
	mu   sync.RWMutex
	data map[string]model.MIC
}

func (s *memoryMICStore) List() ([]model.MIC, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.MIC, 0, len(s.data))
	for _, m := range s.data {
		out = append(out, m)
	}
	return out, nil
}

func (s *memoryMICStore) Get(id string) (*model.MIC, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("mic %q not found", id)
	}
	return &m, nil
}

func (s *memoryMICStore) Create(mic *model.MIC) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[mic.ID]; exists {
		return fmt.Errorf("mic %q already exists", mic.ID)
	}
	s.data[mic.ID] = *mic
	return nil
}

func (s *memoryMICStore) Update(mic *model.MIC) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[mic.ID]; !exists {
		return fmt.Errorf("mic %q not found", mic.ID)
	}
	s.data[mic.ID] = *mic
	return nil
}

func (s *memoryMICStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("mic %q not found", id)
	}
	delete(s.data, id)
	return nil
}

func (s *memoryMICStore) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.data[id]
	if !ok {
		return fmt.Errorf("mic %q not found", id)
	}
	m.Revoked = true
	s.data[id] = m
	return nil
}

// ---------------------------------------------------------------------------
// Firmware store
// ---------------------------------------------------------------------------

type memoryFirmwareStore struct {
	mu   sync.RWMutex
	data map[string]model.FirmwareImage
}

func (s *memoryFirmwareStore) List() ([]model.FirmwareImage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.FirmwareImage, 0, len(s.data))
	for _, f := range s.data {
		out = append(out, f)
	}
	return out, nil
}

func (s *memoryFirmwareStore) Get(id string) (*model.FirmwareImage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("firmware %q not found", id)
	}
	return &f, nil
}

func (s *memoryFirmwareStore) Create(fw *model.FirmwareImage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[fw.ID]; exists {
		return fmt.Errorf("firmware %q already exists", fw.ID)
	}
	s.data[fw.ID] = *fw
	return nil
}

func (s *memoryFirmwareStore) Update(fw *model.FirmwareImage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[fw.ID]; !exists {
		return fmt.Errorf("firmware %q not found", fw.ID)
	}
	s.data[fw.ID] = *fw
	return nil
}

func (s *memoryFirmwareStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("firmware %q not found", id)
	}
	delete(s.data, id)
	return nil
}

// ---------------------------------------------------------------------------
// Tenant store
// ---------------------------------------------------------------------------

type memoryTenantStore struct {
	mu      sync.RWMutex
	data    map[string]model.Tenant
	slugIdx map[string]string // slug -> id
}

func (s *memoryTenantStore) List() ([]model.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Tenant, 0, len(s.data))
	for _, t := range s.data {
		out = append(out, t)
	}
	return out, nil
}

func (s *memoryTenantStore) Get(id string) (*model.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found", id)
	}
	return &t, nil
}

func (s *memoryTenantStore) GetBySlug(slug string) (*model.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.slugIdx[slug]
	if !ok {
		return nil, fmt.Errorf("tenant with slug %q not found", slug)
	}
	t := s.data[id]
	return &t, nil
}

func (s *memoryTenantStore) Create(tenant *model.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[tenant.ID]; exists {
		return fmt.Errorf("tenant %q already exists", tenant.ID)
	}
	if _, exists := s.slugIdx[tenant.Slug]; exists {
		return fmt.Errorf("tenant slug %q already taken", tenant.Slug)
	}
	s.data[tenant.ID] = *tenant
	s.slugIdx[tenant.Slug] = tenant.ID
	return nil
}

func (s *memoryTenantStore) Update(tenant *model.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.data[tenant.ID]
	if !exists {
		return fmt.Errorf("tenant %q not found", tenant.ID)
	}
	if old.Slug != tenant.Slug {
		delete(s.slugIdx, old.Slug)
		s.slugIdx[tenant.Slug] = tenant.ID
	}
	s.data[tenant.ID] = *tenant
	return nil
}

func (s *memoryTenantStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, exists := s.data[id]
	if !exists {
		return fmt.Errorf("tenant %q not found", id)
	}
	delete(s.slugIdx, t.Slug)
	delete(s.data, id)
	return nil
}

// ---------------------------------------------------------------------------
// Cluster store
// ---------------------------------------------------------------------------

type memoryClusterStore struct {
	mu   sync.RWMutex
	data map[string]model.Cluster
}

func (s *memoryClusterStore) List(tenantID string) ([]model.Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Cluster, 0)
	for _, c := range s.data {
		if tenantID == "" || c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *memoryClusterStore) Get(id string) (*model.Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", id)
	}
	return &c, nil
}

func (s *memoryClusterStore) Create(cluster *model.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[cluster.ID]; exists {
		return fmt.Errorf("cluster %q already exists", cluster.ID)
	}
	s.data[cluster.ID] = *cluster
	return nil
}

func (s *memoryClusterStore) Update(cluster *model.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[cluster.ID]; !exists {
		return fmt.Errorf("cluster %q not found", cluster.ID)
	}
	s.data[cluster.ID] = *cluster
	return nil
}

func (s *memoryClusterStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("cluster %q not found", id)
	}
	delete(s.data, id)
	return nil
}

// ---------------------------------------------------------------------------
// Audit log store
// ---------------------------------------------------------------------------

type memoryAuditLogStore struct {
	mu      sync.RWMutex
	entries []model.AuditEntry
}

func (s *memoryAuditLogStore) Append(entry *model.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, *entry)
	return nil
}

func (s *memoryAuditLogStore) List(tenantID string, limit int) ([]model.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.AuditEntry, 0)
	// Walk backwards for most-recent-first ordering.
	for i := len(s.entries) - 1; i >= 0; i-- {
		if tenantID == "" || s.entries[i].TenantID == tenantID {
			out = append(out, s.entries[i])
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
