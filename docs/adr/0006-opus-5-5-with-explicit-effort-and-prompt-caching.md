---
status: accepted
date: 2026-10-13
---
# The coach runs on Claude Opus 5.5 with explicit effort and prompt caching

`ANTHROPIC_MODEL` defaults to `claude-opus-5-5`; the stale `claude-sonnet-5` default goes.
Cache reads cost the same on Opus 5.5 and Sonnet 5.5 ($0.20 per million tokens), and the
shared block (system prompt plus about 90 tool definitions) is the bulk of every request,
so the Opus premium lands only on uncached input and on output. The Phase 5 live pass
measures the five questions at `medium` and `high` effort and once on Sonnet 5.5, and the
plan records the per-question cost before Phase 7a sets the daily cap.

## Fixed by the API, recorded so nobody re-tries them

Thinking cannot be disabled on Opus 5.5, so `effort` is the only dial and is set
explicitly (the default is `medium`). Forced `tool_choice` is rejected, so the chart tool
is offered under `auto` and prompted for, never forced. `cache_control` goes on the system
prompt and on the last tool definition so the whole block is one cache prefix.

Chat titles and empty-chat suggestions (Phase 7b) use `claude-haiku-5-5`.
