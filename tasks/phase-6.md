# Phase 6: charts

Branch `phase-6-charts` off `main`. Blocked by: Phase 5 merged. About five days. Can run
in parallel with Phase 7a.

Outcome: a reply may carry one chart card (line, bar or scatter) that the coach asked for
through the virtual `coach__show_chart` tool, validated by the backend, drawn by hand in
SVG, with a "View data" table. The five example questions that have a pattern show it.

Decisions this phase builds on: ADR 0005, 0009. The chart card's visual spec is in the
artifact's Design tab (§7 anatomy, §8 when a chart appears) and the reference renderer is
the artifact page's own script.

## Step 0 (Leo)

Nothing new. The Phase 5 container and `.env` are reused.

## Tasks

### Task 6.1: `show_chart` spec and validator

**Description:** A pure Go package that defines the tool's JSON schema and validates a
call into a typed `ChartSpec` or a readable error naming the field.

**Acceptance criteria:**
- [ ] `internal/chart` defines `Spec{Kind, Title, XLabel, YLabel, XUnit, YUnit, Data,
      Series[], Caption}` with `Kind ∈ {line, bar, scatter}`, `Data ∈ {activities, sleep,
      hrv, stress, body_battery, training_load, readiness, vo2max, steps, body}`,
      `Series[].Label` and `Series[].Points[]{X, Y}`.
- [ ] `Validate` rejects: fewer than 5 or more than 120 points in any series; more than
      one series for bar and scatter; more than two for line; two line series with
      different `YUnit` (the schema has one unit, so this is a doc rule enforced by the
      prompt; the validator rejects a `y_unit` of `mixed`); an unknown kind or data name;
      an empty title or caption; a non-finite number. Each error names the field.
- [ ] `ToolDefinition()` returns the Anthropic tool param (name `show_chart`, description
      with the rules above in two sentences, the JSON schema).
- [ ] Table tests cover every rejection and one valid spec per kind.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/chart`

**Blocked by:** none (Phase 5 merged)

**Files likely touched:** `backend/internal/chart/chart.go`, `backend/internal/chart/chart_test.go`.

**Estimated scope:** S

### Task 6.2: Virtual `coach` source in the agent loop

**Description:** The agent offers `coach__show_chart` next to the Garmin tools and answers
it itself.

**Acceptance criteria:**
- [ ] `agent.New` registers a built-in source `coach` whose only tool is `show_chart`;
      the tool is namespaced like the others; `tool_choice` stays `auto`.
- [ ] A valid call stores the spec on the `Result` (`Result.Chart *chart.Spec`), emits
      `Event{Kind: chart}` through `Observe`, and returns the tool result "Chart accepted;
      continue with the prose".
- [ ] A second call in the same reply returns the tool error "One chart per reply; the
      first one is kept" and the first spec stays.
- [ ] An invalid call returns the validator's error as the tool result so the model can
      correct it or drop it; no event is emitted; the reply continues.
- [ ] Scripted-model tests cover: no chart, one chart, two charts, invalid then valid,
      invalid then prose only.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/agent`

**Blocked by:** 6.1

**Files likely touched:** `backend/internal/agent/agent.go`, `backend/internal/agent/chart_test.go` (new).

**Estimated scope:** M

### Task 6.3: SSE `chart` event and two mock scenarios

**Description:** The stream carries the chart to the browser and mock mode can show one.

**Acceptance criteria:**
- [ ] `POST /chat/stream` emits `event: chart` with the spec as JSON after the steps and
      before `answer`; `POST /chat` returns `chart` in its JSON body (null when none).
- [ ] Two mock scenarios carry charts: "sleep vs pace" (scatter, 8 weeks) and "training
      blocks" (bar, 12 weeks with a partial current week flagged by a `partial: true`
      point attribute that the validator allows and the renderer draws faint); one
      existing scenario carries an invalid spec to prove the drop path; `chat_test.go`
      asserts the frame order and that the invalid one emits no `chart` frame.

**Verification:**
- [ ] Tests pass: `cd backend && go test -count=1 ./internal/api ./internal/mockchat`
- [ ] Manual check: `curl -N` against mock mode shows `event: chart` once.

**Blocked by:** 6.2

**Files likely touched:** `backend/internal/api/chat.go`, `backend/internal/api/chat_test.go`,
`backend/internal/mockchat/mockchat.go`, `backend/internal/chart/chart.go` (the `partial`
attribute).

**Estimated scope:** S

### Task 6.4: Prompt rules for charts and the live chart-rate test

**Description:** Teach the coach when a chart earns its place and when it does not.

**Acceptance criteria:**
- [ ] The system prompt gains a "Charts" section: one chart at most, only when the
      answer is a pattern over time or a relationship between two measures; never for a
      single number (use the figure marker); reshape the data yourself; pace in seconds
      per kilometre; two series only when they share a unit; the chart comes before the
      prose and the prose must still stand without it.
- [ ] The opt-in live model test (`RUNCOACH_LIVE_MODEL_TESTS=1`) runs the five questions
      over the fake data and records which charted, with what kind; the expectation
      written in the test is: questions 1 and 3 chart, 2 and 5 may, 4 does not.

**Verification:**
- [ ] Tests pass: `go test ./...` without the variable makes no network call.
- [ ] Manual check: the live test's output pasted into `docs/live/phase-6.md`.

**Blocked by:** 6.2

**Files likely touched:** `backend/internal/agent/agent.go`, `backend/internal/agent/crosssource_test.go`
(renamed to `live_model_test.go`), `docs/live/phase-6.md` (new).

**Estimated scope:** S

### Task 6.5: `lib/chart.js` layout engine

**Description:** Pure functions that turn a spec into SVG geometry, with no DOM.

**Acceptance criteria:**
- [ ] `layout(spec, width)` returns ticks (`niceTicks`), scales, axis labels, formatted
      tick text (`fmt` handles `min/km` as m:ss from seconds, `h`, `km`, plain numbers),
      series paths (line: polyline with the second series dashed; bar: rects with 2 px
      gaps and faint partial bars; scatter: 8 px markers plus a least-squares trend line),
      direct end labels for two-series lines, and the "View data" rows.
- [ ] Vitest covers: tick niceness at three spans, pace formatting, trend-line slope on a
      known set, two-series label placement not overlapping, 5-point and 120-point inputs.
- [ ] The functions match the artifact page's reference renderer in behaviour (same
      `niceTicks` and `fmt` rules).

**Verification:**
- [ ] Tests pass: `cd frontend && npm test`

**Blocked by:** none

**Files likely touched:** `frontend/src/lib/chart.js`, `frontend/src/lib/chart.test.js`.

**Estimated scope:** M

### Task 6.6: `ChartCard.vue` with data table

**Description:** The card the reply shows: title, chart, caption, Garmin data used, and a
"View data" toggle.

**Acceptance criteria:**
- [ ] `ChartCard.vue` renders the SVG from `layout()`, in the Field Notes tokens
      (accent stroke, muted second stroke, faint grid), responsive from 320 px up, with
      native `<title>` hover on marks, `role="img"` and an `aria-label` from the title and
      caption.
- [ ] "View data" toggles a table of the points (x, y per series) and a "Copy" button
      that copies tab-separated rows.
- [ ] `useChat` keeps the `chart` event on the turn; `AgentTurn.vue` shows the card
      above the prose and only once the `answer` arrives (ADR 0009); no answer, no card.
- [ ] `prefers-reduced-motion` respected (no draw-in animation).

**Verification:**
- [ ] Tests pass: `cd frontend && npm test` (component test for the toggle and for
      "no card before answer").
- [ ] Manual check: screenshots at 1280, 561 and 360 px in mock mode for both chart
      scenarios.

**Blocked by:** 6.3, 6.5

**Files likely touched:** `frontend/src/components/ChartCard.vue` (new),
`frontend/src/components/AgentTurn.vue`, `frontend/src/lib/useChat.js` and test,
`frontend/src/lib/api.js`.

**Estimated scope:** M

### Task 6.7: Playwright smoke on mock mode in CI

**Description:** Browser legs for the Verifier, before the first UI-visible milestone.

**Acceptance criteria:**
- [ ] `frontend/e2e/` has one Playwright spec that starts the backend in mock mode and
      the Vite preview, asks the two charting questions, asserts a card with an SVG and a
      working "View data" toggle, asks a non-charting question and asserts no card.
- [ ] CI runs it in a third job with Playwright's Chromium cached; the run is under
      three minutes.

**Verification:**
- [ ] Tests pass: `cd frontend && npx playwright test`
- [ ] Manual check: CI green with the new job.

**Blocked by:** 6.6

**Files likely touched:** `frontend/e2e/chart.spec.js` (new), `frontend/playwright.config.js`
(new), `frontend/package.json`, `.github/workflows/ci.yml`.

**Estimated scope:** S

### Task 6.8: Live pass with charts

**Description:** The five questions again, live, with charts on.

**Acceptance criteria:**
- [ ] `docs/live/phase-6.md` records per question: did it chart, kind, point count,
      whether the chart's numbers match the tool results, whether the prose stood alone,
      tokens and cost (the chart call adds output tokens; record the delta against
      Phase 5).
- [ ] Any validator rejection seen live is listed with the raw call, and either the
      prompt or the validator is adjusted and the question re-run.

**Verification:**
- [ ] Manual check: file complete; five screenshots in `docs/live/phase-6/`.

**Blocked by:** 6.4, 6.6

**Files likely touched:** `docs/live/phase-6.md`, `backend/internal/agent/agent.go`.

**Estimated scope:** M (mostly Leo's time)

## Checkpoint: Phase 6

- [ ] Backend, frontend and Playwright suites green; CI green.
- [ ] Questions 1 and 3 chart live; a single-number question does not.
- [ ] The invalid-chart mock scenario renders prose with no card.
- [ ] PR `phase-6-charts` → `main` merged by Leo.

## What the Verifier drives

Mock mode through Playwright (chart present, chart absent, invalid chart dropped) and the
live five with screenshots. No feature flag; the matrix row is "live vs mock".
