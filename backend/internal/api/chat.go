package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/agent"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
)

type chatRequest struct {
	Question string `json:"question"`
}

// handleChat answers a training question by running the Claude tool-calling loop
// over the Strava MCP session.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a non-empty \"question\""})
		return
	}

	if s.cfg.AnthropicAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ANTHROPIC_API_KEY is not set"})
		return
	}

	// Longer timeout: the tool-calling loop makes several model + MCP round-trips.
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	tok, err := s.strava.ValidToken(ctx)
	if errors.Is(err, strava.ErrNoToken) {
		writeJSON(w, http.StatusPreconditionRequired, map[string]string{
			"error": "no Strava connection yet — authorize first at /auth/strava/login",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	session, err := s.stravaMCP.Connect(ctx, tok.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer session.Close()

	ag := agent.New(s.anthropic, anthropic.Model(s.cfg.AnthropicModel), session)
	answer, err := ag.Answer(ctx, req.Question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"answer": answer})
}
