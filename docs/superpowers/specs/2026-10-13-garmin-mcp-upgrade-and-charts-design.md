# RunCoach AI — "Ask your Garmin": personal MVP, expansion-ready

Design spec, 13 Oct 2026, **revision 5**. Status: decisions recorded as ADRs, plan written, awaiting Leo's gate.
Decisions: `docs/adr/0001`–`0009`. Terms: `CONTEXT.md`. Plan: `tasks/plan.md` and `tasks/todo.md`.
Full PRD, TRD and design mockups: Claude artifact https://claude.ai/artifact/MfXsK5cNaLNZGPENiA6zHt
(tabs `#prd`, `#trd`, `#design`). This file is the repo-side record.

Revision history, same day: rev 1 single-user Strava+Garmin with charts · rev 2 multi-user with
Strava sign-in and a ChatGPT-style shell · rev 3 Garmin only, multi-runner via a login gateway ·
rev 4: personal MVP for Leo first, Garmin only, with the seams to add other runners later,
Strava removed from product and code · **rev 5 (this): Architect and Planner pass. Token bundle
pasted into the app instead of a laptop import command (ADR 0003); read surface filtered in Go by
deny pattern instead of upstream's allowlist (ADR 0004); no database until Phase 7a and migrations
restart (ADR 0008); SSE keepalive; in-flight guard. Phases re-cut: build (5, 6, 7) first, deploy on free
tiers and test live (8) last.**

---

## 1. The MVP

A chat over Leo's own Garmin data, reachable from his phone, answering in prose with one chart
card when the pattern is the point. About three working weeks.

**In:** Garmin live on Leo's account; Strava gone; the full read surface of the Garmin MCP (~90
tools, writes denied); the five example questions answered from real data; one chart per reply
with "View data"; owner-password login; chats saved and listed in a plain sidebar; free hosting
(Render free + Neon free) so the phone can reach it; Garmin tokens imported from the laptop;
expansion seams (`runner_id` everywhere, auth and Garmin behind interfaces).

**After the MVP has been used (later):** streaming text deltas + Stop; dark theme; suggestions on
an empty chat; chat search and rename; Haiku 5.5 chat titles.

**Roadmap, designed for but not scheduled:** other runners via the MIT MissingMCP gateway
(OAuth client + per-runner Garmin sign-in there), bring-your-own MCP URL, tenancy tests,
privacy page, Garmin's developer program if it reopens.

Done when Leo asks a question from his phone and gets a grounded answer with a chart.

### Five example questions

1. Does my sleep actually affect my pace? Show me the pattern.
2. Am I overtraining right now?
3. How did my last three training blocks compare?
4. I have a half marathon in 10 weeks. Am I on track, given how recovered I've been?
5. Why has my body battery been so low this week?

### Requirements (numbered as in the artifact)

- R1 Grounded answers only, citing the runner's numbers and dates.
- R2 Full read surface of the Garmin MCP; write/delete/upload tools denied in the server env.
- R3 ≤1 chart per reply: line (1–2 series), bar, scatter; ≥5 points; never a dual axis.
- R4 Chart card: title, units, Garmin data used, model caption, "View data" table.
- R5 Chart never renders without its prose; invalid chart drops, prose still renders.
- R6 Streaming deltas + Stop (later). MVP: steps stream, reply arrives whole, as today.
- R7 One owner login (password hash in config), session cookie, works on the phone.
- R8 Garmin sign-in happens once on Leo's laptop (`garmin-mcp-auth`); the one-line base64 token
  bundle it writes (`~/.garminconnect_base64`) is pasted into the app's Connection screen, probed,
  and stored encrypted in Postgres. **The hosted service never signs in to Garmin; it only uses
  tokens, and the child process environment is built from scratch so it cannot.**
- R9 Token expiry (~6 months) surfaces in the chip, the reply and `/healthz` with the fix.
- R10–R13 Shell: sidebar by day with delete (search/rename later), suggestions (later), phone drawer,
  light theme (dark later), one Garmin chip (connected / tokens expired / unavailable).
- R14 Every table has `runner_id` from its first migration; the owner is row one; a scoped
  query helper is the only data access (test enforces).
- R15 `garmin.Source` interface: `LocalSpawn` now; `GatewayHTTP` and `BYOURL` later.
- R16 `auth.Provider` interface: `OwnerPassword` now; `GatewayOAuth` later. Routes never check
  "is this Leo".
- R17 `usage` + `daily_question_cap` per runner even with one runner (Leo's cost guard).
- R18 Free tiers only; the model is the only paid line.
- R19 Garmin endpoint changes → "Garmin unavailable", never a pretend answer.

---

## 2. Decisions

| # | Decision | Chosen |
|---|---|---|
| D1 | Scope | Personal MVP first, expansion-ready (13 Oct) |
| D2 | Tool surface | Every upstream read tool; writes denied by name pattern applied in Go to the live `tools/list` (ADR 0004); upstream's allowlist ignores unknown names silently |
| D3 | Look | ChatGPT layout with the RunCoach palette and type |
| D4 | Hosting | Free only: one Render free web service + Neon free Postgres (Fly has no free tier for new orgs; Render free has no disk → tokens in Postgres; Oracle Always Free VM as fallback / future gateway host) |
| D5 | Model | `claude-opus-5-5` for the coach, effort set explicitly (default is `medium`); `claude-haiku-5-5` for titles. Cache reads are $0.20/MTok on Opus and Sonnet alike, so the shared ~90-tool block costs the same; the 2x premium is on uncached input/output only |
| D6 | Other runners, later | MissingMCP gateway (MIT, OAuth 2.1 + DCR over unmodified garmin_mcp, per-user workers, sign-in throttle tuned vs Garmin's edge; ~2k users/wk hosted) → fork onto a free VM. Alternatives: official-API connectors the runner pays for (AthleteData $9/mo, Fitness AI $3/mo) as BYO URL; Garmin's Connect Developer Program (paused for new applicants since spring 2026) |

Settled earlier and kept: virtual `show_chart` tool (model reshapes data; backend validates;
SSE `chart` event); hand-rolled SVG charts; two-series cap because accent `#2E5D4E` and muted
`#6C6F67` fail categorical distinctness (ΔE 10.9 < 15) → identity by stroke form + direct labels.

### Why the server never signs in to Garmin

Garmin's SSO sits behind Cloudflare and rate-limits server-side sign-ins per IP (stock ~20
failures/h → day-long block); the mobile login flow has returned 429 for everyone since March
2026; a free host's shared egress gets burned first. So MFA login runs on Leo's laptop and the
bundle is pasted into the app (`POST /garmin/tokens`), which probes it and stores it encrypted.
Upstream falls back to an email/password sign-in when tokens fail and those variables exist, so
`LocalSpawn` builds the child env from scratch and a test proves `GARMIN_EMAIL` never reaches it.
Token refresh (OAuth2 from OAuth1) happens in memory per spawn and is not a sign-in.

---

## 3. Technical shape (after the MVP)

```
Leo (laptop/phone) ──HTTPS, owner session──▶ Vue 3 (chat shell, ChartCard)
                                               │ SSE
                                               ▼
   Go backend: auth middleware · runner-scoped queries · agent loop · show_chart
        │ Messages API, streaming, cached tool block           ──▶ Claude Opus 5.5
        │ stdio MCP per reply, local garmin-mcp, GARMINTOKENS=<private temp dir>, read surface only
        ▼
   Neon Postgres (free): runners(1) · sessions · garmin_connections(blob AES-GCM) · chats · usage · audit
   One image, one Render free web service, one origin (UI embedded in the Go binary).
   Laptop, every ~6 months: garmin-mcp-auth → paste ~/.garminconnect_base64 into /connection.
```

### Removing Strava (5.1)

Delete `cmd/strava-mcp`, `cmd/strava-check`, `internal/strava/`, `internal/api/strava_auth.go`
+ `/auth/strava/*`, `internal/mcpclient/strava.go` + `SourceStrava`, `STRAVA_*` config/env,
Strava guidance and multi-source selection rules in `agent.go`, `oauth_tokens` and
`activity_cache` tables (migration), every Strava mention in mock scenarios, step labels,
greeting, header, README, CLAUDE.md, spec. Keep the source-namespacing and `connect()`.

Keep: agent loop, `Observer` events, SSE writer, mock mode, stub MCP tests, Garmin Dockerfile
and compose (local), `ChatThread`, `StatusSteps`, `useChat`, answer parser.

### Tool surface

Deny by pattern, in Go (`mcpclient.ReadOnly`), applied to the live `tools/list`: `set_* add_*
create_* delete_* upload_* remove_* log_* update_* download_* request_reload
set_fit_download_dir` plus nutrition, challenges, gear, courses, workout builders/templates,
women's health. Exposed ≈ 90: activities (~16), sleep/recovery (~10),
daily health (~20), training/performance (~14), body (~6), profile/devices (~5), FIT (2).
Cheap and findable: prompt caching on system + tools; 12-line category map in the system
prompt; `return_timeseries` off; 8 tool turns; **live `tools/list` diff test** (three current
allowlist names don't exist upstream: `get_heart_rate_variability_summary`→`get_hrv_data`,
`get_vo2_max_trend`→`get_vo2max_trend`, `get_daily_stress`→`get_stress_data`).

### Data model

`runners(id, kind owner|garmin, garmin_profile_id null, display_name, daily_question_cap, …)`,
`sessions`, `garmin_connections(runner_id, kind local_tokens|gateway_token|byo_url, blob bytea,
key_version, status, connected_at, last_ok_at, expires_est_at, last_error)`,
`chat_sessions`/`chat_messages` + `runner_id` (chart spec in its own `chart jsonb`), `audit_log`,
`usage(runner_id, day, questions, input_tokens, cached_input_tokens, output_tokens)`.
Written as a fresh `000001` in Phase 7 (ADR 0008); Phases 5 and 6 need no database.

### `show_chart` (virtual tool, source `coach`)

`kind ∈ {line,bar,scatter}`, `title`, `x_label`, `y_label`, `x_unit`, `y_unit`, `data[]`
(activities, sleep, hrv, stress, body_battery, training_load, readiness, vo2max, steps, body),
`series[1..2]{label, points[5..120]{x,y}}`, `caption`. Bar/scatter one series; line ≤2 on one
unit; pace in s/km rendered m:ss; second call → readable tool error, first kept; invalid →
field named, no event, prose continues; persisted in `tool_calls`. `tool_choice` stays `auto`
(forced tool choice is rejected on Opus 5.5).

### Streaming (`POST /chats/:id/stream`)

MVP: `step` events then `chart` then `answer` (whole), as today plus `chart`. Later: `delta` and
`done`, Stop aborts and saves partial. Card appears with the first text, never before.

### API (MVP)

`POST /login` · `POST /logout` · `GET /me` (runner, Garmin state + fix text, usage/cap) ·
`POST/DELETE /garmin/tokens` (paste the bundle; probed before it is stored) ·
`GET/POST /chats` · `PATCH/DELETE /chats/:id` · `GET /chats/:id/messages` ·
`POST /chats/:id/stream` (history loaded server-side; 429 at the cap; 409 while a reply is in
flight; `: keepalive` every 15 s) · `GET /healthz` (db, Garmin state, expiry estimate, no secrets).
In production the Go binary serves the Vue build from the same origin (ADR 0007).
`RUNCOACH_MOCK=1` skips login, uses the owner row, replays canned steps + chart + answer.

### Hosting

Render free web service (sleeps after 15 idle min, ~1 min wake → waking state on the login
page; 750 h/month; no disk) + Neon free (0.5 GB, 100 CU-h/month, scales to zero). Fallback:
Oracle Always Free ARM VM (always on, disk; reclaimed <20 % util over 7 days → keepalive).
Claude ≈ $0.08–0.25/question on Opus 5.5 with caching, measured in 5.4; cap 30/day.

---

## 4. Phases

Build first, deploy and test last (`tasks/plan.md`). Phases 5–7 are code against unit tests, the
stub MCP server and mock mode; Phase 8 is free hosting plus the live pass.

| Phase | Outcome | Size |
|---|---|---|
| 5 Garmin core | Strava removed; database optional; pin `43b0c5b`; read surface filtered in Go; Garmin-only prompt on Opus 5.5 with explicit effort and caching | ≈ 2 d |
| 6 Charts | `show_chart`, SSE `chart`, prompt rules, `lib/chart.js` + `ChartCard.vue`, two mock scenarios | ≈ 3 d |
| 7 Shell | fresh migrations with `runner_id`, owner login, token paste, saved chats + cap + in-flight guard, sidebar + phone drawer, one origin with keepalive | ≈ 4 d |
| 8 Deploy and test | one image, Render free + Neon free, Garmin auth on the laptop and paste, live tool-diff test, five questions live from the phone and measured, Playwright smoke | ≈ 3 d |
| Later | streaming deltas + Stop, dark theme, suggestions, search/rename, Haiku titles; other runners via the MissingMCP gateway | when wanted |

## 5. Assumptions

1. Garmin remains reachable through garth / python-garminconnect for token-based calls; Garmin
   may block it; the chip says "unavailable" when it does.
2. Upstream `Taxuspt/garmin_mcp` stays the server; our client is transport-level.
3. One chart per reply, three kinds, native hover only.
4. Leo is the only user until he decides otherwise; no credential of anyone else is ever handled.
5. The product does not use Garmin's name as its own and states it is not affiliated.

## 6. Status

Decisions in `docs/adr/`, plan in `tasks/plan.md`. Phase 5 is being built on `phase-5-garmin-core`.
