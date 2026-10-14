<div align="center">

# 🏃 RunCoach AI

**Ask your Garmin anything.**

A chat over your own Garmin data. Ask in plain words; the coach reads your runs, sleep, HRV, stress and training load through the Model Context Protocol and answers in grounded prose, with one chart when a pattern is the point.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Vue](https://img.shields.io/badge/Vue_3-Vite_8-4FC08D?logo=vuedotjs&logoColor=white)
![MCP](https://img.shields.io/badge/protocol-MCP-6E56CF)
![Claude](https://img.shields.io/badge/Claude-Opus_5.5-D97757?logo=anthropic&logoColor=white)
![Hosting](https://img.shields.io/badge/hosting-free_tiers-2E5D4E)

<img src="docs/screenshots/live-how-s-my-training.gif" alt="Asking RunCoach 'how's my training': the steps appear as it reads Garmin, then a bar chart of distance per run and a short reply" width="900">

<sub>A real question against my own Garmin account, recorded as it happened: 18 seconds, three Garmin tools, one chart, about five cents.</sub>

</div>

---

## The idea

A Garmin watch records far more than the Garmin app lets you ask about. It knows every run, every night's sleep, your HRV, stress, body battery and training load, but it only shows them one screen at a time, and it never answers a question like *"does my sleep actually affect my pace?"*

RunCoach does. It hands Claude about eighty read-only Garmin tools and lets you ask:

> _"Does my sleep actually affect my pace? Show me the pattern."_
> _"Am I overtraining right now?"_
> _"How did my last three training blocks compare?"_
> _"I have a half marathon in 10 weeks. Am I on track, given how recovered I've been?"_
> _"Why has my body battery been so low this week?"_

Every reply is grounded in what the tools returned, cites the numbers and dates, and says plainly when the data is not there. Replies are short, the conclusion comes first, and when a conclusion rests on a series of values the chart carries them, as the proof. Off-topic questions are declined before they cost anything. No generic training advice, no dashboards. **The chat is the product.**

It is built for one runner first (me), with every table and interface shaped so a second runner is additive later.

## How it works

```mermaid
flowchart LR
    You([You, laptop or phone]) -->|owner login| UI["Vue 3<br/>chat shell · chart card"]
    UI -->|SSE| Go["Go backend<br/>MCP client · agent loop"]
    Go <-->|"Messages API<br/>tool use · cached tool block"| Claude["Claude Opus 5.5"]
    Go -->|"stdio MCP, per reply<br/>77 read-only tools"| MCP["garmin-mcp<br/><i>upstream, pinned</i>"]
    MCP --> Garmin[("Garmin Connect")]
    Go --> DB[("Postgres<br/>chats · encrypted token bundle")]
    Laptop([Your laptop, every ~6 months]) -. "garmin-mcp-auth → paste the bundle" .-> UI
```

The Go backend is a pure MCP client. A small model screens each question first: off-topic ones are declined on the spot, and on-topic ones get a concrete scope (which metrics, which dates). Then the backend spawns the upstream [`Taxuspt/garmin_mcp`](https://github.com/Taxuspt/garmin_mcp) server, filters its tool list down to the reads, and runs Claude's tool loop over exactly that scope. The frontend streams the steps as they happen, then the chart and the reply.

```mermaid
sequenceDiagram
    participant U as You
    participant F as Vue
    participant G as Go
    participant M as garmin-mcp
    participant C as Claude
    U->>F: "Does my sleep affect my pace?"
    F->>G: POST /api/chats/:id/stream
    G->>C: triage (Haiku): on topic? what to read?
    C-->>G: scope: sleep and runs, last 8 weeks
    G->>M: spawn with your token bundle
    G->>C: question + scope + 77 tools (cached)
    C-->>G: call get_sleep_summary_range
    G->>M: tool call
    M-->>G: nights
    G-->>F: step: Reading your Garmin sleep
    C-->>G: call get_activities_by_date
    G-->>F: step: Reading your activities
    C-->>G: call coach__show_chart (points)
    G-->>G: validate the chart
    G-->>F: chart
    C-->>G: the prose
    G-->>F: answer
    G->>M: close
```

<img src="docs/screenshots/live-how-s-my-training.png" alt="The finished reply to 'how's my training': a bar chart of distance per run for 1 to 14 October above two short paragraphs" width="900">

<sub>The finished reply. The chart is drawn from the activities the coach read, the caption names the pattern, and "View data" shows the points.</sub>

### The chart is the proof

The coach has a `show_chart` tool next to the Garmin ones. After reading the data and before writing, it reshapes the values its conclusion rests on into points and calls it; the backend validates the spec (line, bar or scatter, 5 to 120 points, one axis, never a dual axis) and the browser draws it by hand in SVG. The prose then names the pattern and the one or two numbers that matter. A single number gets a pull quote instead. Every chart has a "View data" table.

<img src="docs/screenshots/desktop-bars.png" alt="Weekly distance bar chart with the current week drawn faint and the data table open" width="900">

<sub>Mock mode, labelled as such: canned answers for the five example questions, so the UI can be built and demoed with no account at all.</sub>

### On the phone

<p>
<img src="docs/screenshots/phone-chart.png" alt="The same answer on a phone" width="300">
<img src="docs/screenshots/phone-drawer.png" alt="The chat list as a drawer on a phone" width="300">
</p>

### Signing in, and connecting Garmin

One password for the owner, remembered for 30 days. Garmin is connected with a token bundle created once on your own laptop by upstream's `garmin-mcp-auth`; the server imports it on start (or from a paste, on a hosted copy), checks it with one profile read, stores it encrypted, and hands it to each spawned `garmin-mcp`. **The hosted server never signs in to Garmin itself.**

<p>
<img src="docs/screenshots/login.png" alt="Owner sign-in" width="440">
<img src="docs/screenshots/connection.png" alt="The Connection page where the Garmin token bundle is pasted" width="440">
</p>

## Decisions worth knowing

Each one is an ADR in [`docs/adr/`](docs/adr/); the terms are in [`CONTEXT.md`](CONTEXT.md).

| | Decision | Why |
|---|---|---|
| [0001](docs/adr/0001-garmin-is-the-only-source.md) | Garmin is the only source | It already holds everything the training log had, plus recovery |
| [0003](docs/adr/0003-hosted-server-never-signs-in-to-garmin.md) | The server never signs in to Garmin | Garmin throttles server sign-ins per IP; the spawned process gets an allowlisted environment so it cannot sign in even if asked |
| [0004](docs/adr/0004-upstream-garmin-mcp-unmodified-read-surface-filtered-in-go.md) | Upstream unmodified; the read surface is a deny pattern in Go | Upstream's own allowlist ignores unknown names silently; a pattern denies a new write tool by shape |
| [0005](docs/adr/0005-charts-are-a-virtual-tool-validated-by-the-backend.md) | Charts are a tool the model calls and the backend validates | The model reshapes, the backend checks, the browser draws |
| [0006](docs/adr/0006-opus-5-5-with-explicit-effort-and-prompt-caching.md) | Opus 5.5 with explicit effort and prompt caching | Cache reads cost the same as Sonnet; the premium lands only on uncached tokens |
| [0007](docs/adr/0007-one-free-render-service-serves-api-and-ui-from-one-origin.md) | One free Render service serves API and UI from one origin, Neon for Postgres | Free only; one cookie, no CORS, tokens encrypted in the database because there is no disk |

## Tech stack

| Layer | Choice |
|---|---|
| Backend | **Go 1.26**, stdlib `net/http`, `pgx/v5`, `golang-migrate`; one `store` package is the only data access |
| Agent | **Claude** via the Anthropic Go SDK: Haiku 5.5 screens and scopes, Sonnet or Opus 5.5 answers; tool loop, explicit effort, prompt caching on the system prompt and tool block |
| Protocol | **MCP** via the official `modelcontextprotocol/go-sdk` |
| Garmin | **`Taxuspt/garmin_mcp`** at a pinned commit, spawned per reply; 77 of its 155 tools exposed |
| Frontend | **Vue 3 + Vite + Tailwind v4**, the "Field Notes" design in [`DESIGN.md`](DESIGN.md), hand-rolled SVG charts |
| Database | **Postgres**, optional locally; chats and the encrypted token bundle |
| Hosting | **Free tiers**: one Render web service from the `Dockerfile`, Neon Postgres |

## Run it

**Prerequisites:** Go 1.26+, Node 22+. Docker only for the Garmin server. Postgres only once you want chats to survive a restart.

Try the UI against canned answers first, with nothing else set up:

```bash
cd backend && RUNCOACH_MOCK=1 go run ./cmd/server     # API on :8787, no login, no credentials, no database
cd frontend && npm install && npm run dev               # UI on :5173
```

Then for real, locally:

```bash
cd backend
cp .env.example .env                                    # ANTHROPIC_API_KEY, GARMIN_MCP_COMMAND, GARMIN_EMAIL/PASSWORD, then:
go run ./cmd/token-key                                  #   RUNCOACH_TOKEN_KEY
echo -n 'your password' | go run ./cmd/hash-password    #   RUNCOACH_OWNER_PASSWORD_HASH (keep the quotes)

cd ../deploy/garmin-mcp
docker compose build garmin-mcp                         # garmin_mcp at the pinned commit
./auth.sh                                               # sign in once (MFA); leaves the token bundle for the server

cd ../../backend && go run ./cmd/server                 # imports the bundle on start: "garmin: connected"
```

Open the UI, sign in, and ask. When the bundle expires, about six months later, the chip says so; run `auth.sh` again and restart. Values in `.env` win over the shell's environment, so a key exported for another tool cannot leak in.

### What a question costs

Pricing is per token, so every question has a price. Measured on the question above, on Sonnet 5.5: about 8k input tokens, 59k read from cache at a tenth of the price, 1k output, roughly five cents. The triage step on Haiku is a fraction of that, the daily cap is 30 questions, and `ANTHROPIC_MODEL` switches between Haiku, Sonnet and Opus with no other change.

### Deploy, free

`Dockerfile` builds one image (the Vue build embedded in the Go binary, `garmin-mcp` installed beside it) and [`render.yaml`](render.yaml) describes one Render free web service with `/healthz` as its health check. Pair it with a Neon free Postgres and set `DATABASE_URL`, `ANTHROPIC_API_KEY`, `RUNCOACH_OWNER_PASSWORD_HASH`, `RUNCOACH_TOKEN_KEY` and `RUNCOACH_GARMIN_BUNDLE` (the line `bundle.sh` prints) in the dashboard. Migrations run on start. The free service sleeps when idle and takes about a minute to wake; the login page says so while it waits.

### Tests, with no credentials and no network

```bash
cd backend && go test ./...
cd frontend && npm test
cd frontend && npm run e2e      # browser walkthrough against a mock-mode backend
```

The agent's full tool loop runs against in-process stub MCP servers and a scripted stand-in for the Messages API: tool namespacing and routing, the read-surface filter, the cache and effort markers on every request, chart validation, and grounding scenarios on fabricated sleep and pace datasets. The store suite runs on an in-memory store always and on Postgres when `RUNCOACH_TEST_DATABASE_URL` is set. One opt-in test lets the real model choose its tools over the fake data:

```bash
RUNCOACH_LIVE_MODEL_TESTS=1 go test ./internal/agent/ -run LiveModel -v   # costs Anthropic tokens
```

## Status

- [x] Phases 1–4: design, Go backend, Garmin wiring, Vue chat UI, all against mocks
- [x] Phase 5, Garmin core: read surface filtered in Go, Garmin-only prompt with a category map
- [x] Phase 6, charts: one validated chart card per reply
- [x] Phase 7, shell: owner login, token paste, saved chats, sidebar and phone drawer, one origin
- [x] Live on my own account: the sign-in, the bundle import, triage, the first real replies with charts
- [ ] **Phase 8, deploy on free tiers**: image and blueprint written; the Render + Neon setup, the full five-question pass and Playwright in CI remain

The plan and its checklist are in [`tasks/`](tasks/).

> `garmin_mcp` drives Garmin Connect's private endpoints with a personal login. There is no public API behind it, so it can break when Garmin changes them; the coach then says "Garmin unavailable" rather than guessing.

## Design

The UI follows a **"Field Notes"** direction: quiet, one accent colour, generous whitespace, replies as flowing prose rather than chat bubbles. Reference in [`DESIGN.md`](DESIGN.md).

| Token | Hex | Role |
|---|---|---|
| `paper` | `#EDECE6` | App background |
| `ink` | `#1C201C` | Primary text |
| `accent` | `#2E5D4E` | Deep pine green, the one accent |

---

<div align="center">

**Built by [Leo Magsino Jr](https://github.com/lmagsino)** · Personal project · 2026 · Not affiliated with Garmin

</div>
