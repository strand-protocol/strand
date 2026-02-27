package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	nodes, err := s.store.Nodes().List()
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	node, err := s.store.Nodes().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var node model.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if node.ID == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}
	if node.Status == "" {
		node.Status = "online"
	}
	node.LastSeen = time.Now()
	if err := s.store.Nodes().Create(&node); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.metrics.IncNode()
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	var node model.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	node.ID = id
	if err := s.store.Nodes().Update(&node); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := s.store.Nodes().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.metrics.DecNode()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	node, err := s.store.Nodes().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	// Optionally decode metrics from body.
	var metrics model.NodeMetrics
	if r.Body != nil && r.ContentLength > 0 {
		json.NewDecoder(r.Body).Decode(&metrics)
		node.Metrics = metrics
	}
	node.LastSeen = time.Now()
	node.Status = "online"
	if err := s.store.Nodes().Update(node); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
