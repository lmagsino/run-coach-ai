# Reach matrix

Review question 2: does this change touch code that other paths run through? Shared and
registry rows are the ones a reviewer must check beyond the diff; anything unlisted is
specific.

| Path (glob) | Class | What runs through it | Notes |
|---|---|---|---|
| `backend/internal/agent/**` | shared | every reply: live, mock-off edge cases, the live-model tests, Phase 6 chart tool | The system prompt text is a registry: a wording change alters tool choice for all five questions |
| `backend/internal/agent/agent.go` (system prompt constants) | registry | every question the coach answers | Re-run the live pass section that the change targets |
| `backend/internal/mcpclient/**` and later `backend/internal/garmin/**` | shared | every spawn: replies, probe, `/healthz`, the live tool-diff test | The deny pattern is a registry: one pattern hides tools everywhere |
| `deploy/garmin-mcp/Dockerfile` (`GARMIN_MCP_REF`) | registry | the whole tool surface and every payload shape | A bump needs the live tool-diff test and the five questions re-run |
| `deploy/garmin-mcp/tools.txt` | registry | prompt category-map test, mock scenario pinning test | Regenerate from the pinned commit, never by hand |
| `backend/internal/api/sse.go` | shared | `/chat/stream` (later `/chats/:id/stream`), mock streaming, Playwright | Frame shape is an outward contract with `frontend/src/lib/api.js` |
| `backend/internal/api/chat.go` | shared | live and mock replies, Phase 6 chart event, Phase 7a persistence and cap | |
| `backend/internal/mockchat/**` | registry | the frontend in dev, Playwright, the step-label pinning test | Tool names here must exist in `tools.txt` |
| `backend/internal/config/**` | registry | every binary: server, migrate, hash-password | A renamed env var breaks Render until the dashboard is updated |
| `backend/internal/db/migrations/**` | registry | every table; Phase 7a restarts at 000001 | Nothing deployed before 7a.8 |
| `backend/internal/store/**` (7a) | shared | every data access, all runners | The scoped helper is the tenancy seam |
| `backend/internal/auth/**` (7a) | shared | every route except `/login` and `/healthz` | |
| `frontend/src/lib/api.js` | shared | every request the UI makes; the SSE parser | Pairs with `sse.go` |
| `frontend/src/lib/useChat.js` | shared | the thread, steps, chart card, Stop (7b) | |
| `frontend/src/lib/stepLabels.js` | registry | every step label; unknown tools fall back to a generic line | Keys must be real tool names |
| `frontend/src/style.css` (`@theme`) | registry | every component's colours and breakpoints | `wide:` is 561 px, not Tailwind's `sm:` |
| `frontend/src/components/AgentTurn.vue` | shared | every reply: prose, figure, chart card, streamed text (7b) | |
| `.github/workflows/ci.yml` | registry | every PR | Must stay credential-free |
| `Dockerfile` (root, 7a) | registry | the deployed image: UI, API, garmin-mcp | |
