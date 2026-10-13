# Todo (ordered checklist; details in tasks/phase-*.md)

## Phase 5: Garmin live, Strava out (branch phase-5-garmin-live)
- [ ] Step 0 (Leo): Anthropic key; Garmin account with MFA ready; Docker running; `backend/.env` rebuilt from `.env.example`
- [ ] 5.1 Remove Strava from the backend
- [ ] 5.2 Remove Strava from the frontend and the docs
- [ ] 5.3 Database optional
- [ ] 5.4 Garmin client: pin bump, read-surface filter in Go
- [ ] Step 0b (Leo): `docker compose build` and `garmin-mcp-auth` with MFA
- [ ] 5.5 Live tool-diff test
- [ ] 5.6 Garmin-only prompt, Opus 5.5, effort, caching
- [ ] 5.7 Live pass: five questions, measured
- [ ] 5.8 Rough edges from the live pass
- [ ] Checkpoint: tests and CI green; no Strava left; `docs/live/phase-5.md` written; PR merged

## Phase 6: charts (branch phase-6-charts; needs Phase 5 merged)
- [ ] 6.1 `show_chart` spec and validator
- [ ] 6.2 Virtual `coach` source in the agent loop
- [ ] 6.3 SSE `chart` event and two mock scenarios
- [ ] 6.4 Prompt rules for charts and the live chart-rate test
- [ ] 6.5 `lib/chart.js` layout engine
- [ ] 6.6 `ChartCard.vue` with data table
- [ ] 6.7 Playwright smoke on mock mode in CI
- [ ] 6.8 Live pass with charts
- [ ] Checkpoint: charting questions chart, others do not; invalid chart drops; PR merged

## Phase 7a: MVP shell and hosting (branch phase-7a-shell-hosting; needs Phase 5 merged)
- [ ] Step 0 (Leo): Render and Neon accounts; `RUNCOACH_TOKEN_KEY` and the owner password hash generated
- [ ] 7a.1 Fresh migrations with `runner_id` everywhere and the scoped query helper
- [ ] 7a.2 `auth.Provider`, `OwnerPassword`, sessions, `/login` `/logout` `/me`
- [ ] 7a.3 `garmin.Source`, `LocalSpawn`, token crypto, `POST /garmin/tokens`
- [ ] 7a.4 Chat persistence, cap and in-flight guard
- [ ] 7a.5 Login and Connection screens
- [ ] 7a.6 Router, sidebar, chip, phone drawer
- [ ] 7a.7 One image: embedded UI, SSE keepalive, `/healthz`
- [ ] 7a.8 Render + Neon, first question from the phone
- [ ] Checkpoint MVP: a grounded reply with a chart on the phone; expiry state shows the fix text; PR merged

## Phase 7b: polish after use (branch phase-7b-polish; needs 6 and 7a merged)
- [ ] 7b.1 Streaming deltas and Stop
- [ ] 7b.2 Dark theme
- [ ] 7b.3 Suggestions on an empty chat
- [ ] 7b.4 Chat search and rename
- [ ] 7b.5 Haiku 5.5 chat titles
- [ ] 7b.6 Playwright: login, sidebar, chart, in CI
