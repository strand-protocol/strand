package apiserver

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	s.metrics.IncRequest()
	tenantID := r.URL.Query().Get("tenant_id")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	entries, err := s.store.AuditLog().List(tenantID, limit)
	if err != nil {
		s.metrics.IncError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
