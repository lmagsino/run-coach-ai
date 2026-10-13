---
status: accepted
date: 2026-10-13
---
# Charts are a virtual tool the model calls and the backend validates

The coach draws a chart by calling `coach__show_chart` with already-reshaped points
(kind, title, units, one or two series of 5 to 120 points, caption). The backend answers
that tool itself: it validates the spec, emits one SSE `chart` event, stores the spec with
the reply, and refuses a second call in the same reply with a readable error. The frontend
renders it with hand-rolled SVG (`lib/chart.js`, `ChartCard.vue`).

## Considered

- A text marker like the existing `[figure: …]`: cannot carry 60 points without
  corrupting the prose, and parsing it is a guess.
- Backend-derived charts from the raw tool payloads: the backend would have to know every
  payload shape upstream emits, which is the thing we refused to own in ADR 0004.
- A chart library: adds a dependency and a theme fight for three chart kinds.

## Rules that look like limits but are the design

Line charts take at most two series, on one unit, never a dual axis: the product's two
chart colours (accent and muted) fail categorical distinctness (ΔE 10.9 under 15), so the
second series is told apart by a dashed stroke and a direct label, which does not scale to
three. Pace is passed in seconds per kilometre and rendered m:ss. A chart never renders
without its prose, and an invalid chart drops while the prose still renders.
