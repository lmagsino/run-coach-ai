---
status: accepted
date: 2026-10-13
---
# Garmin is the only source; Strava is removed from the code, not disabled

The original product reasoned across Strava and Garmin. Garmin already holds every
training session plus the sleep, HRV, stress and load signals Strava never had, and
Strava's hosted MCP needs a paid subscription while our own `strava-mcp` wrapped one
endpoint. So the product is Garmin only, and Phase 5 deletes the Strava code, tables,
env vars, mock scenarios and docs rather than leaving them behind a flag.

## Why delete rather than disable

A disabled source still costs every reader a question ("is this live?"), keeps a
migration and an OAuth flow alive that nothing exercises, and keeps multi-source
selection rules in the system prompt that make the coach hedge. Nothing has ever run
live, so there is no data to preserve.

## Kept on purpose

The per-source tool namespacing (`garmin__get_sleep_data`) and the `Source` slice in the
agent stay even with one source. They are the seam the roadmap's gateway and
bring-your-own-URL sources plug into (see ADR 0002).
