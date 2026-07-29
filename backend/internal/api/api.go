// Package api wires the HTTP handlers for the RunCoach backend.
package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	cfg       *config.Config
	pool      *pgxpool.Pool
	strava    *strava.Client
	stravaMCP *mcpclient.Strava
	garminMCP *mcpclient.Garmin // nil when GARMIN_MCP_COMMAND is unset
	anthropic anthropic.Client
	// sources names the registered tool sources, in the order the agent exposes
	// them. Served to the UI so it can describe what it is actually connected to.
	sources []string
}

// New builds a Server from configuration and a database pool.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	store := strava.NewTokenStore(pool)
	stravaCmd, stravaArgs := splitCommand(cfg.StravaMCPCommand)
	s := &Server{
		cfg:       cfg,
		pool:      pool,
		strava:    strava.NewClient(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaRedirectURI, store),
		stravaMCP: mcpclient.NewStrava(stravaCmd, stravaArgs...),
		anthropic: anthropic.NewClient(), // reads ANTHROPIC_API_KEY from env
	}

	// Garmin is optional: until the container is built and authenticated
	// (Phase 5), leaving GARMIN_MCP_COMMAND unset keeps Strava as the only
	// tool source rather than failing every request on a missing image.
	sources := []string{mcpclient.SourceStrava}
	if cfg.GarminMCPCommand != "" {
		garminCmd, garminArgs := splitCommand(cfg.GarminMCPCommand)
		s.garminMCP = mcpclient.NewGarmin(garminCmd, garminArgs...)
		sources = append(sources, mcpclient.SourceGarmin)
	}
	s.sources = sources
	log.Printf("agent tool sources: %s", strings.Join(sources, ", "))
	if cfg.MockMode {
		log.Printf("RUNCOACH_MOCK is set: /chat and /chat/stream serve canned answers; " +
			"no Strava, Garmin or Claude call is made")
	}

	return s
}

// splitCommand splits a configured MCP launch command into program and args.
// Whitespace-separated only — no shell quoting, so paths must not contain spaces.
func splitCommand(cmd string) (string, []string) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /auth/strava/login", s.handleStravaLogin)
	mux.HandleFunc("GET /auth/strava/callback", s.handleStravaCallback)
	mux.HandleFunc("GET /sources", s.handleSources)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("POST /chat/stream", s.handleChatStream)
	// The frontend is served from a different origin in development (Vite on
	// :5173), so every route needs CORS, including the OPTIONS preflight.
	return withCORS(s.cfg.AllowedOrigin, mux)
}
