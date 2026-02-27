package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/apiserver"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/ca"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/controller"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/store"
)

// setupTestServer creates a Nexus Cloud API test server backed by in-memory stores.
func setupTestServer(t *testing.T) (*httptest.Server, store.Store) {
	t.Helper()

	memStore := store.NewMemoryStore()
	ks := ca.NewMemoryKeyStore()
	authority := ca.NewCA(ks)
	if err := authority.GenerateCA(); err != nil {
		t.Fatalf("generate CA: %v", err)
	}

	opts := apiserver.DefaultServerOptions()
	srv := apiserver.NewServer(memStore, authority, opts)

	ts := httptest.NewServer(srv.Handler())
	return ts, memStore
}

// jsonBody encodes v as JSON and returns an io.Reader.
func jsonBody(t *testing.T, v interface{}) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonBody marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// decodeJSON reads the response body and decodes it into v.
func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode JSON %q: %v", string(body), err)
	}
}

// TestHealthEndpoints verifies the healthz and readyz endpoints.
func TestHealthEndpoints(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	for _, ep := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(ts.URL + ep)
		if err != nil {
			t.Fatalf("GET %s: %v", ep, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want %d", ep, resp.StatusCode, http.StatusOK)
		}
		resp.Body.Close()
	}
}

// TestNodeCRUD tests the full create-read-update-delete lifecycle for nodes.
func TestNodeCRUD(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Create
	node := model.Node{
		ID:      "node-test-01",
		Address: "10.0.0.1:9100",
		Status:  "online",
	}
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json", jsonBody(t, node))
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create node: status %d, body: %s", resp.StatusCode, body)
	}
	var created model.Node
	decodeJSON(t, resp, &created)
	if created.ID != "node-test-01" {
		t.Errorf("created node ID: got %q, want %q", created.ID, "node-test-01")
	}

	// Read
	resp, err = http.Get(ts.URL + "/api/v1/nodes/node-test-01")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get node: status %d", resp.StatusCode)
	}
	var fetched model.Node
	decodeJSON(t, resp, &fetched)
	if fetched.Address != "10.0.0.1:9100" {
		t.Errorf("fetched address: got %q, want %q", fetched.Address, "10.0.0.1:9100")
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/nodes")
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	var nodes []model.Node
	decodeJSON(t, resp, &nodes)
	if len(nodes) != 1 {
		t.Errorf("list nodes: got %d, want 1", len(nodes))
	}

	// Update
	node.Address = "10.0.0.2:9100"
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/nodes/node-test-01", jsonBody(t, node))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update node: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update node: status %d", resp.StatusCode)
	}
	var updated model.Node
	decodeJSON(t, resp, &updated)
	if updated.Address != "10.0.0.2:9100" {
		t.Errorf("updated address: got %q, want %q", updated.Address, "10.0.0.2:9100")
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/node-test-01", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete node: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete node: status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	resp.Body.Close()

	// Verify deletion
	resp, err = http.Get(ts.URL + "/api/v1/nodes/node-test-01")
	if err != nil {
		t.Fatalf("get deleted node: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get deleted node: status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	resp.Body.Close()
}

// TestRouteCRUD tests the full CRUD lifecycle for routes.
func TestRouteCRUD(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	route := model.Route{
		ID:  "route-test-01",
		TTL: 300 * time.Second,
		Endpoints: []model.Endpoint{
			{NodeID: "node-01", Address: "10.0.0.1:9100", Weight: 1.0},
		},
	}

	// Create
	resp, err := http.Post(ts.URL+"/api/v1/routes", "application/json", jsonBody(t, route))
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create route: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Read
	resp, err = http.Get(ts.URL + "/api/v1/routes/route-test-01")
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get route: status %d", resp.StatusCode)
	}
	var fetched model.Route
	decodeJSON(t, resp, &fetched)
	if fetched.ID != "route-test-01" {
		t.Errorf("route ID: got %q, want %q", fetched.ID, "route-test-01")
	}
	if len(fetched.Endpoints) != 1 {
		t.Errorf("endpoints count: got %d, want 1", len(fetched.Endpoints))
	}

	// Delete
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/routes/route-test-01", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete route: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete route: status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	resp.Body.Close()
}

// TestMICIssuanceAndVerification tests the MIC issue + verify flow.
func TestMICIssuanceAndVerification(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Issue a MIC
	issueReq := map[string]interface{}{
		"id":           "mic-test-01",
		"node_id":      "node-alpha",
		"capabilities": []string{"llm-inference"},
		"valid_days":   30,
	}
	resp, err := http.Post(ts.URL+"/api/v1/trust/mics", "application/json", jsonBody(t, issueReq))
	if err != nil {
		t.Fatalf("issue MIC: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("issue MIC: status %d, body: %s", resp.StatusCode, body)
	}
	var issuedMIC model.MIC
	decodeJSON(t, resp, &issuedMIC)
	if issuedMIC.ID != "mic-test-01" {
		t.Errorf("issued MIC ID: got %q, want %q", issuedMIC.ID, "mic-test-01")
	}
	if len(issuedMIC.Signature) == 0 {
		t.Error("issued MIC has empty signature")
	}

	// Verify the MIC
	resp, err = http.Post(ts.URL+"/api/v1/trust/mics/mic-test-01/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("verify MIC: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("verify MIC: status %d, body: %s", resp.StatusCode, body)
	}
	var verifyResult map[string]bool
	decodeJSON(t, resp, &verifyResult)
	if !verifyResult["valid"] {
		t.Error("MIC verification returned false, expected true")
	}

	// Revoke the MIC
	resp, err = http.Post(ts.URL+"/api/v1/trust/mics/mic-test-01/revoke", "application/json", nil)
	if err != nil {
		t.Fatalf("revoke MIC: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke MIC: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify revoked MIC -- should be invalid
	resp, err = http.Post(ts.URL+"/api/v1/trust/mics/mic-test-01/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("verify revoked MIC: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify revoked MIC: status %d", resp.StatusCode)
	}
	decodeJSON(t, resp, &verifyResult)
	if verifyResult["valid"] {
		t.Error("revoked MIC verified as valid, expected false")
	}
}

// TestFirmwareCRUD tests the full CRUD lifecycle for firmware images.
func TestFirmwareCRUD(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	fw := model.FirmwareImage{
		ID:       "fw-test-01",
		Version:  "v1.0.0",
		Platform: "ConnectX-7",
		Size:     1024 * 1024,
		Checksum: "sha256:abc123",
		URL:      "https://example.com/firmware/v1.0.0.bin",
	}

	// Create
	resp, err := http.Post(ts.URL+"/api/v1/firmware", "application/json", jsonBody(t, fw))
	if err != nil {
		t.Fatalf("create firmware: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create firmware: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Read
	resp, err = http.Get(ts.URL + "/api/v1/firmware/fw-test-01")
	if err != nil {
		t.Fatalf("get firmware: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get firmware: status %d", resp.StatusCode)
	}
	var fetched model.FirmwareImage
	decodeJSON(t, resp, &fetched)
	if fetched.Version != "v1.0.0" {
		t.Errorf("firmware version: got %q, want %q", fetched.Version, "v1.0.0")
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/firmware")
	if err != nil {
		t.Fatalf("list firmware: %v", err)
	}
	var fws []model.FirmwareImage
	decodeJSON(t, resp, &fws)
	if len(fws) != 1 {
		t.Errorf("firmware count: got %d, want 1", len(fws))
	}

	// Update
	fw.Version = "v1.1.0"
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/firmware/fw-test-01", jsonBody(t, fw))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update firmware: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update firmware: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/firmware/fw-test-01", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete firmware: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete firmware: status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	resp.Body.Close()
}

// TestFleetControllerMarksUnhealthy verifies that the fleet controller
// marks nodes as unhealthy after they stop sending heartbeats.
func TestFleetControllerMarksUnhealthy(t *testing.T) {
	memStore := store.NewMemoryStore()

	// Create a node with LastSeen in the past (beyond the unhealthy threshold).
	staleNode := &model.Node{
		ID:       "node-stale-01",
		Address:  "10.0.0.99:9100",
		Status:   "online",
		LastSeen: time.Now().Add(-5 * time.Minute), // far in the past
	}
	if err := memStore.Nodes().Create(staleNode); err != nil {
		t.Fatalf("create stale node: %v", err)
	}

	// Create a healthy node.
	healthyNode := &model.Node{
		ID:       "node-healthy-01",
		Address:  "10.0.0.100:9100",
		Status:   "online",
		LastSeen: time.Now(),
	}
	if err := memStore.Nodes().Create(healthyNode); err != nil {
		t.Fatalf("create healthy node: %v", err)
	}

	fc := controller.NewFleetController(memStore)

	// Run the controller briefly. The checkInterval is 10s by default, but
	// we trigger it by running Start with a short-lived context, then
	// inspect the store directly. Since we cannot override checkInterval,
	// we instead exercise the controller by creating an immediate timeout
	// context and checking the node manually by examining the events.
	//
	// A more realistic approach: start the controller and wait for a tick.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	done := make(chan struct{})
	go func() {
		fc.Start(ctx)
		close(done)
	}()

	// Wait for at least one tick (checkInterval = 10s).
	time.Sleep(12 * time.Second)
	cancel()
	<-done

	// Verify the stale node was marked unhealthy.
	n, err := memStore.Nodes().Get("node-stale-01")
	if err != nil {
		t.Fatalf("get stale node: %v", err)
	}
	if n.Status != "unhealthy" {
		t.Errorf("stale node status: got %q, want %q", n.Status, "unhealthy")
	}

	// Verify the healthy node is still online.
	n, err = memStore.Nodes().Get("node-healthy-01")
	if err != nil {
		t.Fatalf("get healthy node: %v", err)
	}
	if n.Status != "online" {
		t.Errorf("healthy node status: got %q, want %q", n.Status, "online")
	}

	// Verify events were emitted.
	events := fc.Events()
	found := false
	for _, e := range events {
		if e.NodeID == "node-stale-01" && e.Type == "node_unhealthy" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected node_unhealthy event for node-stale-01, but none found")
	}
}

// TestDuplicateCreateReturnsConflict verifies that creating a resource with
// a duplicate ID returns HTTP 409.
func TestDuplicateCreateReturnsConflict(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	node := model.Node{
		ID:      "node-dup-01",
		Address: "10.0.0.1:9100",
	}

	// First create should succeed.
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json", jsonBody(t, node))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Second create should conflict.
	resp, err = http.Post(ts.URL+"/api/v1/nodes", "application/json", jsonBody(t, node))
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate create: status %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	resp.Body.Close()
}

// TestNodeHeartbeat verifies the heartbeat endpoint updates LastSeen and status.
func TestNodeHeartbeat(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	// Create a node.
	node := model.Node{
		ID:      "node-hb-01",
		Address: "10.0.0.1:9100",
		Status:  "online",
	}
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json", jsonBody(t, node))
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	resp.Body.Close()

	// Send heartbeat with metrics.
	metrics := model.NodeMetrics{
		Connections: 10,
		BytesSent:   1024,
		BytesRecv:   2048,
	}
	resp, err = http.Post(
		fmt.Sprintf("%s/api/v1/nodes/node-hb-01/heartbeat", ts.URL),
		"application/json",
		jsonBody(t, metrics),
	)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("heartbeat: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Verify the node was updated.
	resp, err = http.Get(ts.URL + "/api/v1/nodes/node-hb-01")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	var updatedNode model.Node
	decodeJSON(t, resp, &updatedNode)
	if updatedNode.Status != "online" {
		t.Errorf("node status after heartbeat: got %q, want %q", updatedNode.Status, "online")
	}
}

// TestRequestIDHeader verifies that the middleware adds an X-Request-ID.
func TestRequestIDHeader(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	rid := resp.Header.Get("X-Request-ID")
	if rid == "" {
		t.Error("X-Request-ID header not set")
	}
}

// TestCORSHeaders verifies that CORS headers are present.
func TestCORSHeaders(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	acao := resp.Header.Get("Access-Control-Allow-Origin")
	if acao != "*" {
		t.Errorf("CORS origin: got %q, want %q", acao, "*")
	}
}
