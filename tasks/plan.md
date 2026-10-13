# Plan: RunCoach AI, Garmin-only personal MVP

Build first, deploy and test last. Decisions are in `docs/adr/`, terms in `CONTEXT.md`, the
design in `docs/superpowers/specs/2026-10-13-garmin-mcp-upgrade-and-charts-design.md` and
the visual PRD/TRD/Design/Plan at https://claude.ai/artifact/MfXsK5cNaLNZGPENiA6zHt.

Phases 5 to 7 are code only, verified with unit tests, the stub MCP server and mock mode,
like Phases 1 to 4 were. Phase 8 puts it on free hosting and tests it against real Garmin
data and the real model. Each phase is one branch and one PR to `main`.

| Phase | Branch | What it builds | Size |
|---|---|---|---|
| 5 Garmin core | `phase-5-garmin-core` | Strava out, database optional, upstream pin bumped, read surface filtered in Go, Garmin-only prompt on Opus 5.5 with caching | ≈ 2 d |
| 6 Charts | `phase-6-charts` | `coach__show_chart`, SSE `chart`, `lib/chart.js`, `ChartCard.vue`, two mock scenarios | ≈ 3 d |
| 7 Shell | `phase-7-shell` | fresh migrations with `runner_id`, owner login, token paste, saved chats, sidebar, phone drawer, one origin | ≈ 4 d |
| 8 Deploy and test | `phase-8-deploy` | one image, Render free + Neon free, Garmin tokens pasted, live pass on the five questions, Playwright smoke | ≈ 3 d |

Later, after use: streaming deltas and Stop, dark theme, suggestions, search and rename,
Haiku titles. Roadmap: other runners through the MissingMCP gateway (ADR 0002).

## Phase 5: Garmin core

- [ ] 5.1 Remove Strava from the backend: `cmd/strava-mcp`, `cmd/strava-check`,
      `internal/strava`, `strava_auth.go`, `mcpclient/strava.go`, `Strava*` config, the
      multi-source prompt rules, the Strava migration and `activity_cache`; mock scenarios
      on Garmin tools only. Done when `grep -ri strava backend/` is empty and tests pass.
- [ ] 5.2 Remove Strava from the frontend and docs: step labels, greeting, header, composer
      hint; README, CLAUDE.md, feature spec, `.env.example`. Done when only ADR 0001,
      PROGRESS history and the spec's revision note mention it.
- [ ] 5.3 Database optional: the server starts without `DATABASE_URL`; `/healthz` says
      `db: off` (ADR 0008).
- [ ] 5.4 Garmin client: pin `GARMIN_MCP_REF=43b0c5b375fc1c7bd6bdbead69968dfabddec265`;
      `deploy/garmin-mcp/tools.txt` with the 155 upstream names; `mcpclient.ReadOnly(name)`
      deny pattern replaces `DefaultGarminTools`; `agent.buildTools` applies it; the child
      environment is built from scratch (no `GARMIN_EMAIL`/`GARMIN_PASSWORD`, ADR 0003);
      stub test proves a `delete_*` tool never reaches the model.
- [ ] 5.5 Prompt and model: Garmin-only system prompt with a twelve-line category map
      checked against `tools.txt`; `ANTHROPIC_MODEL` default `claude-opus-5-5`;
      `ANTHROPIC_EFFORT` (default `medium`) sent on every request; `cache_control` on the
      system prompt and the last tool, asserted by the scripted-model tests (ADR 0006).
- [ ] Checkpoint: `go test ./...`, `npm test`, CI green; mock mode answers the five
      questions with Garmin-only steps. PR to `main`.

## Phase 6: Charts

- [ ] 6.1 `internal/chart`: `Spec` (kind line|bar|scatter, title, labels, units, data name,
      1–2 series of 5–120 points, caption, optional `partial` on a point), `Validate` naming
      the failing field, `ToolDefinition()` (ADR 0005).
- [ ] 6.2 Agent: built-in source `coach` with `show_chart`; valid call → `Result.Chart`
      and an `Event{Kind: chart}`; second call refused readably; invalid call returns the
      error and no event; `tool_choice` stays `auto`.
- [ ] 6.3 API: SSE `chart` event after the steps and before `answer`; `chart` field on
      `POST /chat`; two mock scenarios with charts (sleep vs pace scatter, training-block
      bars with a partial week) and one with an invalid spec that must drop.
- [ ] 6.4 Prompt: a "Charts" section (one at most, patterns and relationships only, never a
      single number, reshape yourself, pace in s/km, two series only on one unit).
- [ ] 6.5 `frontend/src/lib/chart.js`: pure layout (nice ticks, scales, m:ss pace, trend
      line, dashed second series with direct labels, faint partial bars) with vitest.
- [ ] 6.6 `ChartCard.vue`: SVG in the Field Notes tokens, title, Garmin data used, caption,
      "View data" table with Copy; shown above the prose only once `answer` arrives
      (ADR 0009); 320 px up; reduced motion respected.
- [ ] Checkpoint: tests and CI green; both chart scenarios render in mock mode at 1280,
      561 and 360 px; the invalid one shows prose only. PR to `main`.

## Phase 7: Shell

- [ ] 7.1 Fresh `000001` migration: `runners` (owner seeded), `sessions`,
      `garmin_connections`, `chat_sessions`, `chat_messages`, `usage`, `audit_log`, all
      with `runner_id`; `internal/store.Scoped` is the only data access, with a test that
      scans for stray `pool.Query` calls (ADR 0002, 0008).
- [ ] 7.2 Auth: `auth.Provider` + `OwnerPassword` (bcrypt hash from
      `RUNCOACH_OWNER_PASSWORD_HASH`); cookie session (`HttpOnly; Secure; SameSite=Lax`,
      30 days); `POST /login`, `POST /logout`, `GET /me`; login rate-limited;
      `cmd/hash-password`; mock mode skips login.
- [ ] 7.3 Garmin: `garmin.Source` + `LocalSpawn` (decrypt the bundle into a `0700` temp
      dir, spawn with `GARMINTOKENS`, delete on close); `internal/crypto` AES-256-GCM
      under `RUNCOACH_TOKEN_KEY` with `key_version`; `POST /garmin/tokens` probes the
      pasted bundle with `get_user_profile` before storing; `DELETE /garmin/tokens`;
      status connected / expired / unavailable with a `fix` sentence on `/me` (ADR 0003).
- [ ] 7.4 Chats: `GET/POST /chats`, `PATCH/DELETE /chats/:id`, `GET /chats/:id/messages`,
      `POST /chats/:id/stream` with history loaded server-side; 429 at the daily cap; 409
      while a reply is running; usage from the API's token counts; mock mode uses an
      in-memory store.
- [ ] 7.5 Screens: `/login` with a "waking up" state while `/healthz` is unreachable;
      `/connection` with the chip state, the laptop commands and the paste box.
- [ ] 7.6 Shell: `vue-router` (`/login`, `/`, `/c/:id`, `/connection`); sidebar by day
      with new and delete, the Garmin chip and the usage line; drawer below the `wide:`
      breakpoint; thread, steps, composer reused.
- [ ] 7.7 One origin: the Go binary serves `frontend/dist` through `embed.FS`, API under
      `/api/`, CORS only when `ALLOWED_ORIGIN` is set; SSE `: keepalive` every 15 s;
      `/healthz` reports db and Garmin state with no secrets (ADR 0007).
- [ ] Checkpoint: tests and CI green; login, paste (against the local Docker image), two
      chats and a follow-up work locally with Postgres; mock mode still needs none.
      PR to `main`.

## Phase 8: Deploy (free) and test

- [ ] 8.1 Image: root `Dockerfile` (node build → go build with the dist embedded →
      `python:3.12-slim` with `garmin-mcp` at the pin, non-root); `render.yaml` for one
      free web service with health check `/healthz`; migrations run on start under an
      advisory lock; `.dockerignore`.
- [ ] 8.2 Hosting (Leo): Neon free project; Render free web service from the repo; env
      vars set in the dashboard (`DATABASE_URL`, `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`,
      `ANTHROPIC_EFFORT`, `RUNCOACH_TOKEN_KEY`, `RUNCOACH_OWNER_PASSWORD_HASH`); deploy on
      push to `main`.
- [ ] 8.3 Garmin (Leo): `docker compose build` and `garmin-mcp-auth` once on the laptop;
      paste `~/.garminconnect_base64` on `/connection`; opt-in live tool-diff test
      (`RUNCOACH_LIVE_GARMIN_TESTS=1`) proves the read surface and records the token file
      names and spawn time in `docs/live/phase-8.md`.
- [ ] 8.4 Live pass: the five questions from the phone, each at `medium` and `high`
      effort and once on Sonnet 5.5; tools chosen, grounded or not, chart or not, wall
      time, tokens with and without cache, cost; 60 s idle stream survives Render; recorded
      in `docs/live/phase-8.md` with screenshots.
- [ ] 8.5 Rough edges: prompt first, code second; re-run the affected questions; README
      "status" and "deploy" sections; token refresh instructions.
- [ ] 8.6 Playwright smoke on mock mode in CI: a chart question, a non-chart question,
      login redirect, sidebar new/delete; under three minutes.
- [ ] Done: Leo asks a question from his phone and gets a grounded reply with a chart;
      removing the token row shows the fix text, never a pretend answer.

## Risks

| Risk | Where it is caught |
|---|---|
| Garmin changes a private endpoint and upstream lags | pinned commit; "Garmin unavailable" state; probe on the chip and `/healthz` |
| The model picks wrong tools among ~90 or answers ungrounded | category map; 8.4 records every choice; 8.5 fixes the prompt |
| Cost per question above the estimate | 8.4 measures with and without cache; effort dial; Sonnet 5.5 one line away |
| Render's proxy drops long SSE streams | keepalive in 7.7; proven in 8.4 |
| Token bundle file names differ from the assumption | 8.3 records them before 7.3's spawn is trusted live; the probe fails loudly otherwise |
