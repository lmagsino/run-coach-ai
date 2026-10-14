// Package config loads runtime configuration from environment variables,
// with a .env file loaded automatically in local development.
package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	Port string // HTTP listen port

	// DatabaseURL is optional until chats are persisted (Phase 7, ADR 0008).
	// Empty means the server runs without a database and /healthz reports it.
	DatabaseURL string

	// GarminMCPCommand launches the upstream garmin-mcp server, which the
	// backend connects to as an MCP client over stdio for each reply. The
	// hosted image has the binary on PATH; local dev points it at the Docker
	// image. The runner's token bundle is handed to it as GARMINTOKENS.
	GarminMCPCommand string

	// OwnerPasswordHash is the bcrypt hash the owner logs in with (ADR 0002).
	// Empty means nobody can log in outside mock mode.
	OwnerPasswordHash string
	// TokenKey seals the stored Garmin token bundle (ADR 0003). Base64 of 32
	// random bytes; `go run ./cmd/token-key` prints one.
	TokenKey string

	// GarminBundle and GarminBundleFile let the server import the token bundle
	// on start instead of waiting for a paste: the inline value on a host, or
	// the file deploy/garmin-mcp/auth.sh writes on the laptop. The paste page
	// stays as the fallback. Empty or missing means nothing is imported.
	GarminBundle     string
	GarminBundleFile string

	AnthropicAPIKey string
	AnthropicModel  string // Claude model id for the agent loop
	AnthropicEffort string // effort level sent with every request (ADR 0006)

	// AllowedOrigin is a browser origin permitted by CORS. Empty, the default,
	// disables CORS: the UI is served from the same origin in production and
	// the Vite dev server proxies /api (ADR 0007).
	AllowedOrigin string

	// MockMode serves canned answers instead of calling Claude or Garmin, so
	// the frontend can be built and demoed with no credentials at all. It must
	// never be enabled anywhere the answers could be mistaken for real.
	MockMode bool
}

// Load reads configuration from the environment. It first attempts to load a
// .env file (ignored if absent, e.g. in production), then reads variables.
func Load() (*Config, error) {
	// Best-effort: a missing .env is fine (vars may come from the real env).
	_ = godotenv.Load()

	cfg := &Config{
		Port:              getenv("PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		GarminMCPCommand:  getenv("GARMIN_MCP_COMMAND", "garmin-mcp"),
		OwnerPasswordHash: os.Getenv("RUNCOACH_OWNER_PASSWORD_HASH"),
		TokenKey:          os.Getenv("RUNCOACH_TOKEN_KEY"),
		GarminBundle:      os.Getenv("RUNCOACH_GARMIN_BUNDLE"),
		GarminBundleFile:  getenv("RUNCOACH_GARMIN_BUNDLE_FILE", ".garmin_bundle.b64"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:    getenv("ANTHROPIC_MODEL", "claude-opus-5-5"),
		AnthropicEffort:   getenv("ANTHROPIC_EFFORT", "medium"),
		AllowedOrigin:     os.Getenv("ALLOWED_ORIGIN"),
		MockMode:          truthy(os.Getenv("RUNCOACH_MOCK")),
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
