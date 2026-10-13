# Phase 5: Garmin live, Strava out

Branch `phase-5-garmin-live` off `main`. Blocked by: nothing. About four days.

Outcome: the coach answers the five example questions from Leo's real Garmin data on Opus
5.5, with Strava gone from the code, the upstream pin current, every read tool exposed and
every write tool denied, and the cost per question measured. Nothing has ever run live
before this phase; it is the one that finds out whether the idea works.

Decisions this phase builds on: ADR 0001, 0004, 0006, 0008. Terms: `CONTEXT.md`.

## Step 0 (Leo, not an agent)

- An Anthropic API key with billing on.
- The Garmin account to use (personal), email, password and MFA device at hand.
- Docker Desktop running. `backend/.env` rebuilt from `.env.example` (the current one is
  stale and all placeholders).
- After 5.4 lands: `cd deploy/garmin-mcp && docker compose build && docker compose
  --profile auth run --rm garmin-mcp-auth`, enter the MFA code once. Tokens land in the
  named volume `run-coach-ai-garmin-tokens`.

## Tasks

### Task 5.1: Remove Strava from the backend

**Description:** Delete every Strava code path so the backend is Garmin only and the agent
is built from a one-source set. Keep the per-source namespacing and `connect()`.

**Acceptance criteria:**
- [ ] `backend/cmd/strava-mcp`, `backend/cmd/strava-check`, `backend/internal/strava/`,
      `backend/internal/api/strava_auth.go`, `backend/internal/mcpclient/strava.go` and
      `strava_test.go` are deleted; `SourceStrava` and the `/auth/strava/*` routes are gone;
      `config.Config` has no `Strava*` fields.
- [ ] `internal/agent/agent.go` keeps `Source`, namespacing and `Observe`; `sourceGuidance`
      has a Garmin entry only; `selectionRules` (multi-source) is deleted; the system
      prompt no longer mentions Strava.
- [ ] `internal/mockchat` scenarios reference Garmin tools only, every name present in the
      upstream tool list (`deploy/garmin-mcp/tools.txt`, added by 5.4; until then the
      Phase 3 names `get_activities`, `get_sleep_data`, `get_hrv_trend`,
      `get_stress_summary`, `get_training_load_trend`); the pinning test still passes.
- [ ] Migration `000002_oauth_tokens` is deleted and `activity_cache` is removed from
      `000001_init` (ADR 0008: nothing has run outside Leo's laptop).
- [ ] `go build ./... && go vet ./... && go test -count=1 ./...` green;
      `grep -ri strava backend/` returns nothing.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./...`
- [ ] Build succeeds: `cd backend && go build ./...`
- [ ] Manual check: `RUNCOACH_MOCK=1 go run ./cmd/server` answers the five mock questions
      with Garmin-only steps.

**Blocked by:** none

**Files likely touched:** the deletions above, `internal/config/config.go`,
`internal/api/api.go`, `internal/api/chat.go`, `internal/api/sources.go`,
`internal/agent/agent.go` and its tests, `internal/mockchat/mockchat.go`,
`internal/db/migrations/*`, `.env.example`.

**Estimated scope:** M (many files, all deletions or one-line edits)

### Task 5.2: Remove Strava from the frontend and the docs

**Description:** The UI and the project docs describe a Garmin-only product.

**Acceptance criteria:**
- [ ] `frontend/src/lib/stepLabels.js` has no Strava keys; `SOURCE_NAMES` is Garmin only;
      `App.vue` greeting, `AppHeader.vue` and `Composer.vue` hint speak of Garmin only;
      `npm test` green.
- [ ] `README.md`, `CLAUDE.md`, `running-agent-feature-spec.md`, `DESIGN.md` and
      `backend/.env.example` describe Garmin only; `CLAUDE.md`'s repo layout, MCP
      architecture, env table and "current phase" sections are rewritten for this plan and
      point at `docs/adr/`, `CONTEXT.md` and `tasks/`.
- [ ] `grep -ri strava` over the repo matches only `docs/adr/0001*`, `PROGRESS.md`
      (history) and the spec's revision note.

**Verification:**
- [ ] Tests pass: `cd frontend && npm test`
- [ ] Build succeeds: `cd frontend && npm run build`
- [ ] Manual check: the header and greeting in mock mode name Garmin only.

**Blocked by:** 5.1

**Files likely touched:** `frontend/src/lib/stepLabels.js` and test, `frontend/src/App.vue`,
`frontend/src/components/AppHeader.vue`, `frontend/src/components/Composer.vue`,
`README.md`, `CLAUDE.md`, `running-agent-feature-spec.md`, `DESIGN.md`.

**Estimated scope:** M

### Task 5.3: Database optional

**Description:** With nothing to store until Phase 7a, the server runs without Postgres.

**Acceptance criteria:**
- [ ] `DATABASE_URL` unset or empty starts the server; `GET /healthz` returns
      `{"status":"ok","db":"off"}`; set, it connects as before and reports `db: "up"`.
- [ ] `cmd/migrate` still works when the URL is set and exits with a clear message when
      it is not.
- [ ] CI unchanged (it already runs without a database).

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./...`
- [ ] Manual check: `DATABASE_URL= go run ./cmd/server` then `curl localhost:8080/healthz`.

**Blocked by:** 5.1

**Files likely touched:** `backend/cmd/server/main.go`, `backend/internal/config/config.go`,
`backend/internal/api/health.go`, `backend/cmd/migrate/main.go`.

**Estimated scope:** S

### Task 5.4: Garmin client: pin bump and read-surface filter in Go

**Description:** Move the upstream pin to the current commit and replace the ten-name
allowlist with a deny pattern applied in Go to the live tool list (ADR 0004).

**Acceptance criteria:**
- [ ] `deploy/garmin-mcp/Dockerfile` pins `GARMIN_MCP_REF=43b0c5b375fc1c7bd6bdbead69968dfabddec265`
      (upstream HEAD on 12 Oct 2026) and the compose file no longer forwards
      `GARMIN_ENABLED_TOOLS`; `deploy/garmin-mcp/tools.txt` lists the 155 upstream tool
      names at that commit with a one-line header saying how it was produced.
- [ ] `mcpclient.DefaultGarminTools` is deleted. A pure function
      `mcpclient.ReadOnly(name string) bool` denies names matching
      `set_* add_* create_* delete_* upload_* remove_* log_* update_* download_*
      request_reload set_fit_download_dir` and the nutrition, challenge, gear, course,
      workout-builder, workout-template and women's-health groups, and allows the rest;
      a table test covers one name per pattern and the three Phase 3 names that never
      existed (`get_heart_rate_variability_summary`, `get_vo2_max_trend`,
      `get_daily_stress`) are nowhere in the code.
- [ ] `agent.buildTools` applies the source's filter to `tools/list` before exposing
      tools; a stub MCP test proves a `delete_*` tool is listed by the server and absent
      from the model's tool set, and that calling it by name returns the existing
      unknown-tool error.
- [ ] The Garmin client passes `GARMINTOKENS` through unchanged (local dev keeps the
      Docker volume) and the child environment is built from an explicit list, not
      `os.Environ()`; a test asserts `GARMIN_EMAIL` and `GARMIN_PASSWORD` are absent
      from the child env even when set in the parent (ADR 0003).

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./...`
- [ ] Build succeeds: `cd deploy/garmin-mcp && docker compose build`
- [ ] Manual check: `docker compose config` validates.

**Blocked by:** 5.1

**Files likely touched:** `deploy/garmin-mcp/Dockerfile`, `deploy/garmin-mcp/docker-compose.yml`,
`deploy/garmin-mcp/tools.txt` (new), `backend/internal/mcpclient/garmin.go` and a new
test, `backend/internal/mcpclient/mcpclient.go`, `backend/internal/agent/agent.go`,
`backend/internal/agent/tools_test.go`.

**Estimated scope:** M

### Task 5.5: Live tool-diff test

**Description:** An opt-in test that talks to the real, authenticated garmin-mcp and
proves the read surface is what the plan says. Also records the facts later phases
depend on instead of guessing them.

**Acceptance criteria:**
- [ ] `RUNCOACH_LIVE_GARMIN_TESTS=1 go test ./internal/mcpclient -run Live` connects
      through `GARMIN_MCP_COMMAND`, lists tools, and asserts: every name in the prompt's
      category map (5.6) exists; no exposed name matches a deny pattern; the exposed count
      is between 80 and 100; it is skipped without the variable.
- [ ] The test (or a small `go run ./cmd/garmin-check`) calls `get_user_profile` and one
      sleep tool and prints the shapes, and the result is pasted into
      `docs/live/phase-5.md` under "Tool facts": the exposed count, the exact file names
      in the token volume (`docker run --rm -v run-coach-ai-garmin-tokens:/t alpine ls /t`),
      and whether `~/.garminconnect_base64` was written by `garmin-mcp-auth`.
- [ ] Spawn time (connect to `tools/list` returned) is measured and recorded.

**Verification:**
- [ ] Tests pass: the live test green on Leo's laptop; `go test ./...` still makes no
      network call without the variable.
- [ ] Manual check: `docs/live/phase-5.md` has the "Tool facts" section filled.

**Blocked by:** 5.4, Step 0 (built and authenticated image)

**Files likely touched:** `backend/internal/mcpclient/garmin_live_test.go` (new),
`backend/cmd/garmin-check/main.go` (new, optional), `docs/live/phase-5.md` (new).

**Estimated scope:** S

### Task 5.6: Garmin-only prompt, Opus 5.5, effort, caching

**Description:** Rewrite the system prompt for one source with ~90 tools, and move the
model call to Opus 5.5 with explicit effort and prompt caching (ADR 0006).

**Acceptance criteria:**
- [ ] The system prompt keeps `basePrompt` grounding rules and `answerFormat` (the
      `[figure: …]` marker stays), drops multi-source text, and adds a category map of
      about twelve lines (activities, sleep, HRV, stress, body battery, training load and
      status, readiness, VO2max, steps, body composition, profile and devices, FIT) naming
      the two or three tools to reach for in each; every tool named exists in
      `deploy/garmin-mcp/tools.txt` (a test checks the list against the file).
- [ ] The prompt tells the model to pass `return_timeseries=false` (or the upstream
      equivalent recorded in 5.5) unless the question needs intra-day detail, and to call
      at most eight tools.
- [ ] `ANTHROPIC_MODEL` defaults to `claude-opus-5-5`; a new `ANTHROPIC_EFFORT` (default
      `medium`) is sent as the request's effort parameter per the Go SDK v1.60 API
      (verify the field name against the SDK source, do not guess); `.env.example` and
      `CLAUDE.md` updated.
- [ ] `cache_control: ephemeral` is set on the system prompt and on the last tool
      definition; the scripted-model tests assert both markers are present on every
      request.
- [ ] `maxTurns` stays 8; `maxPriorTurns` stays 6.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./...`
- [ ] Manual check: a request body logged in debug mode shows model, effort and the two
      cache markers.

**Blocked by:** 5.1

**Files likely touched:** `backend/internal/agent/agent.go`, `backend/internal/agent/*_test.go`,
`backend/internal/config/config.go`, `backend/.env.example`, `CLAUDE.md`.

**Estimated scope:** M

### Task 5.7: Live pass: five questions, measured

**Description:** The first time the product runs for real. Mock off, real Garmin, real
model, through the UI, and everything written down.

**Acceptance criteria:**
- [ ] `docs/live/phase-5.md` has one section per example question (CONTEXT.md "Live
      pass"; questions in the spec §1) recording: tools called in order, whether every
      number in the reply traces to a tool result (grounded yes/no with the counter-example
      if no), wall time, input tokens, cached input tokens, output tokens, and cost at
      Opus 5.5 list prices.
- [ ] Each question is run at `ANTHROPIC_EFFORT=medium` and `high`, and once on
      `claude-sonnet-5-5` at `medium`, with a one-paragraph comparison and a recommended
      default written at the top of the file.
- [ ] The cold-spawn cost (first reply after idle) and the per-reply spawn time are
      recorded.
- [ ] A list of every wrong-tool choice, ungrounded claim, empty-range or tool-error
      behaviour seen, each with the question that produced it, for 5.8.

**Verification:**
- [ ] Manual check: the file is complete; no section says "TBD".

**Blocked by:** 5.5, 5.6, Step 0

**Files likely touched:** `docs/live/phase-5.md`.

**Estimated scope:** M (mostly Leo's time)

### Task 5.8: Rough edges from the live pass

**Description:** Fix what 5.7 found, prompt first, code second, and re-run the affected
questions.

**Acceptance criteria:**
- [ ] Every item in 5.7's list is either fixed (with the re-run result appended to
      `docs/live/phase-5.md`) or recorded as deferred with a reason and a phase.
- [ ] UI edge cases proven in mock mode with a new scenario where needed: a very long
      reply, a tool returning an empty range, a tool error mid-reply (the reply says what
      it could not read).
- [ ] `PROGRESS.md` Phase 5 checked off; `README.md` has a two-paragraph "status" that
      says what works live.

**Verification:**
- [ ] Tests pass: backend and frontend suites green; CI green.
- [ ] Manual check: the five questions re-run grounded at the recommended effort.

**Blocked by:** 5.7

**Files likely touched:** `backend/internal/agent/agent.go`, `backend/internal/mockchat/mockchat.go`,
frontend components as found, `docs/live/phase-5.md`, `PROGRESS.md`, `README.md`.

**Estimated scope:** M

## Checkpoint: Phase 5

- [ ] All tests pass; CI green on the PR.
- [ ] No Strava left outside history (`grep -ri strava`).
- [ ] `docs/live/phase-5.md` complete, with the recommended effort and the cost per question.
- [ ] PR `phase-5-garmin-live` → `main` merged by Leo.

## What the Verifier drives

In the running app with `RUNCOACH_MOCK` off and the authenticated container: the five
questions, each PASS only if the reply is grounded and the steps name Garmin tools that
exist. In mock mode: the same five plus the three edge scenarios from 5.8. There is no
feature flag in this phase; the matrix row is "live vs mock".
