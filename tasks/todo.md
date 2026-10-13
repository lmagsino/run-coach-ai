# Todo (details in tasks/plan.md)

## Phase 5: Garmin core (phase-5-garmin-core)
- [x] 5.1 Remove Strava from the backend
- [x] 5.2 Remove Strava from the frontend and docs
- [x] 5.3 Database optional
- [x] 5.4 Garmin client: pin bump, read-surface filter in Go, child env from scratch
- [x] 5.5 Garmin-only prompt, Opus 5.5, effort, caching
- [ ] Checkpoint: tests and CI green; mock mode Garmin-only; PR merged

## Phase 6: Charts (phase-6-charts)
- [ ] 6.1 `internal/chart` spec and validator
- [ ] 6.2 Virtual `coach` source in the agent
- [ ] 6.3 SSE `chart` event and mock scenarios
- [ ] 6.4 Prompt rules for charts
- [ ] 6.5 `lib/chart.js`
- [ ] 6.6 `ChartCard.vue`
- [ ] Checkpoint: both chart scenarios render; invalid drops; PR merged

## Phase 7: Shell (phase-7-shell)
- [ ] 7.1 Fresh migrations and the scoped store
- [ ] 7.2 Owner login and sessions
- [ ] 7.3 `garmin.Source`, `LocalSpawn`, token crypto, token paste
- [ ] 7.4 Saved chats, cap, in-flight guard
- [ ] 7.5 Login and Connection screens
- [ ] 7.6 Router, sidebar, chip, phone drawer
- [ ] 7.7 One origin, keepalive, `/healthz`
- [ ] Checkpoint: works locally with Postgres; mock mode needs none; PR merged

## Phase 8: Deploy (free) and test (phase-8-deploy)
- [ ] 8.1 Dockerfile and `render.yaml`
- [ ] 8.2 Neon + Render set up (Leo)
- [ ] 8.3 Garmin auth on the laptop, paste, live tool-diff test (Leo + code)
- [ ] 8.4 Live pass from the phone, measured
- [ ] 8.5 Rough edges, README
- [ ] 8.6 Playwright smoke in CI
- [ ] Done: grounded reply with a chart on the phone
