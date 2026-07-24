// Package api wires the HTTP handlers for the RunCoach backend.
package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	cfg    *config.Config
	pool   *pgxpool.Pool
	strava *strava.Client
}

// New builds a Server from configuration and a database pool.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	store := strava.NewTokenStore(pool)
	return &Server{
		cfg:    cfg,
		pool:   pool,
		strava: strava.NewClient(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaRedirectURI, store),
	}
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /auth/strava/login", s.handleStravaLogin)
	mux.HandleFunc("GET /auth/strava/callback", s.handleStravaCallback)
	return mux
}
