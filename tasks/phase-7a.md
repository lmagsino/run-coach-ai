# Phase 7a: MVP shell and hosting

Branch `phase-7a-shell-hosting` off `main`. Blocked by: Phase 5 merged (not Phase 6).
About five days. Can run in parallel with Phase 6.

Outcome: Leo opens a URL on his phone, logs in with one password, pastes his Garmin token
bundle once, and has saved chats in a sidebar. One Render free web service and one Neon
free database. Every table has `runner_id`; auth and Garmin access sit behind interfaces.

Decisions this phase builds on: ADR 0002, 0003, 0007, 0008. Screens: artifact Design tab
§2 desktop, §4 phone, §5 connection states, §6 owner login and token paste.

## Step 0 (Leo)

- A Render account (free) and a Neon account (free project, one database).
- Generate secrets locally and keep them in a password manager:
  `RUNCOACH_TOKEN_KEY` (32 random bytes, base64) and `RUNCOACH_OWNER_PASSWORD_HASH`
  (bcrypt of the chosen password; 7a.2 ships `go run ./cmd/hash-password`).
- A fresh local database for the new migrations (`dropdb runcoach_dev && createdb
  runcoach_dev`).

## Tasks

### Task 7a.1: Fresh migrations with `runner_id` everywhere and the scoped query helper

**Description:** Replace the Phase 2 migrations with one that carries the MVP data model
and the seams from ADR 0002.

**Acceptance criteria:**
- [ ] The old `000001_init` is deleted and a new `000001_init.up.sql` creates:
      `runners(id, kind owner|garmin, garmin_profile_id null unique, display_name,
      daily_question_cap int default 30, created_at)`;
      `sessions(id, runner_id, created_at, expires_at, last_seen_at)`;
      `garmin_connections(runner_id pk, kind local_tokens|gateway_token|byo_url, blob
      bytea, key_version int, status connected|expired|unavailable, imported_at,
      last_ok_at, expires_est_at, last_error)`;
      `chat_sessions(id, runner_id, title, created_at, updated_at, deleted_at)`;
      `chat_messages(id, chat_session_id, runner_id, role, content, tool_calls jsonb,
      chart jsonb, created_at)`;
      `usage(runner_id, day, questions, input_tokens, cached_input_tokens,
      output_tokens, primary key (runner_id, day))`;
      `audit_log(id, runner_id, action, at, detail jsonb)`;
      and inserts the owner as runner 1 with `kind = owner`. Every table with data has
      `runner_id` and an index on it. The down migration drops everything.
- [ ] `internal/store` exposes a `Scoped` type built from a runner id whose methods are
      the only way to run queries; a test scans `internal/` for `pool.Query`,
      `pool.Exec` and `QueryRow` outside `internal/store` and fails if any exist.
- [ ] `cmd/migrate up` on the fresh database succeeds; `down` returns it to empty.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/store ./internal/db`
- [ ] Manual check: `psql` shows the owner row.

**Blocked by:** none (Phase 5 merged)

**Files likely touched:** `backend/internal/db/migrations/000001_init.{up,down}.sql`,
`backend/internal/store/store.go` (new) and test, `backend/internal/db/db.go`.

**Estimated scope:** M

### Task 7a.2: `auth.Provider`, `OwnerPassword`, sessions, `/login` `/logout` `/me`

**Description:** One owner password, a cookie session, and a route layer that never asks
who the owner is.

**Acceptance criteria:**
- [ ] `internal/auth` defines `Provider` (`Authenticate(ctx, credentials) (runnerID,
      error)` and the doc naming `GatewayOAuth` as the roadmap implementation) and
      `OwnerPassword` reading `RUNCOACH_OWNER_PASSWORD_HASH` (bcrypt) and returning
      runner 1.
- [ ] `POST /login {password}` sets an `HttpOnly; Secure; SameSite=Lax` cookie holding a
      32-byte random session id stored in `sessions` with a 30-day expiry;
      `POST /logout` deletes it; `GET /me` returns `{runner: {id, display_name}, garmin:
      {status, fix}, usage: {today, cap}}`.
- [ ] Middleware resolves the session to a runner id on every route except `/login` and
      `/healthz`, and rejects the rest with 401; the chat routes read the runner id from
      the context only.
- [ ] Login is rate-limited to 5 attempts per minute per IP (in memory is fine); a wrong
      password returns 401 with no hint.
- [ ] `go run ./cmd/hash-password` prints a bcrypt hash for a password read from stdin.
- [ ] `RUNCOACH_MOCK=1` skips login and uses runner 1, as the spec says.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/auth ./internal/api`
- [ ] Manual check: `curl` login, `/me`, a 401 without the cookie.

**Blocked by:** 7a.1

**Files likely touched:** `backend/internal/auth/*.go` (new), `backend/internal/api/auth.go`
(new), `backend/internal/api/api.go`, `backend/internal/api/middleware.go` (new),
`backend/cmd/hash-password/main.go` (new), `backend/internal/config/config.go`,
`backend/.env.example`.

**Estimated scope:** M

### Task 7a.3: `garmin.Source`, `LocalSpawn`, token crypto, `POST /garmin/tokens`

**Description:** Garmin access behind an interface, with the token bundle stored
encrypted and materialised only for one spawn (ADR 0003, 0004).

**Acceptance criteria:**
- [ ] `internal/garmin` defines `Source` (`Connect(ctx, runnerID) (*mcp.ClientSession,
      func(), error)` and `Probe(ctx, runnerID) error`, doc naming `GatewayHTTP` and
      `BYOURL` as roadmap implementations) and `LocalSpawn` which: loads the runner's
      `garmin_connections` row, decrypts the blob, writes the files `garmin-mcp-auth`
      produces (names from `docs/live/phase-5.md` "Tool facts") into a `0700` temp dir,
      spawns `garmin-mcp` with an environment built from scratch (`PATH`, `HOME`,
      `GARMINTOKENS=<dir>`), and removes the dir in the returned close func. A test
      asserts `GARMIN_EMAIL` and `GARMIN_PASSWORD` never reach the child.
- [ ] `internal/crypto` seals and opens with AES-256-GCM under `RUNCOACH_TOKEN_KEY`,
      prefixing a key version; round-trip and wrong-key tests.
- [ ] `POST /garmin/tokens {bundle}` decodes the base64 bundle, runs `Probe` against a
      spawn using it, and on success stores it encrypted with `status connected`,
      `imported_at now`, `expires_est_at now + 6 months`, `last_ok_at now`; on failure
      returns 422 with the probe error and stores nothing. `DELETE /garmin/tokens`
      removes the row. Both write `audit_log`.
- [ ] After each reply the row's `last_ok_at` is updated on success; an authentication
      error from the child flips `status` to `expired` with `last_error`; a connection or
      HTTP error flips it to `unavailable`. `/me` maps status to the chip and a `fix`
      sentence ("Run garmin-mcp-auth on your laptop and paste the new bundle").
- [ ] `GARMIN_MCP_COMMAND` is gone; local dev points `RUNCOACH_GARMIN_MCP_BIN` at the
      installed `garmin-mcp` (default `garmin-mcp` on `PATH`); the Docker compose
      profile remains only for `garmin-mcp-auth` on the laptop, and its README says so.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/garmin ./internal/crypto ./internal/api`
- [ ] Manual check: paste the bundle locally, see `connected`; paste garbage, see 422.

**Blocked by:** 7a.1

**Files likely touched:** `backend/internal/garmin/*.go` (new, absorbs
`internal/mcpclient/garmin.go`), `backend/internal/crypto/*.go` (new),
`backend/internal/api/garmin.go` (new), `backend/internal/api/chat.go`,
`backend/internal/config/config.go`, `backend/.env.example`, `deploy/garmin-mcp/README.md`.

**Estimated scope:** M

### Task 7a.4: Chat persistence, cap and in-flight guard

**Description:** Chats live in Postgres, history is loaded server-side, and two guards
bound cost.

**Acceptance criteria:**
- [ ] `GET /chats` (by day, newest first, excluding deleted), `POST /chats` (title = first
      question, set on the first message), `PATCH /chats/:id {title}`, `DELETE /chats/:id`
      (soft), `GET /chats/:id/messages`; all through `store.Scoped`; a chat of another
      runner id is a 404.
- [ ] `POST /chats/:id/stream` replaces `POST /chat/stream`: the server loads the prior
      turns from the database (last 6, final text only, as `AnswerInConversation`
      expects) and the client no longer sends history; it persists the user message,
      then the reply with `tool_calls` and `chart`.
- [ ] Cap: the stream returns 429 with "Daily cap reached (30). Resets at midnight UTC"
      when `usage.questions >= daily_question_cap`; a successful reply increments
      `questions` and adds the token counts from the API response.
- [ ] In-flight guard: a second stream for the same runner while one is running returns
      409 "A reply is already in progress"; released on completion or client disconnect.
- [ ] Mock mode keeps working with an in-memory store so the frontend and Playwright
      need no database.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/api ./internal/store`
- [ ] Manual check: two chats, a follow-up that references the first turn, a 429 after
      lowering the cap to 1.

**Blocked by:** 7a.2

**Files likely touched:** `backend/internal/api/chats.go` (new), `backend/internal/api/chat.go`,
`backend/internal/store/*.go`, `backend/internal/mockchat/*.go`.

**Estimated scope:** M

### Task 7a.5: Login and Connection screens

**Description:** The two screens that exist only because the app is hosted.

**Acceptance criteria:**
- [ ] `/login`: one password field, "Sign in"; while `/healthz` is unreachable the page
      shows "Waking up the coach, about a minute" and retries every 5 s (ADR 0007); a
      wrong password shows one line and keeps the field.
- [ ] `/connection`: the Garmin chip state in words, the paste box for the token bundle
      with the three laptop commands shown exactly as in the Design tab §6, a "Save"
      that calls `POST /garmin/tokens` and shows the probe error verbatim on 422, and a
      "Remove" that calls `DELETE`. After a successful save the chip reads "Garmin
      connected · tokens expire about <date>".
- [ ] Both screens work at 360 px.

**Verification:**
- [ ] Tests pass: `cd frontend && npm test` (component tests for the waking state and
      the 422 path with a mocked transport).
- [ ] Manual check: screenshots at 1280 and 360 px.

**Blocked by:** 7a.2, 7a.3

**Files likely touched:** `frontend/src/pages/Login.vue`, `frontend/src/pages/Connection.vue`
(new), `frontend/src/lib/api.js`, `frontend/src/lib/useSession.js` (new).

**Estimated scope:** M

### Task 7a.6: Router, sidebar, chip, phone drawer

**Description:** The ChatGPT-style shell in the Field Notes palette (Design tab §2 and §4),
built around the existing thread, steps and composer.

**Acceptance criteria:**
- [ ] `vue-router` with `/login`, `/`, `/c/:id`, `/connection`; unauthenticated routes
      redirect to `/login` (except in mock mode).
- [ ] Sidebar: "New chat", chats grouped by day from `GET /chats`, the current one
      marked, delete with a two-step confirm, the Garmin chip from `/me` at the bottom
      linking to `/connection`, the usage line "n of 30 today".
- [ ] Phone (below the `wide:` breakpoint): the sidebar is a drawer opened from the
      header, closes on navigation, traps focus, and the composer stays above the
      keyboard.
- [ ] The thread, `StatusSteps`, `AgentTurn`, `UserTurn` and `Composer` are reused
      unchanged except for reading from the new `POST /chats/:id/stream` and no longer
      sending history; the reply still arrives whole on `answer` (ADR 0009).
- [ ] An empty chat shows the greeting only (suggestions are 7b).

**Verification:**
- [ ] Tests pass: `cd frontend && npm test`
- [ ] Manual check: screenshots at 1280, 561, 360 px; keyboard navigation of the drawer.

**Blocked by:** 7a.4, 7a.5

**Files likely touched:** `frontend/src/App.vue`, `frontend/src/router.js` (new),
`frontend/src/components/Sidebar.vue`, `frontend/src/components/Drawer.vue`,
`frontend/src/components/GarminChip.vue` (new), `frontend/src/components/AppHeader.vue`,
`frontend/src/lib/useChat.js`, `frontend/src/lib/api.js`, `frontend/package.json`.

**Estimated scope:** M

### Task 7a.7: One image: embedded UI, SSE keepalive, `/healthz`

**Description:** One deployable artefact that serves everything from one origin.

**Acceptance criteria:**
- [ ] The Go binary serves `frontend/dist` through `embed.FS` at `/` with history-mode
      fallback to `index.html`, API routes under `/api/` (frontend `BASE` updated), and
      no CORS headers unless `ALLOWED_ORIGIN` is set (dev only).
- [ ] `Dockerfile` at the repo root: stage 1 `node:22` builds the frontend; stage 2
      `golang:1.26` builds the server with the dist embedded; stage 3 `python:3.12-slim`
      installs `garmin-mcp` at the pinned commit with `uv` and copies the binary; the
      image runs as a non-root user; `docker build` succeeds under 5 minutes and the image
      is under 400 MB.
- [ ] The SSE writer sends a `: keepalive` comment every 15 s while a reply is running;
      a test uses a slow scripted model to prove one is emitted.
- [ ] `GET /healthz` returns `{status, db: up|off, garmin: connected|expired|unavailable|
      none, tokens_expire_est}` with no secrets, cheap enough for Render's health check
      (no Garmin call; it reads the row).
- [ ] `deploy/garmin-mcp/docker-compose.yml` keeps only the `auth` profile, documented
      as the laptop sign-in step.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./...`
- [ ] Build succeeds: `docker build -t runcoach .` then `docker run -e RUNCOACH_MOCK=1 -p
      8080:8080 runcoach` serves the UI and answers a mock question.

**Blocked by:** 7a.3

**Files likely touched:** `Dockerfile` (new), `.dockerignore`, `backend/internal/api/static.go`
(new), `backend/internal/api/sse.go` and test, `backend/internal/api/health.go`,
`backend/cmd/server/main.go`, `frontend/src/lib/api.js`, `frontend/vite.config.js`,
`deploy/garmin-mcp/docker-compose.yml`.

**Estimated scope:** M

### Task 7a.8: Render + Neon, first question from the phone

**Description:** Deploy, connect, ask. Mostly Leo's hands.

**Acceptance criteria:**
- [ ] `render.yaml` describes one free web service built from the root Dockerfile with
      health check `/healthz` and the env vars listed (values set in the dashboard, never
      committed): `DATABASE_URL`, `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`,
      `ANTHROPIC_EFFORT`, `RUNCOACH_TOKEN_KEY`, `RUNCOACH_OWNER_PASSWORD_HASH`.
- [ ] Migrations run on deploy (`cmd/migrate up` as the container's pre-start step, or
      on server start with an advisory lock; pick one and record it in the README).
- [ ] From the phone: login, paste the bundle, ask "Why has my body battery been so low
      this week?", get a grounded reply; the wall time and the first-request wake time
      recorded in `docs/live/phase-7a.md` with a screenshot.
- [ ] An idle stream of 60 s stays open through Render (keepalive proven in production).
- [ ] `README.md` has a "Deploy" section and the two-command token refresh.

**Verification:**
- [ ] Manual check: `docs/live/phase-7a.md` complete; `/healthz` on the public URL shows
      `garmin: connected`.

**Blocked by:** 7a.4, 7a.6, 7a.7, Step 0

**Files likely touched:** `render.yaml` (new), `README.md`, `docs/live/phase-7a.md` (new).

**Estimated scope:** S (plus Leo's time)

## Checkpoint: MVP

- [ ] Backend, frontend and Playwright suites green; CI green.
- [ ] Leo asks a question from his phone and gets a grounded reply (with a chart once
      Phase 6 is merged).
- [ ] Removing the token row and asking again shows the "tokens expired" fix text; no
      pretend answer.
- [ ] PR `phase-7a-shell-hosting` → `main` merged by Leo.

## What the Verifier drives

Three states on the deployed URL: no token bundle (chip "not connected", reply refuses with
the fix), a valid bundle (grounded reply), the cap reached (429 surfaced in the composer).
Mock mode through Playwright for login redirect and sidebar CRUD.
