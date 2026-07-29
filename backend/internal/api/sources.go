package api

import "net/http"

type sourcesResponse struct {
	// Sources registered with the agent, in the order it exposes them.
	Sources []string `json:"sources"`
	// Mock reports that answers are canned rather than real, so the UI can say
	// so instead of presenting fabricated training data as the athlete's own.
	Mock bool `json:"mock"`
}

// handleSources reports which data sources this server actually has. The UI needs
// it because Garmin is config-gated (GARMIN_MCP_COMMAND unset ⇒ Strava only):
// hardcoding "Strava & Garmin" in the header would claim a connection that isn't
// there, which is the same class of dishonesty DESIGN.md §1.5 rules out for
// answer copy.
func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sourcesResponse{Sources: s.sources, Mock: s.cfg.MockMode})
}
