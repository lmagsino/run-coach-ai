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
- [ ] **4. Strava OAuth flow** — authenticate and obtain an access token
- [ ] **5. Strava MCP client** — call hosted MCP (`https://mcp.strava.com/mcp`); prove `list_activities` returns real data
- [ ] **6. Claude tool-calling loop** — test question ("how many runs did I do last week") → calls Strava tool → grounded answer from real data

### Finalize
- [ ] Merge `phase-2-backend` → main via PR (clean checkpoint before Phase 3)

---

## Phase 3 — Garmin Integration
_Not started. See spec §11._

## Phase 4 — Frontend
_Not started. Build Vue chat UI against DESIGN.md._

## Phase 5 — Polish & Demo Readiness
_Not started._
