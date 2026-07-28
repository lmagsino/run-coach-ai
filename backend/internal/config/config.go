// Package config loads runtime configuration from environment variables,
// with a .env file loaded automatically in local development.
package config

import (
	"fmt"
	"os"

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
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
