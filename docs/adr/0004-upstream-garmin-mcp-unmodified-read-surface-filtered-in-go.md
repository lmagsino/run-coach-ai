---
status: accepted
date: 2026-10-13
---
# Upstream garmin_mcp runs unmodified per reply; the read surface is filtered in Go

We use `Taxuspt/garmin_mcp` as published, pinned to a commit, installed into the same
image as the Go server and spawned over stdio for each reply with a private token
directory. We expose every tool that only reads (about 90 of 155) and deny the rest by
name pattern in Go, applied to the live `tools/list`.

## Why not upstream's allowlist

The Phase 3 allowlist named ten tools, three of which do not exist upstream
(`get_heart_rate_variability_summary`, `get_vo2_max_trend`, `get_daily_stress`), and
upstream ignores unknown names silently. A deny pattern applied in Go is testable offline
against the stub MCP server, denies a brand-new write tool by its shape, and fails loudly
in the live tool-diff test if a category-map name disappears.

## Why per-reply spawn, not a long-lived child

A spawn costs one or two seconds of Python start-up, which is small next to a 15 to 40
second reply, and it gives each reply a fresh process with its own token directory that is
deleted afterwards. Upstream also offers `GARMIN_MCP_TRANSPORT=streamable-http`; the
`garmin.Source` seam lets a long-lived child replace the spawn if the measured start-up
ever matters.

## Why not vendor or fork

Garmin's private endpoints change; upstream tracks them. The whole value of the dependency
is that someone else keeps it working. Our client is transport-level only.
