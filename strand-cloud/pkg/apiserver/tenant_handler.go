package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	tenants, err := s.store.Tenants().List()
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tenants)
}

func (s *Server) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenant, err := s.store.Tenants().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var tenant model.Tenant
	if err := json.NewDecoder(r.Body).Decode(&tenant); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if tenant.Name == "" || tenant.Slug == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}
	if tenant.Plan == "" {
		tenant.Plan = "free"
	}
	if tenant.Status == "" {
		tenant.Status = "active"
	}
	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	applyPlanDefaults(&tenant)
	if err := s.store.Tenants().Create(&tenant); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tenant)
}

func (s *Server) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var tenant model.Tenant
	if err := json.NewDecoder(r.Body).Decode(&tenant); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	tenant.ID = id
	tenant.UpdatedAt = time.Now()
	if err := s.store.Tenants().Update(&tenant); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (s *Server) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Tenants().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// applyPlanDefaults sets resource limits based on the tenant's plan.
func applyPlanDefaults(t *model.Tenant) {
	switch t.Plan {
	case "starter":
		t.MaxClusters = 1
		t.MaxNodes = 10
		t.MaxMICsMonth = 1000
		t.TrafficGBIncl = 10
	case "pro":
		t.MaxClusters = 3
		t.MaxNodes = 150
		t.MaxMICsMonth = 10000
		t.TrafficGBIncl = 100
	case "enterprise":
		t.MaxClusters = 999
		t.MaxNodes = 9999
		t.MaxMICsMonth = 999999
		t.TrafficGBIncl = 10000
	default: // free
		t.MaxClusters = 1
		t.MaxNodes = 3
		t.MaxMICsMonth = 100
		t.TrafficGBIncl = 1
	}
}
