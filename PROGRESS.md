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

---

## Phase 4 — Frontend

Code-only phase, like Phases 2–3. Build the Vue 3 chat UI against DESIGN.md and wire it to
the Go backend, tested against **mocked** Strava/Garmin responses — no live credentials.
See `running-agent-feature-spec.md` §11 (Phase 4).

Two architecture decisions taken at kickoff, both because the Phase 2/3 backend has no way
to feed a browser:
- **Status steps come from real backend events, not frontend theatre.** `POST /chat` is a
  single round-trip that returns only when the answer is done, so DESIGN.md's "demo
  signature" status list had nothing to render. Phase 4 adds an SSE endpoint that emits
  per-tool-call events as the agent loop runs. Faking the steps client-side would have to
  be thrown away in Phase 5.
- **The mock data lives in the Go backend, not the frontend.** Phase 2/3's stubs are Go
  *test* fixtures (`stubmcp_test.go`, `fakemodel_test.go`) — unreachable from a browser. A
  dev-only mock mode serves canned answers through the real handler, so the frontend
  exercises the real endpoint, response shape, and CORS. Phase 5 flips the flag off.

### Setup (Step 0)
- [x] Create `phase-4-frontend` branch off main
- [x] Add this Phase 4 section to `PROGRESS.md`
- [x] Create one `phase-4`-labeled GitHub issue per task below (issues #17–#21)

### Tasks
- [x] **1. Vue 3 project setup** (#17) — Vite + Vue 3 + Tailwind under `frontend/`, Tailwind theme
      carrying DESIGN.md's palette tokens and the Space Grotesk / Hanken Grotesk pairing
      (self-hosted via @fontsource, per DESIGN.md §3, since the build is local-only) ✅ Vite 8 + Vue 3.5 + Tailwind v4 (CSS-first `@theme`); all 9 palette tokens, both families, `--container-measure`, the three §7 keyframes, and the non-negotiable `prefers-reduced-motion` reset live in `src/style.css`; `npm run build` clean, fonts bundled locally (no Google Fonts request)
- [x] **2. Core chat UI components** (#18) — message list (user bubble vs. bubble-less agent
      prose), composer + send action, and the status-indicator component with
      done/active/pending step states ✅ Components match `design/mockup.html` (verified by screenshot at 1280/561/540px). Answer text is parsed into DESIGN.md §5 blocks (lede + paragraphs + at most one figure, via a `[figure: value | caption]` marker) rather than the backend returning JSON, so a malformed field can never cost the whole reply. Custom `wide:` breakpoint at 561px because Tailwind's `sm:` would fire 80px early. Fixed a Vue reactivity bug where mutating the pushed (raw) turn object never re-rendered.
- [x] **3. Wire frontend to the backend** (#19) — backend: SSE `/chat/stream` emitting tool-call
      step events + CORS for the Vite dev origin + dev mock mode; frontend: send a message,
      stream status steps, render the answer ✅ `agent.Observe` emits start/end events per tool
      call; `POST /chat/stream` relays them as `{source, tool, state}` (no label — copy is
      DESIGN.md's, so the frontend maps it in `lib/stepLabels.js`). `GET /sources` drives the
      header, greeting and composer hint, so a Strava-only or mocked backend never claims
      otherwise. Backend tested (SSE frame shape, step pairing, per-scenario source plans,
      CORS, credential-free mock path); verified in a real browser against the real server
      for all five spec §4 questions — correct labels, figures, no console or network errors.
      Also fixed mock scenarios that named **nonexistent tools** (the real ones are
      `list_activities` and the Garmin allowlist, which has *no* readiness tool); a test now
      pins them so labels can't silently fall back in Phase 5.
- [x] **4. Mocked end-to-end test** (#20) — walk the 5 example questions from spec §4 through the
      UI against mocked Strava/Garmin data, confirming rendering with fake data ✅ All five driven
      through a real browser against the real Go server: correct source-naming step labels, one
      figure pull-quote each, no console errors, no horizontal overflow. Error paths checked too
      (backend unreachable, in-band SSE `error` event, composer recovers and stays usable).
      **Found that multi-turn memory did not exist** — the thread displayed history but each
      request started fresh, so a follow-up had no referent, against spec §5. Implemented:
      `agent.AnswerInConversation` replays prior exchanges (capped at 6, incomplete turns
      skipped, final answer text only — not tool payloads), and the client sends them since the
      server holds no session state. Verified the follow-up carries all 5 prior turns, in order.
      Also replaced fetch's opaque "Failed to fetch" with a message naming the actual problem.
- [x] **5. Responsive/layout polish** (#21) — correct in a normal browser window, no breakage at
      reasonable widths (DESIGN.md §4 responsive rules) ✅ Swept 15 widths from 1920 down to 320px:
      zero horizontal overflow at every one, and the compact treatment flips at exactly 560/561
      (figure 34→28px, body 16→15px, padding 28→18px) as §4 specifies. Edge cases: a 300-char
      unbroken string doesn't push the layout sideways, the user bubble holds its 82% cap, the
      thread auto-scrolls to the newest turn, and `prefers-reduced-motion` genuinely resolves
      animations to `none`. One fix: below the breakpoint the "Field Notes" tag wrapped to two
      lines and squeezed the connection status into a stub, so the tag (decorative) now hides
      there while the "Mock data" marker never truncates.

### Finalize
- [x] Merge `phase-4-frontend` → main via PR (clean checkpoint before Phase 5) — PR #22, merged as `7d20a3a`

### Deferred to Phase 5 (not Phase-4 scope)
- Everything the UI shows is mocked: no live Strava OAuth, no authenticated Garmin
  container, no real Claude answers. Turning the mock flag off and re-walking the same 5
  questions against real data is the Phase 5 test.
- Multi-turn memory *within* a session now works (task 4), but the thread lives only in the
  browser: the `chat_sessions`/`chat_messages` tables are still unused, so a page reload starts
  a new conversation. Persisting it is Phase 5 or later.

**Empty state (task 2, resolved):** DESIGN.md defines none — `design/mockup.html` shows a
conversation already underway — so first load was a blank thread. Filled with the quietest
thing that works: the agent's existing `RUNCOACH` label plus one muted line, no new
component and no suggestion chips (DESIGN.md §1 restraint). The copy names both sources, so
task 3 must drive it from the sources actually registered — Garmin is config-gated, and a
Strava-only setup must not be told the coach can see sleep and HRV.

---

## Phase 5 — Polish & Demo Readiness
_Not started._
