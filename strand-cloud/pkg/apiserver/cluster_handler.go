package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	tenantID := r.URL.Query().Get("tenant_id")
	clusters, err := s.store.Clusters().List(tenantID)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, clusters)
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cluster, err := s.store.Clusters().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cluster)
}

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var cluster model.Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if cluster.Name == "" || cluster.TenantID == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "name and tenant_id are required")
		return
	}
	if cluster.Status == "" {
		cluster.Status = "provisioning"
	}
	now := time.Now()
	cluster.CreatedAt = now
	cluster.UpdatedAt = now
	if err := s.store.Clusters().Create(&cluster); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cluster)
}

func (s *Server) handleUpdateCluster(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var cluster model.Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cluster.ID = id
	cluster.UpdatedAt = time.Now()
	if err := s.store.Clusters().Update(&cluster); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cluster)
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Clusters().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
