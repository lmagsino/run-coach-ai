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
- [x] **1. `garmin_mcp` scaffolding** — ✅ `deploy/garmin-mcp/` (Dockerfile installing `Taxuspt/garmin_mcp` pinned to commit `68ca159`, compose file for build + one-time `garmin-mcp-auth` login into a named token volume) and `internal/mcpclient/garmin.go` mirroring the Strava client, with a tool allowlist trimming upstream's 110+ tools to the running-relevant ones. `GARMIN_MCP_COMMAND` unset ⇒ Garmin disabled. Compose config validates; Go builds/vets clean. ⏳ Image never built or authenticated (Phase 5).
- [x] **2. Second tool source in the backend** — ✅ `agent.New` now takes N named `Source`s; tool names namespaced per source (`garmin__get_sleep_data`) because both servers expose `get_activities`; `Answer` returns a `Result` with the tool-call trail, surfaced as `"sources"` on `POST /chat`. Verified by tests against in-process stub MCP servers (namespacing + routing each colliding tool to its owner).
- [x] **3. Cross-source tool selection** — ✅ system prompt now built from the agent's actual source set: per-source "what it can/can't answer" guidance, plus multi-source selection rules appended only when >1 source is registered. Verified with a scripted stand-in for the Messages API (httptest + base-URL override) driving each plan: Strava-only, Garmin-only, both-in-one-turn, sequential follow-up, no-tools, hallucinated tool name, failing tool, `maxTurns` exhaustion. ⏳ Whether the *real* model picks the right plan per question is its judgment — Phase 5.
- [x] **4. Mock-based cross-source tests** — ✅ `internal/agent/crosssource_test.go`: fake 8-week Garmin sleep/HRV series + matching Strava long-run series with a signal planted across both (nights <6.2h precede runs ~25s/km slower). Covers sleep-vs-pace (parallel tool calls), overtraining (sequential), and a single-source question that must *not* pull the other source in. Asserts both payloads reach the model verbatim. Plus an opt-in live-model test (`RUNCOACH_LIVE_MODEL_TESTS=1`) that lets the real model choose its own sources over the same fake data — skipped by default, so `go test ./...` makes no network call.
- [x] **5. `CLAUDE.md` update** — ✅ new "Garmin MCP container" section (Phase 5 build + interactive `garmin-mcp-auth` steps, ~6-month token expiry and how it fails, why compose doesn't run the server, tool allowlist caveat, pinned upstream ref); MCP architecture rewritten for two sources + tool namespacing + config-gated source set; testing notes (stub MCP servers, scripted Messages API, opt-in live-model test); env var table, repo layout, `/chat` response shape, current phase.

### Finalize
- [x] Merge `phase-3-garmin` → main via PR (clean checkpoint before Phase 4) — PR #14, merged as `399783c`

### Deferred to Phase 5 (not Phase-3 scope)
- Live Garmin verification: real Garmin account, real credentials (email/password + MFA), container actually authenticated and running, real cross-source question answered against real data.
- This sits alongside the Phase 2 deferral above (live Strava OAuth + real `list_activities` + a live Claude answer), so Phase 5 is the single point where the whole system is connected and validated together.

## Phase 4 — Frontend
_Not started. Build Vue chat UI against DESIGN.md._

## Phase 5 — Polish & Demo Readiness
_Not started._
