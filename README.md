<div align="center">

# 🏃 RunCoach AI

**A chat-first AI running coach that reasons across your Strava _and_ Garmin data — answering questions neither app can answer alone.**

_by [Leo Magsino Jr](https://github.com/lmagsino)_

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Postgres](https://img.shields.io/badge/Postgres-15-4169E1?logo=postgresql&logoColor=white)
![MCP](https://img.shields.io/badge/protocol-MCP-6E56CF)
![Claude](https://img.shields.io/badge/Claude-Sonnet-D97757?logo=anthropic&logoColor=white)
![Vue](https://img.shields.io/badge/Vue_3-planned-4FC08D?logo=vuedotjs&logoColor=white)
![Phase 2](https://img.shields.io/badge/Phase_2-backend_complete-2E5D4E)

</div>

---

## The idea

Strava and Garmin don't share data with each other — but an agent can hold both at once. RunCoach AI connects to each source over the **Model Context Protocol (MCP)**, then reasons across the combined result to answer natural-language training questions:

> _"Does my sleep actually affect my pace?"_ · _"Am I overtraining right now?"_ · _"I have a half marathon in 10 weeks — am I on track, factoring in how recovered I've been?"_

No dashboards, no cards, no widgets. **The chat interaction _is_ the product.**

## Why it's different

| Capability | Strava alone | Garmin alone | RunCoach AI |
|---|:---:|:---:|:---:|
| Sleep/HRV vs. pace correlation | ❌ | ❌ | ✅ combines both |
| Overtraining detection (load + effort variability) | ❌ | ⚠️ partial | ✅ full picture |
| Free-form Q&A over your history | ❌ | ❌ | ✅ core interaction |
| Reconciling conflicting data (same run, different HR) | ❌ | ❌ | ✅ |
| Readiness-adjusted race planning | ⚠️ | ⚠️ | ✅ both signals |

## Architecture

The Go backend is a pure **MCP client**. Each data source is a **self-hosted MCP server** it connects to — keeping the whole system free, local, and fully under our control.

```mermaid
flowchart LR
    User([Runner]) -->|chat| Vue["Vue 3 UI<br/><i>Phase 4</i>"]
    Vue -->|HTTP| Go["Go backend<br/>MCP client + agent loop"]
    Go <-->|"Messages API<br/>+ tool use"| Claude["Claude (Sonnet)"]
    Go -->|"stdio (MCP)"| SMCP["strava-mcp<br/><i>self-hosted, ours</i>"]
    Go -.->|"stdio (MCP), Phase 3"| GMCP["garmin_mcp<br/><i>self-hosted container</i>"]
    SMCP -->|REST| Strava[("Strava API")]
    GMCP -.-> Garmin[("Garmin Connect")]
```

When a question arrives, Claude decides which tools to call, the backend executes them against the MCP server(s), and Claude answers **grounded only in what the tools returned** — no generic training-advice filler.

> **Design note:** the spec originally called for Strava's _hosted_ MCP (`mcp.strava.com`). It turned out to require a paid Strava subscription and is gated to Claude's first-party connector — so we run our **own** thin Strava MCP server over Strava's free REST API instead. Same MCP story, no external gatekeeping.

## Tech stack

| Layer | Choice |
|---|---|
| Backend | **Go 1.26** — stdlib `net/http`, `pgx/v5`, `golang-migrate` |
| Agent | **Claude (Sonnet)** via the Anthropic Go SDK, tool-calling loop |
| Protocol | **MCP** via the official `modelcontextprotocol/go-sdk` |
| Database | **Postgres** — chat sessions/messages + normalized activity cache |
| Auth | **Strava OAuth2** (per-user), tokens stored + auto-refreshed |
| Frontend | **Vue 3 + Tailwind** _(Phase 4)_ |
| Hosting | **Local-only** for v1 — no infra to manage |

## Repository layout

```
run-coach-ai/
├── DESIGN.md, design/                 # Phase 1 — "Field Notes" UI direction
├── running-agent-feature-spec.md      # Full feature spec
├── PROGRESS.md                        # Cross-session task tracker
├── CLAUDE.md                          # Contributor / agent guide
└── backend/                           # Go API (Phase 2)
    ├── cmd/
    │   ├── server/       # HTTP API: /healthz, /auth/strava/*, /chat
    │   ├── migrate/      # DB migration runner
    │   ├── strava-mcp/   # self-hosted Strava MCP server (stdio)
    │   └── strava-check/ # CLI: proves list_activities over MCP
    └── internal/
        ├── config/  db/  strava/  mcpclient/  agent/  api/
```

## Getting started

**Prerequisites:** Go 1.26+, Postgres 15+, a [Strava API app](https://www.strava.com/settings/api) (Client ID/Secret), and an [Anthropic API key](https://console.anthropic.com).

```bash
cd backend
cp .env.example .env         # fill in Strava + Anthropic credentials

createdb runcoach_dev        # one-time (local Postgres)
go run ./cmd/migrate up      # apply schema
go run ./cmd/server          # API on http://localhost:8080
```

Then connect Strava and ask a question:

```bash
# 1. Authorize Strava (opens Strava's consent screen)
open http://localhost:8080/auth/strava/login

# 2. Prove the MCP path returns real activities
go run ./cmd/strava-check

# 3. Ask the coach
curl -s localhost:8080/chat -d '{"question":"how many runs did I do last week?"}'
```

Verify the MCP plumbing anytime — **no credentials needed**:

```bash
go test ./internal/mcpclient/   # builds strava-mcp, connects, checks tool discovery
```

## Roadmap

- [x] **Phase 1 — Design** · chat UI direction ("Field Notes"), locked in [`DESIGN.md`](DESIGN.md)
- [x] **Phase 2 — Backend Foundations** · Go API, Postgres, Strava OAuth, self-hosted Strava MCP, Claude tool-loop
- [ ] **Phase 3 — Garmin Integration** · self-hosted `garmin_mcp`, cross-source reasoning
- [ ] **Phase 4 — Frontend** · Vue 3 chat UI against the Phase 1 design
- [ ] **Phase 5 — Polish & Demo** · end-to-end testing, demo readiness

> Phase 2 is a **code checkpoint** — everything builds and the MCP plumbing is tested; the live paths (real OAuth exchange, real activity data, real answers) are verified once credentials are in place.

## Design

The chat UI follows a **"Field Notes"** direction: quiet-luxury, one confident accent color, generous whitespace, and assistant replies that read as flowing prose rather than chat bubbles — a deliberate anti-chatbot move. Full reference in [`DESIGN.md`](DESIGN.md); working mockup at [`design/mockup.html`](design/mockup.html).

| Token | Hex | Role |
|---|---|---|
| `paper` | `#EDECE6` | App background (soft greige) |
| `ink` | `#1C201C` | Primary text (forest-charcoal) |
| `accent` | `#2E5D4E` | Deep pine green — the one accent |

---

<div align="center">

**Built by [Leo Magsino Jr](https://github.com/lmagsino)** · Personal portfolio project · 2026

</div>
