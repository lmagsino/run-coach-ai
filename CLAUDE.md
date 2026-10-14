# RunCoach AI — Project Guide (CLAUDE.md)

Context for AI coding sessions. Read this first.

## What this is
A chat over one runner's Garmin data. The Go backend is an MCP client to the upstream
`Taxuspt/garmin_mcp` server; Claude runs a tool loop over its read-only tools and answers in
grounded prose, with one chart card when a pattern is the point (Phase 6). Chat *is* the
product. Built for Leo first, with the seams for other runners later.

## Planning artefacts (read before changing anything)

| Stage | Where |
|---|---|
| Decisions | `docs/adr/0001`–`0009` (Garmin only; one runner shaped for many; the server never signs in to Garmin; read surface filtered in Go; charts as a tool; Opus 5.5 with caching; free hosting on one origin; no database until Phase 7; reply arrives whole) |
| Terms | `CONTEXT.md` (runner, owner, token bundle, read surface, deny pattern, live pass, …) |
| Design | `docs/superpowers/specs/2026-10-13-garmin-mcp-upgrade-and-charts-design.md` and the artifact linked at its top |
| Plan | `tasks/plan.md` (phases 5–8 with their tasks and done-when lines), `tasks/todo.md` (checklist) |
| History | `running-agent-feature-spec.md` (the original two-source spec, superseded), `PROGRESS.md` (Phases 1–4) |

Where this file and an ADR disagree, the ADR wins.

## Repo layout (monorepo)
```
run-coach-ai/
├── CONTEXT.md, docs/adr/, tasks/      # glossary, decisions, plan
├── DESIGN.md, design/                 # "Field Notes" UI direction (locked)
├── deploy/garmin-mcp/                 # upstream garmin_mcp image, one-off auth, tools.txt
├── backend/                           # Go API server
└── frontend/                          # Vue 3 app
```

## Working conventions
- **Commit gradually and often.** Small, frequent commits after each working, self-contained
  chunk. Conventional-ish subjects (`feat:`, `fix:`, `chore:`, `docs:`), imperative mood.
- Each commit should build and pass on its own where practical, and do one thing.
- Never commit secrets. `.env` is gitignored; keep `backend/.env.example` in sync when adding
  an env var.
- Tick items off in `tasks/todo.md` as they complete.
- One branch per phase (`phase-5-garmin-core`, `phase-6-charts`, `phase-7-shell`,
  `phase-8-deploy`), one PR to `main` each.
- Git identity in this repo is the personal account (`leo.magsino819@gmail.com`).
- **Run it with `bin/dev`, never a binary you build and launch yourself.** `bin/dev` runs
  the server (`go run`) and the UI (Vite) in the foreground, like ShiftCare's; restart by
  stopping it and running it again. Scratch files go in the repo's ignored `tmp/`. Never `go build -o` a server and start it
  from a script, and never put or launch anything in `/tmp`: on the work laptop
  CrowdStrike blocked exactly that as malware (a compiled server in `/tmp` started by a
  shell script in `/tmp`), while `go run` has never been flagged. Don't run our own server
  inside Docker either: Docker Desktop's Linux VM is a blind spot for the laptop's security
  tool, so it would look like working around it. Docker is only for the third-party Garmin
  tool. The image build for hosting happens on Render, not here.

## Backend (Go)

**Location:** `backend/`, `go.mod` lives here; run all Go commands from this dir.
**Module path:** `github.com/lmagsino/run-coach-ai/backend`. **Go:** 1.26 (`/opt/homebrew/bin/go`).

### Structure
```
backend/
├── cmd/
│   ├── server/         # HTTP API under /api plus the embedded UI
│   ├── migrate/        # DB migration runner (needs DATABASE_URL)
│   ├── hash-password/  # bcrypt hash for RUNCOACH_OWNER_PASSWORD_HASH
│   └── token-key/      # a fresh RUNCOACH_TOKEN_KEY
├── internal/
│   ├── config/       # env-based configuration (config.Load)
│   ├── db/           # pgx pool + embedded migrations (one fresh 000001, ADR 0008)
│   ├── store/        # the ONLY data access: Store interface, Postgres and in-memory impls
│   ├── auth/         # auth.Provider; OwnerPassword (bcrypt)
│   ├── crypto/       # AES-256-GCM sealer for the token bundle
│   ├── garmin/       # garmin.Source; LocalSpawn (bundle → GARMINTOKENS), probe, error classes
│   ├── mcpclient/    # spawns garmin-mcp over stdio; ReadOnly() deny pattern; allowlisted child env
│   ├── agent/        # Claude tool loop over N namespaced MCP sources; prompt; effort; caching; chart tool
│   ├── chart/        # show_chart schema and validator
│   ├── mockchat/     # canned scenarios for the five example questions (RUNCOACH_MOCK=1)
│   ├── web/          # embed of frontend/dist (placeholder committed; Dockerfile fills it)
│   └── api/          # routes, session middleware, login, chats, stream, token paste, health, SSE
└── .env.example
```

### Accounts (Phase 9, ADR 0010)
Runners sign in with an email and a password (`auth.EmailPassword`; an empty email means the
owner). `POST /api/signup` needs `RUNCOACH_INVITE_CODE`; members get `RUNCOACH_MEMBER_CAP`.
`PATCH /api/account`, `POST /api/account/password` (signs out other sessions),
`DELETE /api/account` (members only; cascades to everything the runner owns).
`cmd/set-password <email|owner>` resets from the CLI. The rate limiter counts failures only.
`checkCap` applies the runner's cap, then `RUNCOACH_GLOBAL_DAILY_CAP` across everyone. Members
make their bundle with the `uvx` one-liner or `ghcr.io/lmagsino/garmin-mcp` (published by
`.github/workflows/garmin-mcp-image.yml`); `frontend/src/lib/garmin.js` holds the commands and a
test keeps their pin equal to the Dockerfile's. `npm run e2e:accounts` walks two accounts
through sign-up, isolation, rename, password change and delete.

### Data, sessions and the shell (Phase 7)
- **`internal/store` is the only data access** (ADR 0002). Every statement on a runner's
  table names `runner_id`; a test scans `pg.go` for that and fails if SQL appears anywhere
  else under `internal/`. Without `DATABASE_URL` the in-memory store is used (chats live until
  restart); mock mode always uses it. Run the Postgres suite with
  `RUNCOACH_TEST_DATABASE_URL=postgres://postgres@localhost:5432/runcoach_test?sslmode=disable`.
- **Routes live under `/api`** behind a session cookie (`runcoach_session`, HttpOnly, Lax,
  Secure behind TLS, 30 days). `POST /api/login` (rate-limited) → cookie; `GET /api/me` →
  runner, Garmin state + fix sentence, today's usage, `mock`. Mock mode skips the cookie and
  uses the owner. Handlers read the runner id from the context only; nothing checks "is this
  the owner".
- **Chats:** `GET/POST /api/chats`, `PATCH/DELETE /api/chats/{id}`, `GET …/messages`,
  `POST /api/chats/{id}/stream`. The stream loads history server-side (last 6 completed pairs),
  saves the question first, titles the chat from the first question, answers 429 at the daily
  cap and 409 while a reply runs, writes `: keepalive` every 15 s, saves the reply with its
  `tool_calls` and `chart`, and records the Garmin outcome on the connection row.
  `POST /api/chat` is the stateless curl path.
- **Token paste:** `POST /api/garmin/tokens {bundle}` normalises the base64 line
  `garmin-mcp-auth` writes, probes it with `get_user_profile` through a real spawn, seals it
  (AES-256-GCM, `RUNCOACH_TOKEN_KEY`) and upserts `garmin_connections`. `LocalSpawn` decodes the
  bundle and hands the JSON to the child as `GARMINTOKENS`: python-garminconnect treats a store
  longer than 512 characters as inline token data and parses it with `json.loads`, so it must be
  the JSON, never the base64, and no file is ever written.
- **One origin** (ADR 0007): `internal/web` embeds `frontend/dist` (placeholder committed;
  the Dockerfile copies the real build). Vite proxies `/api` in dev. CORS is off unless
  `ALLOWED_ORIGIN` is set.

### MCP architecture (important)
- **Garmin is the only source.** `garmin.Source` → `LocalSpawn` runs `GARMIN_MCP_COMMAND`
  (default `garmin-mcp`; local dev points it at the Docker image) for each reply and connects
  over stdio. With no stored bundle the command runs as configured, which is the named-volume
  dev setup. Mock mode spawns nothing.
- **The read surface is a deny pattern applied in Go** (`mcpclient.ReadOnly`, ADR 0004) to the
  live `tools/list`: `set_* add_* create_* delete_* upload_* …` and the nutrition, challenge,
  gear, course, workout, women's-health and device-admin groups are never offered to the
  model, so a hallucinated call to one hits the unknown-tool error. 77 of upstream's 155 tools
  remain at the pinned commit. `deploy/garmin-mcp/tools.txt` is the upstream list; tests pin
  the mock scenarios and the prompt's category map to it. Regenerate it when the pin moves.
- **The spawned server gets an allowlisted environment** (`mcpclient.childEnv`, ADR 0003):
  `PATH`, `HOME`, `GARMINTOKENS`, `DOCKER_*` and nothing else. Upstream falls back to an
  email-and-password sign-in when its tokens fail and `GARMIN_EMAIL`/`GARMIN_PASSWORD` are set;
  a server-side sign-in is the one thing this service must never do. A test asserts the
  credentials never reach the child.
- **Tool names are namespaced per source** (`garmin__get_sleep_data`). One source today; the
  roadmap's gateway source would collide on every name, so the prefix is what routes a call.
- **The agent** lists the source's tools, applies its `Allow` filter, exposes the rest to
  Claude under namespaced names, routes each call back, records the trail (`sources` on
  `POST /chat`) and sums token usage (`Result.Usage`, cached input separately).

### Model (ADR 0006)
`ANTHROPIC_MODEL` defaults to `claude-opus-5-5`; `ANTHROPIC_EFFORT` (default `medium`) is sent
as `output_config.effort` on every request. `cache_control: ephemeral` sits on the system
prompt and on the last tool definition, so the whole shared block is one cache prefix; a test
asserts both markers. The system prompt is Garmin-only: grounding rules, a twelve-line
category map pointing at the right tool per question type, the `[figure: …]` marker rule, and
the current date. `maxTurns` is 8; `maxPriorTurns` is 6.

### Charts (ADR 0005)
`internal/chart` holds the `show_chart` schema and `Validate` (kind line|bar|scatter, 5–120
points, one series except line with two on one unit, every error names its field). The agent
appends it as `coach__show_chart` under the built-in `coach` source and answers it in-process:
first valid call kept and emitted as `EventChart`, a second refused readably, an invalid one
returned as a tool error so the prose continues. The stream sends `event: chart` after the
steps and before `answer`; `POST /chat` carries `chart` (or null). Mock scenarios carry
specs through the same validation, one deliberately invalid to prove the drop path. The
frontend holds the chart until the answer arrives (`useChat`), then `ChartCard.vue` draws it
from `lib/chart.js` (pure layout: nice ticks, m:ss pace, dashed second series with direct
labels, faint partial bars, least-squares trend) with a "View data" table.

### Run locally
```bash
cd backend
cp .env.example .env            # first time; ANTHROPIC_API_KEY, GARMIN_MCP_COMMAND
RUNCOACH_MOCK=1 ../bin/dev      # canned answers, no credentials, no database; API + UI
../bin/dev                      # live: needs the key and the authenticated Garmin image
curl localhost:8787/healthz     # {"status":"ok","db":"off"} without DATABASE_URL
curl -s localhost:8787/api/chat -d '{"question":"how did I sleep this week?"}'
```

### Garmin MCP container (local dev)
`deploy/garmin-mcp/Dockerfile` installs upstream at `GARMIN_MCP_REF` (currently `43b0c5b`,
12 Oct 2026). `docker compose build garmin-mcp`, then `docker compose --profile auth run --rm
garmin-mcp-auth` once, interactively (email, password, MFA), which writes the token bundle
into the named volume `run-coach-ai-garmin-tokens` (about six months). Compose never runs the
server itself; the backend spawns it per reply. When the tokens expire, the first reply fails
with an authentication error; rerun the auth step.

### Testing
`go test ./...` makes no network call. In-process stub MCP servers (`stubmcp_test.go`) and a
scripted Messages API (`fakemodel_test.go`) cover namespacing and routing, the read-surface
filter, the effort and cache markers, grounding over fabricated sleep and pace datasets, prior
turns, and the observer events. `RUNCOACH_LIVE_MODEL_TESTS=1` enables the one test that lets
the real model choose its tools over the fake data.

## Frontend (Vue 3)

**Location:** `frontend/`. Vite + Vue 3.5 + Tailwind v4. `npm run dev` on :5173, `npm test`
(vitest), `npm run build`. The backend must be running on :8787; mock mode needs nothing else.

### Design system
`DESIGN.md` is the source of truth and `design/mockup.html` the canonical screen. Palette
tokens live in `src/style.css` as Tailwind v4 `@theme` variables under the same names
(`paper`, `ink`, `accent`). One accent by design.
- **`wide:` is a custom breakpoint at 561px**, not Tailwind's `sm:` (640px). Compact-first.
- **Fonts are self-hosted** via `@fontsource`; only the specified weights are imported.
- **`prefers-reduced-motion` is a base-layer reset** in `src/style.css`.
- **Agent replies get no bubble; only user messages do** (DESIGN.md §1.2).

### Shell and flow
`vue-router` (`/login`, `/`, `/c/:id`, `/connection`); the guard loads `/me` once and sends a
401 to `/login`. `App.vue` is the shell: `Sidebar` beside the page above the `wide:` breakpoint,
a drawer behind the header's menu button below it. `useSession` (one shared `me`), `useChats`
(the list), `useChat(transport, {loadMessages})` (one chat's thread; `open(id)` rebuilds it
from saved messages; `send` streams; the server holds the history so nothing is replayed from
the client). On `/` the first question creates a chat and the URL follows. `LoginPage` polls
`/api/healthz` and shows the waking state until the host answers. `ConnectionPage` is the
paste box. Answer text is parsed, not JSON (`lib/answer.js`); step labels in
`lib/stepLabels.js`; the chart card in `ChartCard.vue` from `lib/chart.js`.

## Required env vars
See `backend/.env.example`. Summary:
| Var | Purpose |
|---|---|
| `PORT` | HTTP port (default 8787 locally; hosts set their own). Not 8080: on the work laptop a security agent stalls PATCH requests to localhost:8080 |
| `DATABASE_URL` | Postgres connection string. **Optional** until Phase 7; unset ⇒ `/healthz` says `db: off` |
| `GARMIN_MCP_COMMAND` | Command to launch garmin-mcp (default `garmin-mcp`; local dev: the Docker image). The stored bundle is passed as `GARMINTOKENS` |
| `RUNCOACH_OWNER_EMAIL` / `RUNCOACH_OWNER_PASSWORD_HASH` | The owner's sign-in, applied only where the owner row has none (ADR 0010). Quote the hash |
| `RUNCOACH_INVITE_CODE` | Opens sign-up for other runners. Unset ⇒ sign-up closed |
| `RUNCOACH_MEMBER_CAP` / `RUNCOACH_GLOBAL_DAILY_CAP` | Daily questions for a new member (10) and for everyone together (100; 0 = off) |
| `RUNCOACH_TOKEN_KEY` | base64 32-byte key sealing the stored bundle (`cmd/token-key`) |
| `GARMIN_EMAIL` / `GARMIN_PASSWORD` | Read only by the one-off `garmin-mcp-auth` step via compose. The server never reads them and never passes them to the child |
| `GARMIN_IS_CN` | `true` only for Garmin Connect China |
| `ANTHROPIC_API_KEY` | Claude API access |
| `ANTHROPIC_MODEL` | Coach model (default `claude-opus-5-5`) |
| `ANTHROPIC_EFFORT` | `low` / `medium` / `high` (default `medium`) |
| `ALLOWED_ORIGIN` | Browser origin allowed by CORS. Empty (default) disables CORS; the UI shares the API's origin |
| `RUNCOACH_MOCK` | **Dev only.** `1`/`true`/`yes`/`on` ⇒ canned answers, no credentials, no database |

## Current phase (updated 14 Oct 2026)
Phase 9 (accounts) is built: sign-up by invite, email sign-in, account page, per-runner Garmin
bundles, caps. Phase 8 (free hosting) is next.

## Earlier status
Phases 5 (Garmin core), 6 (charts) and 7 (shell: login, token paste, saved chats, sidebar, one
origin) are built against mocks and tests. Next: Phase 8 deploy on free tiers and the live
pass. Track in `tasks/todo.md`. Nothing has run against a live Garmin account yet; Phase 8 is
where that happens. `npm run e2e` (browser walkthrough on mock mode) and `node e2e/shots.mjs`
are the manual checks before merging UI changes.
