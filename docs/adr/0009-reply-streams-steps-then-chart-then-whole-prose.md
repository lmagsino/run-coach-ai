---
status: accepted
date: 2026-10-13
---
# In the MVP a reply streams its steps and chart, then arrives whole

The streaming contract for the MVP is the Phase 4 one plus one event: `step` events as
tools run, at most one `chart` event, then a single `answer` event with the whole prose.
Token-by-token `delta` events and a Stop button come later, after the MVP has been used.

The prose cannot usefully stream before the tools finish, because the coach has nothing
grounded to say until then, and a reply with several Garmin calls spends most of its 15 to
40 seconds in the tool loop that the steps already make visible. Streaming the final text
saves a few seconds of perceived wait and costs a second code path through the agent loop
(partial saves, abort handling) that the chart card and the shell should not wait for.
When deltas land, the `chart` event keeps its place: the card appears with the first text,
never before it.
