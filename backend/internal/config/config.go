// Package config loads runtime configuration from environment variables,
// with a .env file loaded automatically in local development.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	Port        string // HTTP listen port
	DatabaseURL string // Postgres connection string (pgx-compatible)

	StravaClientID     string
	StravaClientSecret string
	StravaRedirectURI  string

	// StravaMCPCommand is the command (with args) that launches the self-hosted
	// strava-mcp server, which the backend connects to as an MCP client over stdio.
	StravaMCPCommand string

	// GarminMCPCommand is the equivalent for the self-hosted garmin_mcp
	// container. Empty disables Garmin entirely, leaving Strava as the only
	// tool source — which is how it runs until the container is built.
	GarminMCPCommand string

	AnthropicAPIKey string
	AnthropicModel  string // Claude model id for the agent loop

	// AllowedOrigin is the browser origin permitted by CORS — the Vite dev
	// server in local development. Empty disables the CORS headers entirely.
	AllowedOrigin string

	// MockMode serves canned answers instead of calling Claude or any MCP
	// source, so the frontend can be built and demoed before credentials
	// exist (Phases 2–4 are deliberately credential-free; Phase 5 turns this
	// off). It bypasses the Strava-token and Anthropic-key requirements, so it
	// must never be enabled anywhere the answers could be mistaken for real.
	MockMode bool
}

// defaultDatabaseURL points at the local dev Postgres created in Phase 2.
const defaultDatabaseURL = "postgres://postgres@localhost:5432/runcoach_dev?sslmode=disable"

// Load reads configuration from the environment. It first attempts to load a
// .env file (ignored if absent, e.g. in production), then reads variables.
// Only DatabaseURL and Port have defaults; the Strava/Anthropic credentials
// are optional here so the server and migrations can run before they exist.
func Load() (*Config, error) {
	// Best-effort: a missing .env is fine (vars may come from the real env).
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        getenv("DATABASE_URL", defaultDatabaseURL),
		StravaClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		StravaClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		StravaRedirectURI:  getenv("STRAVA_REDIRECT_URI", "http://localhost:8080/auth/strava/callback"),
		StravaMCPCommand:   getenv("STRAVA_MCP_COMMAND", "go run ./cmd/strava-mcp"),
		GarminMCPCommand:   os.Getenv("GARMIN_MCP_COMMAND"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:     getenv("ANTHROPIC_MODEL", "claude-sonnet-5"),
		AllowedOrigin:      getenv("ALLOWED_ORIGIN", "http://localhost:5173"),
		MockMode:           truthy(os.Getenv("RUNCOACH_MOCK")),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

// truthy accepts the spellings people actually type in a .env file. Anything
// else — including "0", "false" and "" — is off, so a mistyped value fails
// closed rather than silently serving fake data.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
