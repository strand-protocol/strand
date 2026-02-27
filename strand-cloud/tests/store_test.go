package tests

import (
	"testing"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
	"github.com/strand-protocol/strand/strand-cloud/pkg/store"
)

// ---------------------------------------------------------------------------
// Node store
// ---------------------------------------------------------------------------

func TestNodeStore_CRUD(t *testing.T) {
	s := store.NewMemoryStore()
	ns := s.Nodes()

	// Create
	node := &model.Node{
		ID:              "node-1",
		Address:         "10.0.0.1",
		Status:          "online",
		FirmwareVersion: "1.0.0",
		LastSeen:        time.Now(),
	}
	if err := ns.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	// Duplicate create
	if err := ns.Create(node); err == nil {
		t.Fatal("expected error on duplicate create")
	}

	// Get
	got, err := ns.Get("node-1")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if got.ID != "node-1" || got.Address != "10.0.0.1" {
		t.Fatalf("unexpected node: %+v", got)
	}

	// Get non-existent
	if _, err := ns.Get("node-999"); err == nil {
		t.Fatal("expected error on get non-existent")
	}

	// List
	list, err := ns.List()
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 node, got %d", len(list))
	}

	// Update
	node.Status = "unhealthy"
	if err := ns.Update(node); err != nil {
		t.Fatalf("update node: %v", err)
	}
	got, _ = ns.Get("node-1")
	if got.Status != "unhealthy" {
		t.Fatalf("expected status unhealthy, got %s", got.Status)
	}

	// Update non-existent
	fake := &model.Node{ID: "node-999"}
	if err := ns.Update(fake); err == nil {
		t.Fatal("expected error on update non-existent")
	}

	// Delete
	if err := ns.Delete("node-1"); err != nil {
		t.Fatalf("delete node: %v", err)
	}
	list, _ = ns.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 nodes after delete, got %d", len(list))
	}

	// Delete non-existent
	if err := ns.Delete("node-1"); err == nil {
		t.Fatal("expected error on delete non-existent")
	}
}

// ---------------------------------------------------------------------------
// Route store
// ---------------------------------------------------------------------------

func TestRouteStore_CRUD(t *testing.T) {
	s := store.NewMemoryStore()
	rs := s.Routes()

	route := &model.Route{
		ID:  "route-1",
		SAD: []byte{0x01, 0x02},
		Endpoints: []model.Endpoint{
			{NodeID: "n1", Address: "10.0.0.1", Weight: 1.0},
		},
		TTL:       5 * time.Minute,
		CreatedAt: time.Now(),
	}
	if err := rs.Create(route); err != nil {
		t.Fatalf("create route: %v", err)
	}
	if err := rs.Create(route); err == nil {
		t.Fatal("expected error on duplicate create")
	}

	got, err := rs.Get("route-1")
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if got.ID != "route-1" {
		t.Fatalf("unexpected route: %+v", got)
	}

	list, _ := rs.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 route, got %d", len(list))
	}

	route.TTL = 10 * time.Minute
	if err := rs.Update(route); err != nil {
		t.Fatalf("update route: %v", err)
	}

	if err := rs.Delete("route-1"); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	list, _ = rs.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// MIC store
// ---------------------------------------------------------------------------

func TestMICStore_CRUD(t *testing.T) {
	s := store.NewMemoryStore()
	ms := s.MICs()

	mic := &model.MIC{
		ID:           "mic-1",
		NodeID:       "node-1",
		Capabilities: []string{"route", "stream"},
		ValidFrom:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour),
	}
	if err := ms.Create(mic); err != nil {
		t.Fatalf("create mic: %v", err)
	}
	if err := ms.Create(mic); err == nil {
		t.Fatal("expected error on duplicate create")
	}

	got, err := ms.Get("mic-1")
	if err != nil {
		t.Fatalf("get mic: %v", err)
	}
	if got.ID != "mic-1" {
		t.Fatalf("unexpected mic: %+v", got)
	}

	list, _ := ms.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 mic, got %d", len(list))
	}

	// Revoke
	if err := ms.Revoke("mic-1"); err != nil {
		t.Fatalf("revoke mic: %v", err)
	}
	got, _ = ms.Get("mic-1")
	if !got.Revoked {
		t.Fatal("expected mic to be revoked")
	}

	// Revoke non-existent
	if err := ms.Revoke("mic-999"); err == nil {
		t.Fatal("expected error on revoke non-existent")
	}

	// Delete
	if err := ms.Delete("mic-1"); err != nil {
		t.Fatalf("delete mic: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Firmware store
// ---------------------------------------------------------------------------

func TestFirmwareStore_CRUD(t *testing.T) {
	s := store.NewMemoryStore()
	fs := s.Firmware()

	fw := &model.FirmwareImage{
		ID:        "fw-1",
		Version:   "2.0.0",
		Platform:  "arm64",
		Size:      1024000,
		Checksum:  "sha256:abcdef",
		URL:       "https://firmware.strand.dev/v2.0.0/arm64.bin",
		CreatedAt: time.Now(),
	}
	if err := fs.Create(fw); err != nil {
		t.Fatalf("create firmware: %v", err)
	}
	if err := fs.Create(fw); err == nil {
		t.Fatal("expected error on duplicate create")
	}

	got, err := fs.Get("fw-1")
	if err != nil {
		t.Fatalf("get firmware: %v", err)
	}
	if got.Version != "2.0.0" {
		t.Fatalf("unexpected firmware version: %s", got.Version)
	}

	list, _ := fs.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 firmware, got %d", len(list))
	}

	fw.Version = "2.0.1"
	if err := fs.Update(fw); err != nil {
		t.Fatalf("update firmware: %v", err)
	}

	if err := fs.Delete("fw-1"); err != nil {
		t.Fatalf("delete firmware: %v", err)
	}
	list, _ = fs.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 firmware, got %d", len(list))
	}
}
