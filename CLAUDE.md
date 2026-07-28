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
├── deploy/garmin-mcp/                 # Garmin MCP container (Phase 3)
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
│   ├── mcpclient/    # MCP client: spawns strava-mcp / garmin_mcp over stdio
│   ├── agent/        # Claude tool-calling loop over N namespaced MCP sources
│   └── api/          # HTTP handlers (Server struct)
└── .env.example
```

### MCP architecture (important)
Both data sources are **self-hosted MCP servers**; the backend is a pure **MCP client**. Each is a subprocess speaking MCP over stdio, spawned per chat request and closed when the request ends.
- **Strava:** we run our own `strava-mcp` server (`cmd/strava-mcp`) that wraps the *free* Strava REST API, authenticated with the OAuth token from `internal/strava`. We do **not** use the hosted `mcp.strava.com` (it needs a paid Strava subscription and appears gated to Claude's first-party connector).
- **Garmin:** the upstream `Taxuspt/garmin_mcp` server, run as a container (`deploy/garmin-mcp/`) and installed from git at a pinned commit — we don't vendor or patch it. Unlike Strava there's no per-request token: the container authenticates itself from OAuth tokens cached in a mounted volume.
- **Tool names are namespaced per source** (`strava__get_activities`, `garmin__get_sleep_data`). Both servers expose an activities listing, so unprefixed names would collide — and a routing bug there would silently answer Strava questions with Garmin data.
- The agent (`internal/agent`) lists each source's MCP tools, exposes them to Claude under namespaced names, routes each tool call back to the session that owns it, and records which sources an answer drew on (`POST /chat` returns them as `sources`).
- **Which sources are active is config, not code.** Strava is always registered; Garmin joins only when `GARMIN_MCP_COMMAND` is set. The system prompt is built from the registered set, so with Garmin off the agent is never told it can fetch sleep or HRV data. A *configured but unreachable* source fails the request rather than quietly answering without it.

### Run locally
```bash
cd backend
cp .env.example .env        # first time; fill in real secrets

go run ./cmd/migrate up     # apply DB migrations (down = roll back)
go run ./cmd/server         # start API on http://localhost:8080

curl localhost:8080/healthz # {"status":"ok","db":"up"}

# After connecting Strava (visit /auth/strava/login in a browser):
go run ./cmd/strava-check   # prove list_activities over MCP returns real data
# Answers are {"answer": "...", "sources": ["strava","garmin"]} — `sources` lists
# the sources actually queried, which is what the Phase 4 status lines render.
curl -s localhost:8080/chat -d '{"question":"how many runs did I do last week?"}'
```

Verify MCP plumbing and agent behaviour without any credentials:
```bash
go test ./...                   # no network calls, no Docker, no accounts needed
```
- `internal/mcpclient` builds `strava-mcp`, connects over stdio, checks tool discovery.
- `internal/agent` runs the full tool-calling loop against **in-process stub MCP servers** (`stubmcp_test.go`) and a **scripted stand-in for the Messages API** (`fakemodel_test.go`, an `httptest` server plus a base-URL override). That covers tool namespacing/routing, each source-selection plan, and the cross-source scenarios in `crosssource_test.go`.
- What the stubs *can't* prove is whether the real model picks the right sources for a question. `TestCrossSourceReasoningWithLiveModel` does that against fabricated data, and is skipped unless you opt in:
  ```bash
  RUNCOACH_LIVE_MODEL_TESTS=1 go test ./internal/agent/ -run LiveModel -v   # costs Anthropic tokens
  ```

### Garmin MCP container
Not built or authenticated yet — it's wired up in code, and `deploy/garmin-mcp/` is validated only by `docker compose config`. The steps below are what Phase 5 runs.

```bash
cd deploy/garmin-mcp
docker compose build                                    # installs garmin_mcp at the pinned commit
docker compose --profile auth run --rm garmin-mcp-auth  # interactive: email/password, then MFA code
```
Then uncomment `GARMIN_MCP_COMMAND` in `backend/.env` and restart the server; it should log `agent tool sources: strava, garmin`.

Things worth knowing before touching this:
- **Auth is a one-off, and interactive.** `garmin-mcp-auth` prompts for the MFA code on the terminal and writes OAuth tokens into the `run-coach-ai-garmin-tokens` volume, where they last ~6 months. The server reuses them, so credentials aren't needed per request. When they expire the symptom is tool calls failing, not a clean error — rerun the auth command.
- **Compose does not run the server.** A stdio MCP server has no long-running role: the backend spawns its own `docker run -i --rm ...` per request. Compose exists for the build and the auth step only, which is why both services sit behind profiles.
- **Garmin has no official API.** `garmin_mcp` drives the Garmin Connect *private* endpoints with your personal login, so it can break when Garmin changes them, and aggressive polling risks the account. Keep requests modest.
- **The tool list is deliberately narrowed.** Upstream exposes 110+ tools; `mcpclient.DefaultGarminTools` allowlists ~10 (sleep, HRV, stress, training load, VO2 max, body composition, steps, activities) via `GARMIN_ENABLED_TOOLS`. Unrecognized names are ignored *silently*, so Phase 5 should diff this list against what the live container actually reports.
- **Pinned upstream ref.** `GARMIN_MCP_REF` in `deploy/garmin-mcp/docker-compose.yml`. Bump it deliberately, not incidentally.

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
| `GARMIN_MCP_COMMAND` | Command to launch garmin_mcp. **Unset ⇒ Garmin disabled**, Strava is the only source |
| `GARMIN_EMAIL` / `GARMIN_PASSWORD` | Garmin Connect login — only read by the one-time `garmin-mcp-auth` step |
| `GARMIN_IS_CN` | Set `true` only for Garmin Connect China (garmin.cn) |
| `ANTHROPIC_API_KEY` | Claude API access |
| `ANTHROPIC_MODEL` | Agent model (default `claude-sonnet-5`) |

## Current phase
**Phase 3 — Garmin Integration** (code-only: Garmin wired in as a second tool source, no live account, no frontend). Track progress in `PROGRESS.md`.

All live-credential work — Strava OAuth, Garmin auth, real Claude answers — is consolidated into **Phase 5**, so nothing in Phases 2–4 is blocked on account setup. When something here is described as verified, check `PROGRESS.md` for whether that means *code-complete against mocks* or *verified live*.
