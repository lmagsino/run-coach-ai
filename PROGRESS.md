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

Code-only phase. No live Garmin account, no real credentials, no live API calls — everything that would hit a live service is mocked/stubbed. See `running-agent-feature-spec.md` §11 (Phase 3).

### Setup (Step 0)
- [x] Create `phase-3-garmin` branch off main
- [x] Add this Phase 3 section to `PROGRESS.md`
- [x] Create one `phase-3`-labeled GitHub issue per task below (issues #9–#13)

### Tasks
- [ ] **1. `garmin_mcp` scaffolding** — Dockerfile/compose config for running the `Taxuspt/garmin_mcp` container, plus Go client code (`internal/mcpclient`) that connects to it as an MCP tool source, structured like the existing Strava client
- [ ] **2. Second tool source in the backend** — register Garmin's tools alongside Strava's in the agent's tool-calling loop (code-level only)
- [ ] **3. Cross-source tool selection** — extend the Claude tool-calling logic so the agent picks Strava-only, Garmin-only, or both per question; verified with mocked/stubbed tool responses
- [ ] **4. Mock-based cross-source tests** — fake sleep dataset + fake pace dataset, asserting the agent pulls from and combines both sources
- [ ] **5. `CLAUDE.md` update** — Garmin container setup notes (how it gets run/authenticated in Phase 5)

### Finalize
- [ ] Merge `phase-3-garmin` → main via PR (clean checkpoint before Phase 4)

### Deferred to Phase 5 (not Phase-3 scope)
- Live Garmin verification: real Garmin account, real credentials (email/password + MFA), container actually authenticated and running, real cross-source question answered against real data.
- This sits alongside the Phase 2 deferral above (live Strava OAuth + real `list_activities` + a live Claude answer), so Phase 5 is the single point where the whole system is connected and validated together.

## Phase 4 — Frontend
_Not started. Build Vue chat UI against DESIGN.md._

## Phase 5 — Polish & Demo Readiness
_Not started._
