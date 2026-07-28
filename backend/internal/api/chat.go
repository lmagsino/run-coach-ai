package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/agent"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
)

type chatRequest struct {
	Question string `json:"question"`
}

type chatResponse struct {
	Answer string `json:"answer"`
	// Sources actually queried while answering, in call order. Empty means
	// Claude answered without reaching for any tool.
	Sources []string `json:"sources"`
}

// handleChat answers a training question by running the Claude tool-calling loop
// over every configured MCP source (Strava, and Garmin when it is configured).
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

	stravaSession, err := s.stravaMCP.Connect(ctx, tok.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer stravaSession.Close()
	sources := []agent.Source{{Name: mcpclient.SourceStrava, Session: stravaSession}}

	// Garmin joins the tool set only when configured. A configured-but-broken
	// container is a real error, not something to quietly answer around:
	// dropping it would leave the agent silently unable to see recovery data.
	if s.garminMCP != nil {
		garminSession, err := s.garminMCP.Connect(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		defer garminSession.Close()
		sources = append(sources, agent.Source{Name: mcpclient.SourceGarmin, Session: garminSession})
	}

	ag := agent.New(s.anthropic, anthropic.Model(s.cfg.AnthropicModel), sources...)
	result, err := ag.Answer(ctx, req.Question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{Answer: result.Answer, Sources: result.Sources()})
}
