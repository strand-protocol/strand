package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

func (s *Server) handleListMICs(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	mics, err := s.store.MICs().List()
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mics)
}

func (s *Server) handleGetMIC(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mic, err := s.store.MICs().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mic)
}

// issueMICRequest is the JSON body for MIC issuance.
type issueMICRequest struct {
	ID           string   `json:"id"`
	NodeID       string   `json:"node_id"`
	ModelHash    [32]byte `json:"model_hash"`
	Capabilities []string `json:"capabilities"`
	ValidDays    int      `json:"valid_days"`
}

func (s *Server) handleIssueMIC(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var req issueMICRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" || req.NodeID == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "id and node_id are required")
		return
	}
	if err := ValidateID(req.ID); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid id: "+err.Error())
		return
	}
	if err := ValidateID(req.NodeID); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid node_id: "+err.Error())
		return
	}
	validity := 365
	if req.ValidDays > 0 {
		validity = req.ValidDays
	}
	now := time.Now()
	mic := &model.MIC{
		ID:           req.ID,
		NodeID:       req.NodeID,
		ModelHash:    req.ModelHash,
		Capabilities: req.Capabilities,
		ValidFrom:    now,
		ValidUntil:   now.Add(time.Duration(validity) * 24 * time.Hour),
	}
	if err := s.ca.IssueMIC(mic); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, "issue mic: "+err.Error())
		return
	}
	if err := s.store.MICs().Create(mic); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mic)
}

func (s *Server) handleVerifyMIC(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mic, err := s.store.MICs().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	valid, err := s.ca.VerifyMIC(mic)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": valid})
}

func (s *Server) handleRevokeMIC(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.MICs().Revoke(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.ca.RevokeMIC(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) handleDeleteMIC(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := ValidateID(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.MICs().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
