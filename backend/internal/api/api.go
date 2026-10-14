// Package api wires the HTTP handlers for the RunCoach backend.
package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/crypto"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
	"github.com/lmagsino/run-coach-ai/backend/internal/web"
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	cfg       *config.Config
	pool      *pgxpool.Pool // nil in mock mode or without DATABASE_URL
	store     store.Store
	auth      auth.Provider
	garmin    garmin.Source
	sealer    *crypto.Sealer // nil without RUNCOACH_TOKEN_KEY
	anthropic anthropic.Client
	// inflight holds the runner ids with a reply in progress: one at a time
	// per runner, so a double tap cannot spawn two Garmin processes and bill
	// two answers.
	inflight sync.Map
	logins   *loginLimiter
}

// New builds a Server from configuration and an optional database pool. Without
// a pool the store is in-memory, which is what mock mode and a database-less
// dev server get: chats then live until the process exits.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	s := &Server{
		cfg:       cfg,
		pool:      pool,
		anthropic: anthropic.NewClient(option.WithAPIKey(cfg.AnthropicAPIKey)),
		logins:    newLoginLimiter(5, time.Minute),
	}
	if pool != nil {
		s.store = store.NewPG(pool)
	} else {
		s.store = store.NewMemory()
		if !cfg.MockMode {
			log.Println("no database: accounts and chats are kept in memory until restart")
		}
	}
	s.auth = auth.EmailPassword{Runners: s.store}
	if cfg.TokenKey != "" {
		sealer, err := crypto.New(cfg.TokenKey)
		if err != nil {
			log.Fatalf("RUNCOACH_TOKEN_KEY: %v", err)
		}
		s.sealer = sealer
	}
	s.garmin = &garmin.LocalSpawn{Command: cfg.GarminMCPCommand, Store: s.store, Sealer: s.sealer}

	if cfg.MockMode {
		log.Printf("RUNCOACH_MOCK is set: canned answers, no login, no Garmin or Claude call")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := s.bootstrapOwner(ctx); err != nil {
			log.Printf("owner bootstrap: %v", err)
		}
		if err := s.importGarminBundle(ctx); err != nil {
			log.Printf("garmin bundle import: %v", err)
		}
	}
	return s
}

// bootstrapOwner applies RUNCOACH_OWNER_EMAIL and RUNCOACH_OWNER_PASSWORD_HASH
// to the owner row where the row has none yet. It never overwrites: once the
// owner has changed either on the account page, the env value is history.
func (s *Server) bootstrapOwner(ctx context.Context) error {
	owner, err := s.store.Owner(ctx)
	if err != nil {
		return err
	}
	changed := false
	if owner.Email == "" && s.cfg.OwnerEmail != "" {
		owner.Email, changed = s.cfg.OwnerEmail, true
	}
	if owner.PasswordHash == "" && s.cfg.OwnerPasswordHash != "" {
		owner.PasswordHash, changed = s.cfg.OwnerPasswordHash, true
	}
	if changed {
		if err := s.store.UpdateRunner(ctx, owner); err != nil {
			return err
		}
	}
	if owner.PasswordHash == "" {
		log.Printf("the owner has no password: set RUNCOACH_OWNER_PASSWORD_HASH or run cmd/set-password")
	}
	return nil
}

// Routes returns the HTTP handler with all routes registered. The API lives
// under /api; everything else is the embedded UI (ADR 0007).
func (s *Server) Routes() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/healthz", s.handleHealthz)
	api.HandleFunc("POST /api/login", s.handleLogin)
	api.HandleFunc("POST /api/signup", s.handleSignup)
	api.HandleFunc("POST /api/logout", s.handleLogout)
	api.HandleFunc("GET /api/me", s.handleMe)
	api.HandleFunc("POST /api/garmin/tokens", s.handleGarminTokens)
	api.HandleFunc("DELETE /api/garmin/tokens", s.handleGarminTokensDelete)
	api.HandleFunc("GET /api/chats", s.handleChatsList)
	api.HandleFunc("POST /api/chats", s.handleChatsCreate)
	api.HandleFunc("PATCH /api/chats/{id}", s.handleChatRename)
	api.HandleFunc("DELETE /api/chats/{id}", s.handleChatDelete)
	api.HandleFunc("GET /api/chats/{id}/messages", s.handleChatMessages)
	api.HandleFunc("POST /api/chats/{id}/stream", s.handleChatStream)
	// Stateless question for curl and the live pass: no chat, no history.
	api.HandleFunc("POST /api/chat", s.handleChat)

	root := http.NewServeMux()
	root.Handle("/api/", s.withSession(api))
	root.HandleFunc("GET /healthz", s.handleHealthz) // the host's health check path
	root.Handle("/", web.Handler())
	return withCORS(s.cfg.AllowedOrigin, root)
}
