package api

import (
	"context"
	"net/http"
	"time"
)

// handleHealthz reports service health, including database connectivity.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Mock mode runs without a database at all, so there is nothing to ping. Say
	// "skipped" rather than "up": claiming a healthy database that isn't there
	// would make this endpoint useless for spotting a genuinely missing one.
	if s.pool == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "skipped (mock mode)"})
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
