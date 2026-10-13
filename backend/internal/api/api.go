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
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	cfg       *config.Config
	pool      *pgxpool.Pool     // nil in mock mode or when DATABASE_URL is unset
	garminMCP *mcpclient.Garmin // nil when GARMIN_MCP_COMMAND is unset
	anthropic anthropic.Client
	// sources names the registered tool sources, in the order the agent exposes
	// them. Served to the UI so it can describe what it is actually connected to.
	sources []string
}

// New builds a Server from configuration and an optional database pool.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	s := &Server{
		cfg:       cfg,
		pool:      pool,
		anthropic: anthropic.NewClient(), // reads ANTHROPIC_API_KEY from env
	}

	// Garmin is the only source. Until the local image is built and
	// authenticated, leaving GARMIN_MCP_COMMAND unset keeps the server starting
	// (for mock mode and /healthz) while every live request says what is missing.
	if cfg.GarminMCPCommand != "" {
		garminCmd, garminArgs := splitCommand(cfg.GarminMCPCommand)
		s.garminMCP = mcpclient.NewGarmin(garminCmd, garminArgs...)
		s.sources = []string{mcpclient.SourceGarmin}
	}
	log.Printf("agent tool sources: %s", strings.Join(s.sources, ", "))
	if cfg.MockMode {
		// Mock scenarios narrate Garmin steps whether or not the command is
		// configured; the same response carries mock:true, which the UI shows.
		s.sources = []string{mcpclient.SourceGarmin}
		log.Printf("RUNCOACH_MOCK is set: /chat and /chat/stream serve canned answers; " +
			"no Garmin or Claude call is made")
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
	mux.HandleFunc("GET /sources", s.handleSources)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("POST /chat/stream", s.handleChatStream)
	// The frontend is served from a different origin in development (Vite on
	// :5173), so every route needs CORS, including the OPTIONS preflight.
	return withCORS(s.cfg.AllowedOrigin, mux)
}
