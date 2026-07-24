// Package api wires the HTTP handlers for the RunCoach backend.
package api

import (
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
	anthropic anthropic.Client
}

// New builds a Server from configuration and a database pool.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	store := strava.NewTokenStore(pool)
	mcpFields := strings.Fields(cfg.StravaMCPCommand)
	return &Server{
		cfg:       cfg,
		pool:      pool,
		strava:    strava.NewClient(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaRedirectURI, store),
		stravaMCP: mcpclient.NewStrava(mcpFields[0], mcpFields[1:]...),
		anthropic: anthropic.NewClient(), // reads ANTHROPIC_API_KEY from env
	}
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /auth/strava/login", s.handleStravaLogin)
	mux.HandleFunc("GET /auth/strava/callback", s.handleStravaCallback)
	mux.HandleFunc("POST /chat", s.handleChat)
	return mux
}
