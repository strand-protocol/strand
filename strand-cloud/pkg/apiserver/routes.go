package apiserver

import (
	"encoding/json"
	"net/http"
)

// registerRoutes wires all API v1 routes into the server mux.
func (s *Server) registerRoutes() {
	// Health probes
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Metrics endpoint
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)

	// Nodes
	s.mux.HandleFunc("GET /api/v1/nodes", s.handleListNodes)
	s.mux.HandleFunc("POST /api/v1/nodes", s.handleCreateNode)
	s.mux.HandleFunc("GET /api/v1/nodes/{id}", s.handleGetNode)
	s.mux.HandleFunc("PUT /api/v1/nodes/{id}", s.handleUpdateNode)
	s.mux.HandleFunc("DELETE /api/v1/nodes/{id}", s.handleDeleteNode)
	s.mux.HandleFunc("POST /api/v1/nodes/{id}/heartbeat", s.handleNodeHeartbeat)

	// Routes
	s.mux.HandleFunc("GET /api/v1/routes", s.handleListRoutes)
	s.mux.HandleFunc("POST /api/v1/routes", s.handleCreateRoute)
	s.mux.HandleFunc("GET /api/v1/routes/{id}", s.handleGetRoute)
	s.mux.HandleFunc("PUT /api/v1/routes/{id}", s.handleUpdateRoute)
	s.mux.HandleFunc("DELETE /api/v1/routes/{id}", s.handleDeleteRoute)

	// Trust / MICs
	s.mux.HandleFunc("GET /api/v1/trust/mics", s.handleListMICs)
	s.mux.HandleFunc("POST /api/v1/trust/mics", s.handleIssueMIC)
	s.mux.HandleFunc("GET /api/v1/trust/mics/{id}", s.handleGetMIC)
	s.mux.HandleFunc("POST /api/v1/trust/mics/{id}/verify", s.handleVerifyMIC)
	s.mux.HandleFunc("POST /api/v1/trust/mics/{id}/revoke", s.handleRevokeMIC)
	s.mux.HandleFunc("DELETE /api/v1/trust/mics/{id}", s.handleDeleteMIC)

	// Firmware
	s.mux.HandleFunc("GET /api/v1/firmware", s.handleListFirmware)
	s.mux.HandleFunc("POST /api/v1/firmware", s.handleCreateFirmware)
	s.mux.HandleFunc("GET /api/v1/firmware/{id}", s.handleGetFirmware)
	s.mux.HandleFunc("PUT /api/v1/firmware/{id}", s.handleUpdateFirmware)
	s.mux.HandleFunc("DELETE /api/v1/firmware/{id}", s.handleDeleteFirmware)
}

// handleHealthz is a liveness probe.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReadyz is a readiness probe.
func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// handleMetrics exposes internal counters.
func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.metrics.GetMetrics())
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
