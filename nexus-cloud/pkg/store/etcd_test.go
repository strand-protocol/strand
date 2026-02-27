package store

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

// TestEtcdStore is an integration test that exercises all four sub-stores of
// EtcdStore. It requires a running etcd cluster:
//
//	NEXUS_TEST_ETCD=http://localhost:2379 go test ./pkg/store/...
func TestEtcdStore(t *testing.T) {
	addr := os.Getenv("NEXUS_TEST_ETCD")
	if addr == "" {
		t.Skip("set NEXUS_TEST_ETCD=http://localhost:2379 to run etcd integration tests")
	}

	endpoints := strings.Split(addr, ",")
	s, err := NewEtcdStore(endpoints)
	if err != nil {
		t.Fatalf("NewEtcdStore: %v", err)
	}
	defer s.Close()

	t.Run("Nodes", func(t *testing.T) { testNodeStore(t, s.Nodes()) })
	t.Run("Routes", func(t *testing.T) { testRouteStore(t, s.Routes()) })
	t.Run("MICs", func(t *testing.T) { testMICStore(t, s.MICs()) })
	t.Run("Firmware", func(t *testing.T) { testFirmwareStore(t, s.Firmware()) })
}

// ---------------------------------------------------------------------------
// NodeStore tests
// ---------------------------------------------------------------------------

func testNodeStore(t *testing.T, ns NodeStore) {
	t.Helper()

	id := "etcd-test-node-" + uniqueSuffix()
	node := &model.Node{
		ID:              id,
		Address:         "10.0.0.1:6477",
		Status:          "active",
		FirmwareVersion: "v1.0.0",
		LastSeen:        time.Now().UTC(),
	}

	// Create
	if err := ns.Create(node); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Duplicate create must fail
	if err := ns.Create(node); err == nil {
		t.Error("Create duplicate: expected error, got nil")
	}

	// Get
	got, err := ns.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != id {
		t.Errorf("Get ID: %q != %q", got.ID, id)
	}

	// Update
	node.Status = "draining"
	if err := ns.Update(node); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := ns.Get(id)
	if updated.Status != "draining" {
		t.Errorf("after Update status=%q, want %q", updated.Status, "draining")
	}

	// List — must contain our node
	all, err := ns.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, n := range all {
		if n.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("List: node not found")
	}

	// Delete
	if err := ns.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ns.Get(id); err == nil {
		t.Error("Get after Delete: expected error, got nil")
	}
	// Double delete must fail
	if err := ns.Delete(id); err == nil {
		t.Error("Delete non-existent: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// RouteStore tests
// ---------------------------------------------------------------------------

func testRouteStore(t *testing.T, rs RouteStore) {
	t.Helper()

	id := "etcd-test-route-" + uniqueSuffix()
	route := &model.Route{
		ID:  id,
		TTL: 60 * time.Second,
		Endpoints: []model.Endpoint{
			{NodeID: "node-a", Address: "10.0.0.2:6477", Weight: 1.0},
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := rs.Create(route); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := rs.Create(route); err == nil {
		t.Error("Create duplicate: expected error")
	}

	got, err := rs.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != id {
		t.Errorf("Get ID mismatch")
	}

	route.TTL = 120 * time.Second
	if err := rs.Update(route); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := rs.Get(id)
	if updated.TTL != 120*time.Second {
		t.Errorf("TTL after update: %v, want %v", updated.TTL, 120*time.Second)
	}

	all, _ := rs.List()
	found := false
	for _, r := range all {
		if r.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("List: route not found")
	}

	if err := rs.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := rs.Get(id); err == nil {
		t.Error("Get after Delete: expected error")
	}
}

// ---------------------------------------------------------------------------
// MICStore tests
// ---------------------------------------------------------------------------

func testMICStore(t *testing.T, ms MICStore) {
	t.Helper()

	id := "etcd-test-mic-" + uniqueSuffix()
	mic := &model.MIC{
		ID:           id,
		NodeID:       "node-x",
		Capabilities: []string{"inference"},
		ValidFrom:    time.Now().UTC(),
		ValidUntil:   time.Now().Add(24 * time.Hour).UTC(),
	}

	if err := ms.Create(mic); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ms.Create(mic); err == nil {
		t.Error("Create duplicate: expected error")
	}

	got, err := ms.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.NodeID != "node-x" {
		t.Errorf("NodeID: %q != %q", got.NodeID, "node-x")
	}

	// Revoke
	if err := ms.Revoke(id); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, _ := ms.Get(id)
	if !revoked.Revoked {
		t.Error("expected Revoked=true after Revoke")
	}

	// Update
	mic.Revoked = false
	if err := ms.Update(mic); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := ms.Get(id)
	if updated.Revoked {
		t.Error("expected Revoked=false after Update")
	}

	all, _ := ms.List()
	found := false
	for _, m := range all {
		if m.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("List: MIC not found")
	}

	if err := ms.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ms.Get(id); err == nil {
		t.Error("Get after Delete: expected error")
	}
}

// ---------------------------------------------------------------------------
// FirmwareStore tests
// ---------------------------------------------------------------------------

func testFirmwareStore(t *testing.T, fs FirmwareStore) {
	t.Helper()

	id := "etcd-test-fw-" + uniqueSuffix()
	fw := &model.FirmwareImage{
		ID:        id,
		Version:   "v2.3.1",
		Platform:  "linux-amd64",
		Size:      1024 * 1024,
		Checksum:  "sha256:deadbeef",
		URL:       "https://releases.nexus.example/v2.3.1/nexus-linux-amd64.bin",
		CreatedAt: time.Now().UTC(),
	}

	if err := fs.Create(fw); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Create(fw); err == nil {
		t.Error("Create duplicate: expected error")
	}

	got, err := fs.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version != "v2.3.1" {
		t.Errorf("Version: %q != %q", got.Version, "v2.3.1")
	}

	fw.Version = "v2.3.2"
	if err := fs.Update(fw); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := fs.Get(id)
	if updated.Version != "v2.3.2" {
		t.Errorf("Version after update: %q, want %q", updated.Version, "v2.3.2")
	}

	all, _ := fs.List()
	found := false
	for _, f := range all {
		if f.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("List: firmware not found")
	}

	if err := fs.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := fs.Get(id); err == nil {
		t.Error("Get after Delete: expected error")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// uniqueSuffix returns a short timestamp-based suffix to isolate test keys.
func uniqueSuffix() string {
	return time.Now().Format("20060102T150405.000")
}
