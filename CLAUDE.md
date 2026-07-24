# RunCoach AI — Project Guide (CLAUDE.md)

Context for AI coding sessions. Read this first.

## What this is
A chat-first AI running coach that reasons across **Strava** and **Garmin** data (connected via MCP) to answer questions neither app can answer alone. Chat *is* the product — no dashboards in v1. See `running-agent-feature-spec.md` for the full spec and `DESIGN.md` for the Phase 1 UI direction ("Field Notes").

## Repo layout (monorepo)
```
run-coach-ai/
├── DESIGN.md, design/                 # Phase 1 design (locked)
├── running-agent-feature-spec.md      # Feature spec
├── PROGRESS.md                        # Cross-session task checklist — keep current
├── CLAUDE.md                          # This file
├── backend/                           # Go API server (Phase 2)
└── frontend/                          # Vue 3 app (Phase 4, not yet created)
```

## Working conventions
- **Commit gradually and often.** Make small, frequent commits as work progresses — after each working, self-contained chunk (a passing build, a new endpoint, a migration, a doc update). Never batch a big session into one commit. Many small commits > few large ones. When in doubt, commit.
- Each commit should build/pass on its own where practical, and do one thing.
- Conventional-ish commit subjects (`feat:`, `fix:`, `chore:`, `docs:`), imperative mood.
- Never commit secrets. `.env` is gitignored; keep `backend/.env.example` in sync when adding a new env var.
- Update `PROGRESS.md` (check items off) as tasks complete.
- Git identity in this repo is set locally to the personal account (`leo.magsino819@gmail.com`).

## Backend (Go)

**Location:** `backend/` — `go.mod` lives here, run all Go commands from this dir.
**Module path:** `github.com/lmagsino/run-coach-ai/backend`
**Go version:** 1.26 (installed via Homebrew at `/opt/homebrew/bin/go`).

### Structure
```
backend/
├── cmd/
│   ├── server/       # HTTP API entrypoint (/healthz, /auth/strava/*, /chat)
│   ├── migrate/      # DB migration runner
│   ├── strava-mcp/   # self-hosted Strava MCP server (stdio), wraps Strava REST API
│   └── strava-check/ # CLI: proves list_activities over MCP end-to-end
├── internal/
│   ├── config/       # env-based configuration (config.Load)
│   ├── db/           # pgx pool + embedded migrations
│   │   └── migrations/  # {version}_{title}.{up|down}.sql
│   ├── strava/       # OAuth (oauth), token store, REST client (rest)
│   ├── mcpclient/    # MCP client: spawns strava-mcp, connects over stdio
│   ├── agent/        # Claude tool-calling loop bridged to MCP tools
│   └── api/          # HTTP handlers (Server struct)
└── .env.example
```

### MCP architecture (important)
Both data sources are **self-hosted MCP servers**; the backend is a pure **MCP client**.
- **Strava:** we run our own `strava-mcp` server (`cmd/strava-mcp`) that wraps the *free* Strava REST API, authenticated with the OAuth token from `internal/strava`. We do **not** use the hosted `mcp.strava.com` (it needs a paid Strava subscription and appears gated to Claude's first-party connector).
- **Garmin (Phase 3):** self-hosted `garmin_mcp` container, connected the same way.
- The agent (`internal/agent`) lists MCP tools, exposes them to Claude, and executes Claude's tool calls against the MCP session.

### Run locally
```bash
cd backend
cp .env.example .env        # first time; fill in real secrets

go run ./cmd/migrate up     # apply DB migrations (down = roll back)
go run ./cmd/server         # start API on http://localhost:8080

curl localhost:8080/healthz # {"status":"ok","db":"up"}

# After connecting Strava (visit /auth/strava/login in a browser):
go run ./cmd/strava-check   # prove list_activities over MCP returns real data
curl -s localhost:8080/chat -d '{"question":"how many runs did I do last week?"}'
```

Verify MCP plumbing without any credentials:
```bash
go test ./internal/mcpclient/   # builds strava-mcp, connects, checks tool discovery
```

### Database
- Local Postgres server (asdf/Homebrew), passwordless socket trust as user `postgres`.
- Dev DB: `runcoach_dev`. Connection string in `DATABASE_URL`.
- Migrations use `golang-migrate` as a library with embedded SQL (no separate CLI to install).
- Tables (migration `000001_init`): `chat_sessions`, `chat_messages`, `activity_cache` (source-agnostic: `strava` now, `garmin` later).

### Tech choices
- HTTP: stdlib `net/http` (Go 1.22+ `ServeMux` method+path patterns) — no framework.
- Postgres: `pgx/v5` + `pgxpool`.
- Migrations: `golang-migrate/v4` (embedded, pgx5 driver).
- Config: `.env` via `joho/godotenv` in dev.

### Code style
- Standard Go: `gofmt`/`go vet` clean. Wrap errors with `%w` and context.
- Keep packages small and purpose-named under `internal/`.
- `cmd/*` entrypoints stay thin; logic lives in `internal/`.

## Required env vars
See `backend/.env.example`. Summary:
| Var | Purpose |
|---|---|
| `PORT` | HTTP port (default 8080) |
| `DATABASE_URL` | Postgres connection string |
| `STRAVA_CLIENT_ID` / `STRAVA_CLIENT_SECRET` | Strava OAuth app creds |
| `STRAVA_REDIRECT_URI` | OAuth callback (default `http://localhost:8080/auth/strava/callback`) |
| `STRAVA_MCP_COMMAND` | Command to launch strava-mcp (default `go run ./cmd/strava-mcp`) |
| `ANTHROPIC_API_KEY` | Claude API access |
| `ANTHROPIC_MODEL` | Agent model (default `claude-sonnet-5`) |

## Current phase
**Phase 2 — Backend Foundations** (Strava only, no Garmin, no frontend). Track progress in `PROGRESS.md`.
