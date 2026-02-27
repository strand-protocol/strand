package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/apiserver"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/ca"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/store"
)

// newTestServer creates an httptest.Server backed by in-memory stores and a CA.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := store.NewMemoryStore()
	ks := ca.NewMemoryKeyStore()
	authority := ca.NewCA(ks)
	if err := authority.GenerateCA(); err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	opts := apiserver.DefaultServerOptions()
	srv := apiserver.NewServer(s, authority, opts)
	return httptest.NewServer(srv.Handler())
}

// newAuthTestServer creates a server with API keys and RBAC configured.
func newAuthTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := store.NewMemoryStore()
	ks := ca.NewMemoryKeyStore()
	authority := ca.NewCA(ks)
	if err := authority.GenerateCA(); err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	opts := apiserver.DefaultServerOptions()
	opts.APIKeys = map[string]apiserver.APIKeyInfo{
		"viewer-token":   {Description: "viewer", Role: apiserver.RoleViewer},
		"operator-token": {Description: "operator", Role: apiserver.RoleOperator},
		"admin-token":    {Description: "admin", Role: apiserver.RoleAdmin},
	}
	srv := apiserver.NewServer(s, authority, opts)
	return httptest.NewServer(srv.Handler())
}

// ---------------------------------------------------------------------------
// Health probes
// ---------------------------------------------------------------------------

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestReadyz(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// CORS preflight
// ---------------------------------------------------------------------------

// TestCORSPreflight verifies that:
//   - Preflight OPTIONS returns 204
//   - The default (no AllowedOrigins configured) does NOT set a wildcard
//     Access-Control-Allow-Origin header, enforcing deny-by-default CORS.
func TestCORSPreflight(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/nodes", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	// No wildcard origin must be present when AllowedOrigins is unconfigured.
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v == "*" {
		t.Fatal("wildcard Access-Control-Allow-Origin must not be set in production mode")
	}
	// CORS method/header advertisements should still be present.
	if v := resp.Header.Get("Access-Control-Allow-Methods"); v == "" {
		t.Fatal("Access-Control-Allow-Methods header should be set")
	}
}

// ---------------------------------------------------------------------------
// Node CRUD
// ---------------------------------------------------------------------------

func TestNodeCRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create
	body, _ := json.Marshal(model.Node{ID: "n1", Address: "10.0.0.1"})
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// List
	resp, _ = http.Get(ts.URL + "/api/v1/nodes")
	var nodes []model.Node
	json.NewDecoder(resp.Body).Decode(&nodes)
	resp.Body.Close()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	// Get
	resp, _ = http.Get(ts.URL + "/api/v1/nodes/n1")
	var node model.Node
	json.NewDecoder(resp.Body).Decode(&node)
	resp.Body.Close()
	if node.ID != "n1" {
		t.Fatalf("expected n1, got %s", node.ID)
	}

	// Update
	body, _ = json.Marshal(model.Node{ID: "n1", Address: "10.0.0.2", Status: "degraded"})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/nodes/n1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/n1", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Get deleted -> 404
	resp, _ = http.Get(ts.URL + "/api/v1/nodes/n1")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Route CRUD
// ---------------------------------------------------------------------------

func TestRouteCRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(model.Route{ID: "r1", Endpoints: []model.Endpoint{{NodeID: "n1", Address: "10.0.0.1", Weight: 1.0}}})
	resp, _ := http.Post(ts.URL+"/api/v1/routes", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	resp, _ = http.Get(ts.URL + "/api/v1/routes/r1")
	var route model.Route
	json.NewDecoder(resp.Body).Decode(&route)
	resp.Body.Close()
	if route.ID != "r1" {
		t.Fatalf("expected r1, got %s", route.ID)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/routes/r1", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// MIC issue + verify + revoke via API
// ---------------------------------------------------------------------------

func TestMICIssueVerifyRevoke(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Issue
	issueReq := map[string]interface{}{
		"id":           "mic-api-1",
		"node_id":      "node-api-1",
		"capabilities": []string{"route"},
		"valid_days":   30,
	}
	body, _ := json.Marshal(issueReq)
	resp, err := http.Post(ts.URL+"/api/v1/trust/mics", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("issue mic: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Verify
	resp, _ = http.Post(ts.URL+"/api/v1/trust/mics/mic-api-1/verify", "application/json", nil)
	var verifyResp map[string]bool
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	resp.Body.Close()
	if !verifyResp["valid"] {
		t.Fatal("expected mic to be valid")
	}

	// Revoke
	resp, _ = http.Post(ts.URL+"/api/v1/trust/mics/mic-api-1/revoke", "application/json", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify after revoke
	resp, _ = http.Post(ts.URL+"/api/v1/trust/mics/mic-api-1/verify", "application/json", nil)
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	resp.Body.Close()
	if verifyResp["valid"] {
		t.Fatal("expected mic to be invalid after revoke")
	}
}

// ---------------------------------------------------------------------------
// Firmware CRUD
// ---------------------------------------------------------------------------

func TestFirmwareCRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	fw := model.FirmwareImage{
		ID:       "fw-api-1",
		Version:  "3.0.0",
		Platform: "arm64",
		Size:     2048000,
		Checksum: "sha256:deadbeef",
		URL:      "https://firmware.nexus.dev/v3.0.0/arm64.bin",
	}
	body, _ := json.Marshal(fw)
	resp, _ := http.Post(ts.URL+"/api/v1/firmware", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	resp, _ = http.Get(ts.URL + "/api/v1/firmware/fw-api-1")
	var got model.FirmwareImage
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if got.Version != "3.0.0" {
		t.Fatalf("expected version 3.0.0, got %s", got.Version)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/firmware/fw-api-1", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Metrics endpoint
// ---------------------------------------------------------------------------

func TestMetrics(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Make a couple of requests first.
	http.Get(ts.URL + "/api/v1/nodes")

	resp, _ := http.Get(ts.URL + "/metrics")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var m map[string]int64
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if m["request_count"] < 1 {
		t.Fatalf("expected request_count >= 1, got %d", m["request_count"])
	}
}

// ---------------------------------------------------------------------------
// Heartbeat endpoint
// ---------------------------------------------------------------------------

func TestNodeHeartbeat(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create node first.
	body, _ := json.Marshal(model.Node{ID: "hb-node", Address: "10.0.0.5"})
	resp, _ := http.Post(ts.URL+"/api/v1/nodes", "application/json", bytes.NewReader(body))
	resp.Body.Close()

	// Heartbeat
	resp, err := http.Post(ts.URL+"/api/v1/nodes/hb-node/heartbeat", "application/json", http.NoBody)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Heartbeat for non-existent node
	resp, _ = http.Post(ts.URL+"/api/v1/nodes/missing-node/heartbeat", "application/json", http.NoBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// X-Request-ID middleware
// ---------------------------------------------------------------------------

func TestRequestIDMiddleware(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/healthz")
	resp.Body.Close()
	if id := resp.Header.Get("X-Request-ID"); id == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

// ---------------------------------------------------------------------------
// Validation errors
// ---------------------------------------------------------------------------

func TestCreateNodeValidation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Missing ID
	body, _ := json.Marshal(model.Node{Address: "10.0.0.1"})
	resp, _ := http.Post(ts.URL+"/api/v1/nodes", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	// Invalid JSON
	resp, _ = http.Post(ts.URL+"/api/v1/nodes", "application/json", bytes.NewReader([]byte("not json")))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// RBAC — role-based access control (P1 security)
// ---------------------------------------------------------------------------

func TestRBACUnauthenticated(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	// No auth header → 401
	resp, err := http.Get(ts.URL + "/api/v1/nodes")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRBACHealthzNoAuth(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	// Health endpoints must not require auth.
	resp, _ := http.Get(ts.URL + "/healthz")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /healthz without auth, got %d", resp.StatusCode)
	}
}

func TestRBACViewerCanGet(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("viewer GET /nodes: expected 200, got %d", resp.StatusCode)
	}
}

func TestRBACViewerCannotDelete(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	// First create a node as admin.
	body, _ := json.Marshal(model.Node{ID: "rbac-test-node", Address: "10.0.1.1"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// Viewer tries DELETE → 403.
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/rbac-test-node", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer DELETE: expected 403, got %d", resp.StatusCode)
	}
}

func TestRBACOperatorCanPost(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(model.Node{ID: "op-node", Address: "10.0.2.1"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer operator-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("operator POST /nodes: expected 201, got %d", resp.StatusCode)
	}
}

func TestRBACOperatorCannotDelete(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	// Create as admin first.
	body, _ := json.Marshal(model.Node{ID: "op-del-node", Address: "10.0.3.1"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// Operator tries DELETE → 403.
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/op-del-node", nil)
	req.Header.Set("Authorization", "Bearer operator-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("operator DELETE: expected 403, got %d", resp.StatusCode)
	}
}

func TestRBACAdminCanDelete(t *testing.T) {
	ts := newAuthTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(model.Node{ID: "admin-node", Address: "10.0.4.1"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/admin-node", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("admin DELETE: expected 204, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Path parameter injection (P1 security)
// ---------------------------------------------------------------------------

func TestPathInjectionNodeID(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Node ID containing path traversal characters must be rejected with 400.
	resp, err := http.Get(ts.URL + "/api/v1/nodes/../etc/passwd")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 for traversal ID, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Request body size limit (P1 security)
// ---------------------------------------------------------------------------

func TestRequestBodyTooLarge(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Send a 2 MiB body — exceeds 1 MiB MaxBytesReader limit.
	bigBody := bytes.Repeat([]byte("x"), 2<<20)
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json", bytes.NewReader(bigBody))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	// Expect 413 Payload Too Large or 400 Bad Request (MaxBytesReader triggers 400 via json decode).
	if resp.StatusCode != http.StatusRequestEntityTooLarge && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 413 or 400 for oversized body, got %d", resp.StatusCode)
	}
}
