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
| 9 Accounts | `phase-9-accounts` | sign up by invite, sign in with email, account settings, Garmin per runner by a pasted bundle, cost guards, tenancy proof | ≈ 4 d |

Later, after use: streaming deltas and Stop, dark theme, suggestions, search and rename,
Haiku titles. Roadmap: a hosted sign-in gateway for runners who cannot make a bundle
themselves (ADR 0002, 0010).

## Phase 5: Garmin core

- [x] 5.1 Remove Strava from the backend: `cmd/strava-mcp`, `cmd/strava-check`,
      `internal/strava`, `strava_auth.go`, `mcpclient/strava.go`, `Strava*` config, the
      multi-source prompt rules, the Strava migration and `activity_cache`; mock scenarios
      on Garmin tools only. Done when `grep -ri strava backend/` is empty and tests pass.
- [x] 5.2 Remove Strava from the frontend and docs: step labels, greeting, header, composer
      hint; README, CLAUDE.md, feature spec, `.env.example`. Done when only ADR 0001,
      PROGRESS history and the spec's revision note mention it.
- [x] 5.3 Database optional: the server starts without `DATABASE_URL`; `/healthz` says
      `db: off` (ADR 0008).
- [x] 5.4 Garmin client: pin `GARMIN_MCP_REF=43b0c5b375fc1c7bd6bdbead69968dfabddec265`;
      `deploy/garmin-mcp/tools.txt` with the 155 upstream names; `mcpclient.ReadOnly(name)`
      deny pattern replaces `DefaultGarminTools`; `agent.buildTools` applies it; the child
      environment is built from scratch (no `GARMIN_EMAIL`/`GARMIN_PASSWORD`, ADR 0003);
      stub test proves a `delete_*` tool never reaches the model.
- [x] 5.5 Prompt and model: Garmin-only system prompt with a twelve-line category map
      checked against `tools.txt`; `ANTHROPIC_MODEL` default `claude-opus-5-5`;
      `ANTHROPIC_EFFORT` (default `medium`) sent on every request; `cache_control` on the
      system prompt and the last tool, asserted by the scripted-model tests (ADR 0006).
- [ ] Checkpoint: `go test ./...`, `npm test`, CI green; mock mode answers the five
      questions with Garmin-only steps. PR to `main`.

## Phase 6: Charts

- [x] 6.1 `internal/chart`: `Spec` (kind line|bar|scatter, title, labels, units, data name,
      1–2 series of 5–120 points, caption, optional `partial` on a point), `Validate` naming
      the failing field, `ToolDefinition()` (ADR 0005).
- [x] 6.2 Agent: built-in source `coach` with `show_chart`; valid call → `Result.Chart`
      and an `Event{Kind: chart}`; second call refused readably; invalid call returns the
      error and no event; `tool_choice` stays `auto`.
- [x] 6.3 API: SSE `chart` event after the steps and before `answer`; `chart` field on
      `POST /chat`; two mock scenarios with charts (sleep vs pace scatter, training-block
      bars with a partial week) and one with an invalid spec that must drop.
- [x] 6.4 Prompt: a "Charts" section (one at most, patterns and relationships only, never a
      single number, reshape yourself, pace in s/km, two series only on one unit).
- [x] 6.5 `frontend/src/lib/chart.js`: pure layout (nice ticks, scales, m:ss pace, trend
      line, dashed second series with direct labels, faint partial bars) with vitest.
- [x] 6.6 `ChartCard.vue`: SVG in the Field Notes tokens, title, Garmin data used, caption,
      "View data" table with Copy; shown above the prose only once `answer` arrives
      (ADR 0009); 320 px up; reduced motion respected.
- [ ] Checkpoint: tests and CI green; both chart scenarios render in mock mode at 1280,
      561 and 360 px; the invalid one shows prose only. PR to `main`.

## Phase 7: Shell

- [x] 7.1 Fresh `000001` migration: `runners` (owner seeded), `sessions`,
      `garmin_connections`, `chat_sessions`, `chat_messages`, `usage`, `audit_log`, all
      with `runner_id`; `internal/store.Scoped` is the only data access, with a test that
      scans for stray `pool.Query` calls (ADR 0002, 0008).
- [x] 7.2 Auth: `auth.Provider` + `OwnerPassword` (bcrypt hash from
      `RUNCOACH_OWNER_PASSWORD_HASH`); cookie session (`HttpOnly; Secure; SameSite=Lax`,
      30 days); `POST /login`, `POST /logout`, `GET /me`; login rate-limited;
      `cmd/hash-password`; mock mode skips login.
- [x] 7.3 Garmin: `garmin.Source` + `LocalSpawn` (decrypt the bundle into a `0700` temp
      dir, spawn with `GARMINTOKENS`, delete on close); `internal/crypto` AES-256-GCM
      under `RUNCOACH_TOKEN_KEY` with `key_version`; `POST /garmin/tokens` probes the
      pasted bundle with `get_user_profile` before storing; `DELETE /garmin/tokens`;
      status connected / expired / unavailable with a `fix` sentence on `/me` (ADR 0003).
- [x] 7.4 Chats: `GET/POST /chats`, `PATCH/DELETE /chats/:id`, `GET /chats/:id/messages`,
      `POST /chats/:id/stream` with history loaded server-side; 429 at the daily cap; 409
      while a reply is running; usage from the API's token counts; mock mode uses an
      in-memory store.
- [x] 7.5 Screens: `/login` with a "waking up" state while `/healthz` is unreachable;
      `/connection` with the chip state, the laptop commands and the paste box.
- [x] 7.6 Shell: `vue-router` (`/login`, `/`, `/c/:id`, `/connection`); sidebar by day
      with new and delete, the Garmin chip and the usage line; drawer below the `wide:`
      breakpoint; thread, steps, composer reused.
- [x] 7.7 One origin: the Go binary serves `frontend/dist` through `embed.FS`, API under
      `/api/`, CORS only when `ALLOWED_ORIGIN` is set; SSE `: keepalive` every 15 s;
      `/healthz` reports db and Garmin state with no secrets (ADR 0007).
- [ ] Checkpoint: tests and CI green; login, paste (against the local Docker image), two
      chats and a follow-up work locally with Postgres; mock mode still needs none.
      PR to `main`.

## Phase 8: Deploy (free) and test

- [x] 8.1 Image: root `Dockerfile` (node build → go build with the dist embedded →
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

## Phase 9: Accounts (planned 14 Oct 2026)

Other runners get their own account, their own chats and their own Garmin connection.
Garmin is connected the way it is today: each runner makes a token bundle once on their
own computer and pastes it; the server never signs in to Garmin for anyone (ADR 0003, and
the 14 Oct research: datacenter sign-ins fail). Sign-up is by invite code, because every
reply is billed to one API key; caps bound the spend per runner and per day.

Decisions this phase adds (ADR 0010): accounts are email + password with no email
verification (the invite code is the gate; the owner resets a password from the CLI);
members get a lower default cap and a global daily cap sits above everyone; a deleted
account is a hard delete that takes its chats, usage and bundle with it.

Dependency graph: 9.1 (accounts in the store, email login) → 9.2 (sign-up) → 9.3 (tenancy
proof) → 9.4 (account endpoints) → 9.5 (account page) → 9.7 (caps). 9.6 (member Garmin
onboarding) depends only on 9.2. 9.8 (docs) last.

- [x] 9.1 Accounts in the store and email sign-in: migration `000002_members` adds
      `email` (unique, nullable), `password_hash`, `status active|disabled`,
      `last_login_at` to `runners` and widens `kind` to `owner|member|garmin`; the store
      gains `RunnerByID`, `RunnerByEmail`, `CreateRunner`, `UpdateRunner`, `DeleteRunner`,
      `DeleteSessionsFor`, `GlobalUsage`, on both implementations with the shared suite;
      `auth.EmailPassword` replaces `OwnerPassword` for `POST /api/login {email, password}`
      (an empty email still means the owner, so today's login keeps working); the owner's
      email and hash are upserted on start from `RUNCOACH_OWNER_EMAIL` and the existing
      hash; `/api/me`, `checkCap` and the mock middleware read the runner's own row, not
      `Owner()`. Done when a runner row with an email can sign in and `/api/me` shows its
      own name and cap. M.
- [x] 9.2 Sign-up: `POST /api/signup {email, display_name, password, invite_code}` creates
      a `member` with the member cap, sets the session cookie, audits `sign_up`; refused
      with 403 when the code does not match `RUNCOACH_INVITE_CODE` (and when that is
      unset: sign-up closed), 409 on a used email, 422 on a short password (10+) or a bad
      email; rate-limited like login. `SignUpPage.vue` (email, name, password, invite code,
      one error line); `LoginPage.vue` gains the email field and a "Create account" link.
      Done when a second person can create an account and land in an empty chat. M.
- [x] 9.3 Tenancy proof: an API test signs up two runners and asserts each sees only its
      own chats, messages, Garmin row and usage, and gets 404 for the other's chat id; a
      browser walkthrough `frontend/e2e/accounts.mjs` against a non-mock in-memory server
      (no API key needed: it stops at the chat) covers sign-up, sign-out, sign-in, and
      the second account seeing an empty sidebar. S.
- [x] 9.4 Account endpoints: `PATCH /api/account {display_name}`;
      `POST /api/account/password {current, new}` (revokes every other session);
      `DELETE /api/account {password}` (hard delete, cascades to chats, messages, usage,
      sessions and the Garmin row; the owner cannot delete itself); `cmd/set-password
      <email>` for the owner to reset a member offline. Each audited. S.
- [x] 9.5 Account page: `/account` with sections Profile (name), Password, Garmin (the
      current Connection content), Usage (today of cap), Delete account (type the password);
      `/connection` redirects to `/account#garmin`; the sidebar shows the display name and
      links to the page; `useSession` carries `kind` and `display_name`. M.
- [x] 9.6 Member Garmin onboarding: the Garmin section shows a member the two ways to make
      a bundle without cloning the repo: `uvx --from "garmin-mcp @ git+https://github.com/
      Taxuspt/garmin_mcp@<pin>" garmin-mcp-auth` then `cat ~/.garminconnect_base64 | pbcopy`,
      or Docker with a published image; a GitHub Actions workflow builds
      `deploy/garmin-mcp` and pushes `ghcr.io/lmagsino/garmin-mcp-auth:<pin>` on changes to
      that directory; the owner keeps the start-up import. Done when a member with no clone
      can connect from the instructions on the page. S.
- [x] 9.7 Cost guards: members default to `RUNCOACH_MEMBER_CAP` (10) questions a day, the
      owner keeps 30; `RUNCOACH_GLOBAL_DAILY_CAP` (100) across all runners returns 429 "the
      coach has done its questions for today" before any model call; the reply log line
      carries the runner id; sign-up disabled when the global cap is unset to zero. S.
- [x] 9.8 Docs: ADR 0010, CONTEXT.md (member, invite code, global cap), README "For other
      runners", `.env.example` and `render.yaml` (owner email, invite code, caps), PROGRESS. S.
- [ ] Checkpoint: Go, vitest and both walkthroughs green; on the local server two accounts
      each with their own chats and Garmin state; sign-up refused without the code; member
      capped at 10; a deleted account leaves no rows. PR to `main`.

Out of scope, on purpose: server-side Garmin sign-in for members (ADR 0003), email
verification and password-reset mail (no mail service on free tiers; the invite code and
the CLI reset cover a handful of invited runners), an admin page (the audit log and the
reply log are enough for now), OAuth sign-in.

## Risks

| Risk | Where it is caught |
|---|---|
| Garmin changes a private endpoint and upstream lags | pinned commit; "Garmin unavailable" state; probe on the chip and `/healthz` |
| The model picks wrong tools among ~90 or answers ungrounded | category map; 8.4 records every choice; 8.5 fixes the prompt |
| Cost per question above the estimate | 8.4 measures with and without cache; effort dial; Sonnet 5.5 one line away |
| Render's proxy drops long SSE streams | keepalive in 7.7; proven in 8.4 |
| Token bundle file names differ from the assumption | 8.3 records them before 7.3's spawn is trusted live; the probe fails loudly otherwise |
| Invited runners spend the owner's API credit | invite code, member cap, global daily cap, usage per runner in the log (9.2, 9.7) |
| A member cannot make a bundle on their own computer | the page gives a `uvx` one-liner and a published Docker image (9.6); the gateway stays the roadmap answer for everyone else |
| Holding other people's Garmin tokens | encrypted at rest, never a password, deleted with the account (9.4); the privacy line on the page says what is held |
