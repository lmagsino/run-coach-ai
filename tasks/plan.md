# Implementation Plan: RunCoach AI, Garmin-only personal MVP

Planner stage of the AI Light Factory line. Decisions live in `docs/adr/`, terms in
`CONTEXT.md`, the product and technical design in
`docs/superpowers/specs/2026-10-13-garmin-mcp-upgrade-and-charts-design.md` (rev 5) and
the visual PRD/TRD/Design/Plan at https://claude.ai/artifact/MfXsK5cNaLNZGPENiA6zHt.

## Overview

Turn the mock-only Phase 1–4 codebase into a chat over Leo's own Garmin data, reachable
from his phone, that answers in grounded prose with one chart card when a pattern is the
point. Strava goes; the full read surface of the upstream Garmin MCP comes in; the coach
runs on Opus 5.5; the shell gets an owner login, saved chats and a sidebar; it is hosted on
free tiers. Every table and interface is shaped so a second runner is additive later.

The MVP is Phases 5, 6 and 7a. Phase 7b is polish after the MVP has been used. "Other
runners" is a roadmap section, designed for and not scheduled.

## How the phases are organised

Each phase has its own file in `tasks/`, its own branch, and ends with a live pass that
the Verifier can drive. Each task in a phase file is written in the shape the dev loop's
readiness gate expects (what to build, checkbox acceptance criteria, explicit
`Blocked by`), so any phase can be published as tickets with
`/mattpocock-skills:to-tickets tasks/phase-N.md` and run through `/alf:dev-loop` when
wanted. No tickets exist yet; this is a short project and the files are the plan.

| Phase | File | Branch | Blocked by | Outcome | Size |
|---|---|---|---|---|---|
| 5 | `tasks/phase-5.md` | `phase-5-garmin-live` | nothing | Garmin live on Leo's account, Strava gone, Opus 5.5, five questions answered and measured | ≈ 4 d |
| 6 | `tasks/phase-6.md` | `phase-6-charts` | Phase 5 merged | One chart card per reply, validated server-side, hand-rolled SVG | ≈ 5 d |
| 7a | `tasks/phase-7a.md` | `phase-7a-shell-hosting` | Phase 5 merged (not 6) | Owner login, token paste, saved chats and sidebar, one Render service + Neon, first question from the phone | ≈ 5 d |
| 7b | `tasks/phase-7b.md` | `phase-7b-polish` | 6 and 7a merged | Streaming deltas and Stop, dark theme, suggestions, search and rename, Haiku titles | ≈ 4 d |

Phases 6 and 7a are independent and may run as two lanes once Phase 5 is merged. Before
each phase there is a short "Step 0" of things only Leo can do (accounts, a Docker build,
a sign-up); they are listed at the top of each phase file and are not tasks for an agent.

## Architecture decisions (see `docs/adr/`)

- 0001 Garmin is the only source; Strava is deleted, not disabled.
- 0002 One runner now; `runner_id`, `auth.Provider`, `garmin.Source` and a per-runner cap from the first migration.
- 0003 The hosted server never signs in to Garmin; the token bundle is pasted into the app and stored encrypted; the child environment is built from scratch.
- 0004 Upstream garmin_mcp unmodified, spawned per reply; the read surface is filtered in Go by deny pattern.
- 0005 Charts are a virtual tool validated by the backend; SVG by hand; two series, one axis.
- 0006 Opus 5.5 with explicit effort and prompt caching; Haiku 5.5 for titles.
- 0007 One Render free web service serves API and UI from one origin; Neon Postgres.
- 0008 No database until Phase 7a; migrations restart at 000001.
- 0009 The MVP reply streams steps and chart, then arrives whole; deltas in 7b.

## Task list (index; details in the phase files)

### Phase 5: Garmin live, Strava out
- [ ] 5.1 Remove Strava from the backend
- [ ] 5.2 Remove Strava from the frontend and the docs
- [ ] 5.3 Database optional
- [ ] 5.4 Garmin client: pin bump, read-surface filter in Go
- [ ] 5.5 Live tool-diff test
- [ ] 5.6 Garmin-only prompt, Opus 5.5, effort, caching
- [ ] 5.7 Live pass: five questions, measured
- [ ] 5.8 Rough edges from the live pass

### Checkpoint: Phase 5 merged
- [ ] `go test -count=1 ./...` and `npm test` green; CI green
- [ ] `grep -ri strava` matches only `docs/adr/`, `PROGRESS.md` history and the spec's revision note
- [ ] `docs/live/phase-5.md` records the five questions, tools chosen, grounding, tokens and cost

### Phase 6: charts
- [ ] 6.1 `show_chart` spec and validator
- [ ] 6.2 Virtual `coach` source in the agent loop
- [ ] 6.3 SSE `chart` event and two mock scenarios
- [ ] 6.4 Prompt rules for charts and the live chart-rate test
- [ ] 6.5 `lib/chart.js` layout engine
- [ ] 6.6 `ChartCard.vue` with data table
- [ ] 6.7 Playwright smoke on mock mode in CI
- [ ] 6.8 Live pass with charts

### Checkpoint: Phase 6 merged
- [ ] Each of the five questions that should chart does, with a valid card; the others do not
- [ ] An invalid chart drops and the prose still renders (mock scenario proves it)

### Phase 7a: MVP shell and hosting
- [ ] 7a.1 Fresh migrations with `runner_id` everywhere and the scoped query helper
- [ ] 7a.2 `auth.Provider`, `OwnerPassword`, sessions, `/login` `/logout` `/me`
- [ ] 7a.3 `garmin.Source`, `LocalSpawn`, token crypto, `POST /garmin/tokens`
- [ ] 7a.4 Chat persistence, cap and in-flight guard
- [ ] 7a.5 Login and Connection screens
- [ ] 7a.6 Router, sidebar, chip, phone drawer
- [ ] 7a.7 One image: embedded UI, SSE keepalive, `/healthz`
- [ ] 7a.8 Render + Neon, first question from the phone

### Checkpoint: MVP
- [ ] Leo asks a question from his phone and gets a grounded reply with a chart
- [ ] Expired-token and Garmin-unavailable states show the fix text, never a pretend answer

### Phase 7b: polish after use
- [ ] 7b.1 Streaming deltas and Stop
- [ ] 7b.2 Dark theme
- [ ] 7b.3 Suggestions on an empty chat
- [ ] 7b.4 Chat search and rename
- [ ] 7b.5 Haiku 5.5 chat titles
- [ ] 7b.6 Playwright: login, sidebar, chart, in CI

### Roadmap: other runners (not scheduled, ≈ 7 d)
- R1 `GatewayOAuth` provider and `GatewayHTTP` source against the MissingMCP gateway; runner upsert by Garmin profile id (M)
- R2 `BYOURL` source: probe, store, tool-map adaptation (S)
- R3 Fork MissingMCP onto an Oracle Always Free VM with a keepalive (M)
- R4 Tenancy tests with two runners; delete account; privacy page stating the unofficial access; global unavailable notice (M)
- R5 Apply to Garmin's Connect Developer Program if it reopens (S)

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Garmin changes a private endpoint and upstream lags | High: replies fail | Pin a commit, bump deliberately; the coach says "Garmin unavailable" (R19); probe state on the chip and `/healthz` |
| The model picks wrong tools among ~90 or answers ungrounded | High: the product is wrong | Category map in the prompt; the Phase 5 live pass records every tool choice; the rough-edges task fixes the prompt before anything else is built |
| Cost per question higher than estimated | Medium | Measured in 5.7 with and without cache before the cap is set; effort dial; Sonnet 5.5 is a one-line fallback |
| Render's proxy drops long SSE streams | Medium: replies cut off | Keepalive comment every 15 s (7a.7); acceptance test holds an idle stream 60 s through Render |
| Token bundle format differs from what the plan assumes | Low | 5.5 records the exact file names the installed version writes; 7a.3 materialises those, not a guess |
| Garmin blocks token-based calls from a cloud IP | Medium | Nothing found says data calls are throttled, only sign-ins; the first hosted week shows it; fallback is the Oracle VM |
| Render free service wake time feels broken | Low | Login page shows a waking state (7a.5); accepted in ADR 0007 |

## Open questions (the gate)

None block Phase 5. Two are recorded as decided-by-default and Leo can overturn them at
the gate:

1. ADR 0003's paste box replaces the spec rev 4 `cmd/import-tokens` command. Default: paste.
2. Playwright on mock mode moves from 7b to the end of Phase 6 so the Verifier has browser
   legs for the first UI-visible milestone. Default: Phase 6.
