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

## Backend (Go)

**Location:** `backend/`, `go.mod` lives here; run all Go commands from this dir.
**Module path:** `github.com/lmagsino/run-coach-ai/backend`. **Go:** 1.26 (`/opt/homebrew/bin/go`).

### Structure
```
backend/
├── cmd/
│   ├── server/       # HTTP API: /healthz, /sources, /chat, /chat/stream
│   └── migrate/      # DB migration runner (needs DATABASE_URL)
├── internal/
│   ├── config/       # env-based configuration (config.Load)
│   ├── db/           # pgx pool + embedded migrations (rewritten from 000001 in Phase 7, ADR 0008)
│   ├── mcpclient/    # spawns garmin-mcp over stdio; ReadOnly() deny pattern; allowlisted child env
│   ├── agent/        # Claude tool loop over N namespaced MCP sources; prompt; effort; caching
│   ├── mockchat/     # canned scenarios for the five example questions (RUNCOACH_MOCK=1)
│   └── api/          # HTTP handlers (Server struct), SSE writer, CORS
└── .env.example
```

### MCP architecture (important)
- **Garmin is the only source.** The backend spawns the upstream `garmin-mcp` for each reply
  (`GARMIN_MCP_COMMAND`, a `docker run -i ...` in local dev; a plain binary in the hosted
  image from Phase 8) and connects over stdio. Unset ⇒ no source; live requests answer 503
  saying so. Mock mode still works.
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

### Run locally
```bash
cd backend
cp .env.example .env            # first time; ANTHROPIC_API_KEY, GARMIN_MCP_COMMAND
RUNCOACH_MOCK=1 go run ./cmd/server   # canned answers, no credentials, no database
go run ./cmd/server             # live: needs the key and the authenticated Garmin image
curl localhost:8080/healthz     # {"status":"ok","db":"off"} without DATABASE_URL
curl -s localhost:8080/chat -d '{"question":"how did I sleep this week?"}'
```

### Garmin MCP container (local dev)
`deploy/garmin-mcp/Dockerfile` installs upstream at `GARMIN_MCP_REF` (currently `43b0c5b`,
12 Oct 2026). `docker compose build`, then `docker compose --profile auth run --rm
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
(vitest), `npm run build`. The backend must be running on :8080; mock mode needs nothing else.

### Design system
`DESIGN.md` is the source of truth and `design/mockup.html` the canonical screen. Palette
tokens live in `src/style.css` as Tailwind v4 `@theme` variables under the same names
(`paper`, `ink`, `accent`). One accent by design.
- **`wide:` is a custom breakpoint at 561px**, not Tailwind's `sm:` (640px). Compact-first.
- **Fonts are self-hosted** via `@fontsource`; only the specified weights are imported.
- **`prefers-reduced-motion` is a base-layer reset** in `src/style.css`.
- **Agent replies get no bubble; only user messages do** (DESIGN.md §1.2).

### How a question flows
`Composer` → `useChat` → `lib/api.js` → `POST /chat/stream` → `step` events → `StatusSteps`,
then the answer replaces the steps. `useChat` takes its transport as an argument. Answer text
is parsed, not JSON (`lib/answer.js`: lede, paragraphs, at most one `[figure: …]` pull quote).
Step labels live in `lib/stepLabels.js`, keyed by `garmin/<real tool name>`; unknown tools get
a generic-but-honest line. The UI asks `GET /sources` what it is connected to and labels mock
mode as such.

## Required env vars
See `backend/.env.example`. Summary:
| Var | Purpose |
|---|---|
| `PORT` | HTTP port (default 8080) |
| `DATABASE_URL` | Postgres connection string. **Optional** until Phase 7; unset ⇒ `/healthz` says `db: off` |
| `GARMIN_MCP_COMMAND` | Command to launch garmin-mcp. **Unset ⇒ no source**; live requests answer 503 |
| `GARMIN_EMAIL` / `GARMIN_PASSWORD` | Read only by the one-off `garmin-mcp-auth` step via compose. The server never reads them and never passes them to the child |
| `GARMIN_IS_CN` | `true` only for Garmin Connect China |
| `ANTHROPIC_API_KEY` | Claude API access |
| `ANTHROPIC_MODEL` | Coach model (default `claude-opus-5-5`) |
| `ANTHROPIC_EFFORT` | `low` / `medium` / `high` (default `medium`) |
| `ALLOWED_ORIGIN` | Browser origin allowed by CORS (default the Vite dev server). Empty disables CORS |
| `RUNCOACH_MOCK` | **Dev only.** `1`/`true`/`yes`/`on` ⇒ canned answers, no credentials, no database |

## Current phase
**Phase 5, Garmin core** on `phase-5-garmin-core`: Strava removed, database optional, pin
bumped, read surface filtered in Go, Garmin-only prompt on Opus 5.5. Next: Phase 6 charts,
Phase 7 shell, Phase 8 deploy on free tiers and the live pass. Track in `tasks/todo.md`.
Nothing has run against a live Garmin account yet; Phase 8 is where that happens.
