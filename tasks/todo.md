# Todo (details in tasks/plan.md)

## Phase 5: Garmin core (phase-5-garmin-core)
- [x] 5.1 Remove Strava from the backend
- [x] 5.2 Remove Strava from the frontend and docs
- [x] 5.3 Database optional
- [x] 5.4 Garmin client: pin bump, read-surface filter in Go, child env from scratch
- [x] 5.5 Garmin-only prompt, Opus 5.5, effort, caching
- [x] Checkpoint: tests green; mock mode Garmin-only; browser walkthrough passes (PR merge is Leo's)

## Phase 6: Charts (phase-6-charts)
- [x] 6.1 `internal/chart` spec and validator
- [x] 6.2 Virtual `coach` source in the agent
- [x] 6.3 SSE `chart` event and mock scenarios
- [x] 6.4 Prompt rules for charts
- [x] 6.5 `lib/chart.js`
- [x] 6.6 `ChartCard.vue`
- [x] Checkpoint: both chart scenarios render at 1280/561/360; invalid drops (PR merge is Leo's)

## Phase 7: Shell (phase-7-shell)
- [x] 7.1 Fresh migrations and the scoped store
- [x] 7.2 Owner login and sessions
- [x] 7.3 `garmin.Source`, `LocalSpawn`, token crypto, token paste
- [x] 7.4 Saved chats, cap, in-flight guard
- [x] 7.5 Login and Connection screens
- [x] 7.6 Router, sidebar, chip, phone drawer
- [x] 7.7 One origin, keepalive, `/healthz`
- [x] Checkpoint: store suite green on Postgres; shell walkthrough green in mock mode (PR merge is Leo's)

## Phase 8: Deploy (free) and test (phase-8-deploy)
- [x] 8.1 Dockerfile and `render.yaml`
- [ ] 8.2 Neon + Render set up (Leo)
- [ ] 8.3 Live tool-diff test (`RUNCOACH_LIVE_GARMIN_TESTS=1`), and a first Garmin sign-in from Render (Leo + code)
- [ ] 8.7 Garmin sign-in with email and password: backend done (12c3850); account page form, mock walkthrough step, ADR 0011, README guide still to do
- [ ] 8.4 Live pass from the phone, measured
- [ ] 8.5 Rough edges, README
- [x] 8.6 Playwright smoke in CI (both walkthroughs via bin/dev) and an image build on every push
- [ ] Done: grounded reply with a chart on the phone

## Phase 9: Accounts (phase-9-accounts; needs Phase 7 on main)
- [x] 9.1 Accounts in the store and email sign-in (migration 000002, store methods, `auth.EmailPassword`, owner from env)
- [x] 9.2 Sign-up by invite code, SignUpPage, email on LoginPage
- [x] 9.3 Tenancy proof: two-runner API test, `e2e/accounts.mjs`
- [x] 9.4 Account endpoints: name, password (revokes sessions), delete (hard), `cmd/set-password`
- [x] 9.5 Account page with Profile, Password, Garmin, Usage, Delete; sidebar name
- [x] 9.6 Member Garmin onboarding: uvx one-liner, published auth image via GitHub Actions
- [x] 9.7 Cost guards: member cap, global daily cap, runner id in the reply log
- [x] 9.8 Docs: ADR 0010, CONTEXT.md, README "For other runners", env example, render.yaml
- [x] Checkpoint: two accounts isolated locally; sign-up gated; caps enforced; walkthroughs green (merged straight to main)
