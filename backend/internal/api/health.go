package api

import (
	"context"
	"net/http"
	"time"
)

// handleHealthz reports service health: the database, and the owner's Garmin
// state read from its row (no Garmin call, no secrets). The host's health
// check hits it every few seconds, so it has to stay cheap.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"status": "ok", "db": "off"}
	if s.pool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.pool.Ping(ctx); err != nil {
			http.Error(w, `{"status":"unhealthy","db":"down"}`, http.StatusServiceUnavailable)
			return
		}
		out["db"] = "up"
	}
	if owner, err := s.store.Owner(r.Context()); err == nil {
		if st, err := s.garminStateFor(r, owner.ID); err == nil {
			out["garmin"] = st.Status
			if st.ExpiresEstAt != nil {
				out["tokens_expire_est"] = st.ExpiresEstAt
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}
