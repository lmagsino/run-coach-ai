package api

import (
	"context"
	"net/http"
	"time"
)

// handleHealthz reports service health, including database connectivity.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Mock mode and a server without DATABASE_URL have no database at all, so
	// there is nothing to ping. Say "off" rather than "up": claiming a healthy
	// database that isn't there would hide a genuinely missing one.
	if s.pool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "off"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		http.Error(w, `{"status":"unhealthy","db":"down"}`, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
}
