# RunCoach AI — Progress

Tracks work across sessions so we can pick up where we left off. One section per phase; check items off as they're completed and verified.

---

## Phase 2 — Backend Foundations

Backend-only. Strava first, no Garmin yet, no frontend. See `running-agent-feature-spec.md` §11 (Phase 2) and DESIGN.md for context.

### Setup (Step 0)
- [x] Initialize git repo + initial commit of existing docs
- [x] Create GitHub repo (personal `lmagsino/run-coach-ai`) and push
- [x] Create `phase-2-backend` branch off main
- [x] Create `PROGRESS.md` (this file)
- [x] Create one `phase-2`-labeled GitHub issue per task below (issues #1–#6)

### Tasks
- [ ] **1. Prerequisites** — Register Strava app (client ID/secret), confirm Anthropic API key, define `.env` contents
- [x] **2. Go project init** — `go.mod`, folder structure (`/cmd`, `/internal`), Postgres connection + migrations for: chat sessions, chat messages, activity cache ✅ (builds clean, migrations apply, `/healthz` → db:up)
- [x] **3. Root `CLAUDE.md`** — how to run backend locally, code style/conventions, project structure, required env vars ✅ (includes "commit gradually and often" convention)
- [x] **4. Strava OAuth flow** — code complete & verified w/o creds (authorize redirect, CSRF state, token store, auto-refresh). ⏳ Live code→token exchange still to be run with real Strava creds.
- [x] **5. Strava MCP** — code complete; MCP plumbing verified by integration test. **Architecture change:** instead of the hosted `mcp.strava.com` (needs a paid Strava subscription + appears gated to Claude's own connector), we run a **self-hosted `strava-mcp` server** (`backend/cmd/strava-mcp`, written by us) that wraps the free Strava REST API; the backend connects to it as an MCP client (`internal/mcpclient`). ✅ MCP plumbing verified by a creds-free integration test (subprocess + handshake + tool discovery). ⏳ Live `list_activities` **real data** pending Strava creds + OAuth (`go run ./cmd/strava-check`).
- [x] **6. Claude tool-calling loop** — code complete (`internal/agent` + `POST /chat`). Bridges MCP tools to Claude's tool-use loop; grounded-answer system prompt with current-date injection; model = `claude-sonnet-5` (configurable). ⏳ Live answer still to be run with Anthropic key + real Strava data.

### Finalize
- [x] Merge `phase-2-backend` → main via PR (clean checkpoint before Phase 3)

### Deferred to later phases (not Phase-2 scope)
- Live verification of Tasks 4–6 against real Strava + Anthropic credentials (needs Task 1 prerequisites).
- `/chat` multi-turn memory (chat_sessions/chat_messages tables exist but unused).
- Activity caching into `activity_cache` (table exists but unused).
- Additional Strava MCP tools (streams, zones, gear, performance) — only `list_activities` implemented.

---

## Phase 3 — Garmin Integration
_Not started. See spec §11._

## Phase 4 — Frontend
_Not started. Build Vue chat UI against DESIGN.md._

## Phase 5 — Polish & Demo Readiness
_Not started._
