package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	routes, err := s.store.Routes().List()
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	route, err := s.store.Routes().Get(id)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	var route model.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if route.ID == "" {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "route id is required")
		return
	}
	route.CreatedAt = time.Now()
	if err := s.store.Routes().Create(&route); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.metrics.IncRoute()
	writeJSON(w, http.StatusCreated, route)
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	var route model.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	route.ID = id
	if err := s.store.Routes().Update(&route); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	id := r.PathValue("id")
	if err := s.store.Routes().Delete(id); err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.metrics.DecRoute()
	w.WriteHeader(http.StatusNoContent)
}
