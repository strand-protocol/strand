package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

func (s *Server) handleListFirmware(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	fws, err := s.store.Firmware().List()
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fws)
}

func (s *Server) handleGetFirmware(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	fw, err := s.store.Firmware().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fw)
}

func (s *Server) handleCreateFirmware(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var fw model.FirmwareImage
	if err := json.NewDecoder(r.Body).Decode(&fw); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if fw.ID == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "firmware id is required")
		return
	}
	fw.CreatedAt = time.Now()
	if err := s.store.Firmware().Create(&fw); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, fw)
}

func (s *Server) handleUpdateFirmware(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	var fw model.FirmwareImage
	if err := json.NewDecoder(r.Body).Decode(&fw); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	fw.ID = id
	if err := s.store.Firmware().Update(&fw); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fw)
}

func (s *Server) handleDeleteFirmware(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := s.store.Firmware().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
