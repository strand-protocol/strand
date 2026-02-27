package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

// Key-space constants. All Nexus keys live under /nexus/v1/ to avoid
// collisions with other etcd tenants.
const (
	keyPrefix = "/nexus/v1"
	leaseTTL  = 30 // seconds — used for node heartbeat keys
)

// key builds a fully-qualified etcd key for the given store type and ID.
func key(storeType, id string) string {
	return fmt.Sprintf("%s/%s/%s", keyPrefix, storeType, id)
}

// prefix builds the etcd key prefix for listing all items of a store type.
func prefix(storeType string) string {
	return fmt.Sprintf("%s/%s/", keyPrefix, storeType)
}

// ---------------------------------------------------------------------------
// EtcdStore
// ---------------------------------------------------------------------------

// EtcdStore is an etcd-backed implementation of the Store interface suitable
// for production multi-node deployments. All operations are serialised through
// etcd's linearisable reads/writes; concurrent accesses from multiple control
// plane replicas are therefore safe.
type EtcdStore struct {
	client   *clientv3.Client
	nodes    *EtcdNodeStore
	routes   *EtcdRouteStore
	mics     *EtcdMICStore
	firmware *EtcdFirmwareStore
}

// NewEtcdStore dials the etcd cluster at endpoints and returns a ready
// EtcdStore. The caller must call Close when finished.
func NewEtcdStore(endpoints []string) (*EtcdStore, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd dial: %w", err)
	}
	return &EtcdStore{
		client:   client,
		nodes:    &EtcdNodeStore{client: client},
		routes:   &EtcdRouteStore{client: client},
		mics:     &EtcdMICStore{client: client},
		firmware: &EtcdFirmwareStore{client: client},
	}, nil
}

// Nodes returns the NodeStore sub-store.
func (s *EtcdStore) Nodes() NodeStore { return s.nodes }

// Routes returns the RouteStore sub-store.
func (s *EtcdStore) Routes() RouteStore { return s.routes }

// MICs returns the MICStore sub-store.
func (s *EtcdStore) MICs() MICStore { return s.mics }

// Firmware returns the FirmwareStore sub-store.
func (s *EtcdStore) Firmware() FirmwareStore { return s.firmware }

// Close releases the underlying etcd client connection.
func (s *EtcdStore) Close() error {
	return s.client.Close()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// etcdPut serialises v as JSON and writes it to the given key.
func etcdPut(ctx context.Context, client *clientv3.Client, k string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if _, err := client.Put(ctx, k, string(data)); err != nil {
		return fmt.Errorf("etcd put %q: %w", k, err)
	}
	return nil
}

// etcdGet retrieves the value at key k and deserialises it into v.
// Returns (false, nil) if the key does not exist.
func etcdGet(ctx context.Context, client *clientv3.Client, k string, v any) (bool, error) {
	resp, err := client.Get(ctx, k)
	if err != nil {
		return false, fmt.Errorf("etcd get %q: %w", k, err)
	}
	if len(resp.Kvs) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(resp.Kvs[0].Value, v); err != nil {
		return false, fmt.Errorf("unmarshal %q: %w", k, err)
	}
	return true, nil
}

// etcdList retrieves all key-value pairs with the given prefix and appends
// decoded objects to *out. T must be a pointer type.
func etcdList[T any](ctx context.Context, client *clientv3.Client, pfx string) ([]T, error) {
	resp, err := client.Get(ctx, pfx, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd list %q: %w", pfx, err)
	}
	out := make([]T, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var item T
		if err := json.Unmarshal(kv.Value, &item); err != nil {
			return nil, fmt.Errorf("unmarshal %q: %w", string(kv.Key), err)
		}
		out = append(out, item)
	}
	return out, nil
}

// etcdCreateIfNotExists atomically writes value v at key k only if k does not
// already exist. Returns ErrAlreadyExists if the key is present.
func etcdCreateIfNotExists(ctx context.Context, client *clientv3.Client, k string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	txn := client.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(k), "=", 0)).
		Then(clientv3.OpPut(k, string(data)))
	resp, err := txn.Commit()
	if err != nil {
		return fmt.Errorf("etcd txn create %q: %w", k, err)
	}
	if !resp.Succeeded {
		return fmt.Errorf("%q already exists", k)
	}
	return nil
}

// etcdDelete removes key k. Returns an error if the key is not present.
func etcdDelete(ctx context.Context, client *clientv3.Client, k string) error {
	resp, err := client.Delete(ctx, k)
	if err != nil {
		return fmt.Errorf("etcd delete %q: %w", k, err)
	}
	if resp.Deleted == 0 {
		return fmt.Errorf("%q not found", k)
	}
	return nil
}

// background returns a context for internal etcd operations (no deadline).
func background() context.Context {
	return context.Background()
}

// ---------------------------------------------------------------------------
// EtcdNodeStore
// ---------------------------------------------------------------------------

// EtcdNodeStore implements NodeStore against etcd.
type EtcdNodeStore struct {
	client *clientv3.Client
}

// List returns all Node records stored in etcd.
func (s *EtcdNodeStore) List() ([]model.Node, error) {
	return etcdList[model.Node](background(), s.client, prefix("nodes"))
}

// Get returns the Node with the given ID, or an error if not found.
func (s *EtcdNodeStore) Get(id string) (*model.Node, error) {
	var n model.Node
	found, err := etcdGet(background(), s.client, key("nodes", id), &n)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("node %q not found", id)
	}
	return &n, nil
}

// Create writes a new Node record. Returns an error if one already exists with
// the same ID.
func (s *EtcdNodeStore) Create(node *model.Node) error {
	if err := etcdCreateIfNotExists(background(), s.client, key("nodes", node.ID), node); err != nil {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	return nil
}

// Update overwrites an existing Node record.
func (s *EtcdNodeStore) Update(node *model.Node) error {
	// Verify the node exists before overwriting so Update semantics are clear.
	_, err := s.Get(node.ID)
	if err != nil {
		return err
	}
	return etcdPut(background(), s.client, key("nodes", node.ID), node)
}

// Delete removes the Node record with the given ID.
func (s *EtcdNodeStore) Delete(id string) error {
	if err := etcdDelete(background(), s.client, key("nodes", id)); err != nil {
		return fmt.Errorf("node %q not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// EtcdRouteStore
// ---------------------------------------------------------------------------

// EtcdRouteStore implements RouteStore against etcd.
type EtcdRouteStore struct {
	client *clientv3.Client
}

// List returns all Route records stored in etcd.
func (s *EtcdRouteStore) List() ([]model.Route, error) {
	return etcdList[model.Route](background(), s.client, prefix("routes"))
}

// Get returns the Route with the given ID, or an error if not found.
func (s *EtcdRouteStore) Get(id string) (*model.Route, error) {
	var r model.Route
	found, err := etcdGet(background(), s.client, key("routes", id), &r)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("route %q not found", id)
	}
	return &r, nil
}

// Create writes a new Route record. Returns an error if one already exists
// with the same ID.
func (s *EtcdRouteStore) Create(route *model.Route) error {
	if err := etcdCreateIfNotExists(background(), s.client, key("routes", route.ID), route); err != nil {
		return fmt.Errorf("route %q already exists", route.ID)
	}
	return nil
}

// Update overwrites an existing Route record.
func (s *EtcdRouteStore) Update(route *model.Route) error {
	_, err := s.Get(route.ID)
	if err != nil {
		return err
	}
	return etcdPut(background(), s.client, key("routes", route.ID), route)
}

// Delete removes the Route record with the given ID.
func (s *EtcdRouteStore) Delete(id string) error {
	if err := etcdDelete(background(), s.client, key("routes", id)); err != nil {
		return fmt.Errorf("route %q not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// EtcdMICStore
// ---------------------------------------------------------------------------

// EtcdMICStore implements MICStore against etcd.
type EtcdMICStore struct {
	client *clientv3.Client
}

// List returns all MIC records stored in etcd.
func (s *EtcdMICStore) List() ([]model.MIC, error) {
	return etcdList[model.MIC](background(), s.client, prefix("mics"))
}

// Get returns the MIC with the given ID, or an error if not found.
func (s *EtcdMICStore) Get(id string) (*model.MIC, error) {
	var m model.MIC
	found, err := etcdGet(background(), s.client, key("mics", id), &m)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("mic %q not found", id)
	}
	return &m, nil
}

// Create writes a new MIC record. Returns an error if one already exists with
// the same ID.
func (s *EtcdMICStore) Create(mic *model.MIC) error {
	if err := etcdCreateIfNotExists(background(), s.client, key("mics", mic.ID), mic); err != nil {
		return fmt.Errorf("mic %q already exists", mic.ID)
	}
	return nil
}

// Update overwrites an existing MIC record.
func (s *EtcdMICStore) Update(mic *model.MIC) error {
	_, err := s.Get(mic.ID)
	if err != nil {
		return err
	}
	return etcdPut(background(), s.client, key("mics", mic.ID), mic)
}

// Delete removes the MIC record with the given ID.
func (s *EtcdMICStore) Delete(id string) error {
	if err := etcdDelete(background(), s.client, key("mics", id)); err != nil {
		return fmt.Errorf("mic %q not found", id)
	}
	return nil
}

// Revoke marks a MIC as revoked without deleting it. The revocation is
// preserved in etcd so that auditors can inspect the revocation history.
func (s *EtcdMICStore) Revoke(id string) error {
	m, err := s.Get(id)
	if err != nil {
		return err
	}
	m.Revoked = true
	return etcdPut(background(), s.client, key("mics", id), m)
}

// ---------------------------------------------------------------------------
// EtcdFirmwareStore
// ---------------------------------------------------------------------------

// EtcdFirmwareStore implements FirmwareStore against etcd.
type EtcdFirmwareStore struct {
	client *clientv3.Client
}

// List returns all FirmwareImage records stored in etcd.
func (s *EtcdFirmwareStore) List() ([]model.FirmwareImage, error) {
	return etcdList[model.FirmwareImage](background(), s.client, prefix("firmware"))
}

// Get returns the FirmwareImage with the given ID, or an error if not found.
func (s *EtcdFirmwareStore) Get(id string) (*model.FirmwareImage, error) {
	var f model.FirmwareImage
	found, err := etcdGet(background(), s.client, key("firmware", id), &f)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("firmware %q not found", id)
	}
	return &f, nil
}

// Create writes a new FirmwareImage record. Returns an error if one already
// exists with the same ID.
func (s *EtcdFirmwareStore) Create(fw *model.FirmwareImage) error {
	if err := etcdCreateIfNotExists(background(), s.client, key("firmware", fw.ID), fw); err != nil {
		return fmt.Errorf("firmware %q already exists", fw.ID)
	}
	return nil
}

// Update overwrites an existing FirmwareImage record.
func (s *EtcdFirmwareStore) Update(fw *model.FirmwareImage) error {
	_, err := s.Get(fw.ID)
	if err != nil {
		return err
	}
	return etcdPut(background(), s.client, key("firmware", fw.ID), fw)
}

// Delete removes the FirmwareImage record with the given ID.
func (s *EtcdFirmwareStore) Delete(id string) error {
	if err := etcdDelete(background(), s.client, key("firmware", id)); err != nil {
		return fmt.Errorf("firmware %q not found", id)
	}
	return nil
}
