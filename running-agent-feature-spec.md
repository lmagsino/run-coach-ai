# RunCoach AI — Feature Spec (Chat-First)

## 1. Overview
A chat-first AI agent that connects to both Strava and Garmin via MCP, reasons across both data sources, and answers natural-language questions runners can't get answered by either platform alone — because Strava and Garmin don't share data with each other, but the agent can hold both at once.

No dashboard, no cards, no widgets in v1. The chat interaction *is* the product.

---

## 2. Core Concept
The agent has two MCP tool sets available at once:
- **Strava MCP** (hosted connector): activities, athlete profile, HR/pace zones, activity streams, performance, gear
- **Garmin MCP** (self-hosted): activities, sleep, HR, stress, body composition, training status/readiness

When a question comes in, the agent decides which tool(s) to call — one, the other, or both — then reasons over the combined result before answering. This cross-source reasoning is the entire value proposition; it's the thing neither app can do on its own.

---

## 3. What Makes This Different From Just Using Strava or Garmin
| Capability | Strava alone | Garmin alone | This agent |
|---|---|---|---|
| Sleep/HRV vs. pace/performance correlation | No (no biometrics) | No (no cross-app awareness) | Yes — combines both |
| Overtraining detection using both training load + effort variability | No | Partial (Garmin-only signal) | Yes — full picture |
| Free-form natural language Q&A over your history | No (UI-only) | No (UI-only) | Yes — core interaction |
| Reconciling conflicting data (same run logged twice, differing HR/GPS) | No | No | Yes |
| Readiness-adjusted goal/race planning | Partial (no readiness data) | Partial (Garmin-only) | Yes — both signals combined |

---

## 4. Example Interactions (what it should be able to answer)
- "Does my sleep actually affect my pace? Show me the pattern."
- "Am I overtraining right now?"
- "How did my last 3 marathon training blocks compare?"
- "I have a half marathon in 10 weeks — am I on track, factoring in how recovered I've been?"
- "My watch and Strava show different HR for yesterday's run — which is right?"

---

## 5. Agent Behavior Requirements
- **Tool selection**: agent decides whether a question needs Strava data, Garmin data, or both — not hardcoded per question type.
- **Visible reasoning (lightweight)**: UI shows short status lines while working, e.g. "Checking Garmin sleep data..." then "Checking Strava activities..." — makes the multi-source nature visible rather than hidden, good for demoing.
- **Multi-turn memory**: follow-up questions ("what about the week before that?") retain context within a session.
- **Grounded answers only**: agent should base claims on actual tool data returned, not fill gaps with generic training advice — cite what it found (e.g. "your last 4 long runs averaged X pace").

---

## 6. Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Frontend | Vue 3 + Tailwind | Simple chat UI, minimal surface area for v1 |
| Backend | Go (e.g. net/http or a lightweight framework like Echo/Fiber) | Hosts Garmin MCP client/proxy, manages session state, serves API to Vue app |
| Agent | Claude API (Sonnet) with tool calling, called from Go backend via HTTP | Strava via `mcp_servers` param (native MCP support); Garmin tool results proxied through Go backend |
| Garmin integration | Self-hosted `garmin_mcp` container, deployed alongside Go backend (Fly.io/Render) | No hosted Garmin MCP exists; this is real backend infra, not local-only. Go backend talks to it as an internal service |
| Auth | Strava OAuth (standard, per-user ready) via a Go OAuth2 library; Garmin email/password + MFA, tokens stored encrypted server-side | Matches each service's real auth model |
| Database | Postgres | Normalized activity/health data cache, chat session/message history, user + token storage |
| Deployment | **Local only for v1** — Vue dev server, Go backend, Garmin MCP container (Docker), and Postgres all run on localhost. No hosting provider needed. | Simplest path for a personal/portfolio build; no infra to manage or pay for |

**Note on Go + MCP**: Go actually has good footing here — Anthropic and the MCP community maintain official/community Go SDKs for MCP clients, so writing the client that calls both the Strava hosted MCP and your self-hosted Garmin MCP is straightforward, arguably cleaner than the Rails option. Go's concurrency model (goroutines) is also a nice fit for calling both MCP tool sets in parallel when a question needs both sources.

**Note on local-only**: Strava's MCP is hosted by Strava regardless (it's their server, reached over the internet even when your app is local) — only your own backend, Garmin container, and Postgres stay on localhost. If you later want a shareable link for a portfolio (rather than demoing live/via recording), deployment can be added later without rearchitecting anything — see Section 9.

---

## 7.1 Estimated Costs
| Item | Cost | Notes |
|---|---|---|
| Hosting (backend, DB, frontend) | **$0** | Everything runs locally, no provider needed for v1 |
| Strava API | **$0** | Free for personal/portfolio use within rate limits |
| Garmin Connect | **$0** | No official API cost; self-hosted MCP uses your personal login |
| Claude API (the agent itself) | **~$5–20/month**, usage-dependent | Roughly a few thousand tokens per chat question (tool calls + reasoning); scales with how often you test/demo it, not a fixed fee |

**Total for local-only v1: effectively $0 fixed cost, only variable Claude API usage while actively using it.** If deployed later for a shareable public demo, add roughly **$0–10/month** for lightweight hosting (Fly.io/Render free-to-hobby tiers, Vercel/Netlify free tier for frontend).

---

## 8. MVP Scope (v1)
- Chat UI only — no dashboard, no cards, no charts required
- Both Strava and Garmin MCP connected and callable by the agent
- Agent can answer the 5 example interaction types above, grounded in real data
- Single-user, local-only setup (one Garmin account, one demo Strava account, run on localhost)
- Lightweight "checking X..." status indicators while tools run

## 9. Explicitly Out of Scope for v1
- Dashboard/summary views
- Auto-generated shareable visual cards
- Multi-user Garmin auth (per-user token management) — single demo account is enough for portfolio purposes
- Auto-posting or writing back to either platform
- Mobile app
- Public deployment/hosting (deferred — see Section 7.1 for path if needed later)

## 10. Open Questions
- Garmin demo account: use a real personal account, or a sandbox/dummy account (relevant mainly if you deploy publicly later)?
- If/when deploying later: Fly.io/Render vs. another provider for hosting?

---

## 11. Phased Work Plan

**Phase 1 — Design**
- UI/UX for the chat interface: layout, visual identity, message styling, how status indicators ("Checking Garmin sleep data...") look and feel
- Done first so Phase 4 (frontend build) implements against a concrete design instead of guessing at layout while coding

**Phase 2 — Backend Foundations**
- Initialize repo, create `CLAUDE.md` at project root (bash commands, code style, MCP connection notes, env var setup) so Claude Code has persistent project context from the start
- Go project setup
- Postgres schema: chat sessions, messages, cached activity/health data
- Strava MCP client code (hosted connector) — written and unit/mock-tested, not run against a live account yet
- Basic Claude API tool-calling loop using Strava data only, no Garmin yet
- *(Live Strava credentials, OAuth flow execution, and real API verification deferred to Phase 5)*

**Phase 3 — Garmin Integration**
- `garmin_mcp` (Taxuspt/garmin_mcp) integration code — Dockerfile/config and Go client wiring written, not run against a live account yet
- Wire it into the Go backend as a second tool source alongside Strava (code-level only)
- Cross-source reasoning logic in the Claude API tool-calling loop (agent decides which tool(s) to call) — tested with mocked/stubbed tool responses, not live data
- *(Live Garmin account setup, credentials, and real cross-source verification deferred to Phase 5)*

**Phase 4 — Frontend**
- Build the Vue chat UI against the Phase 1 design
- Wire up to the Go backend
- Add the lightweight "checking X..." status indicators while tools run
- Frontend can be built/tested against mocked backend responses if live credentials aren't wired up yet

**Phase 5 — Credentials, Live Verification, Polish & Demo Readiness**
- Obtain Strava API credentials (client ID/secret) via Strava's developer portal, run the real OAuth flow
- Confirm/set up the Garmin account (personal vs. dedicated demo account), authenticate `garmin_mcp` for real
- Obtain Anthropic API key if not already set up
- Run the full system live end-to-end: test the 5 example questions from Section 4 against real Strava + Garmin data
- Fix rough edges in agent responses/UI surfaced by real data
- Prepare demo format (recording, live walkthrough, or deployed link if pursued later)

**Sequencing rationale**: Phases 2-4 focus purely on writing and unit/mock-testing code, so progress isn't blocked on account setup, API approvals, or credential wrangling. All live credentials and real-data verification — Strava, Garmin, and the actual cross-source reasoning test — are consolidated into Phase 5, so there's one clear point where everything gets connected and validated together, rather than partial verification scattered across phases.