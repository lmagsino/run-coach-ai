# Phase 8: live checks

Results of the opt-in checks against real services. Each says how to rerun it.

## 8.3 The read surface against the real Garmin tool (14 Oct 2026)

```bash
RUNCOACH_LIVE_GARMIN_TESTS=1 go test ./internal/garmin/ -run LiveToolSurface -v   # from backend/, Docker running
```

Run on the laptop against upstream `garmin-mcp` at `43b0c5b` (the local Docker image) with the
owner's real token bundle. One Garmin read, no model call.

| Check | Result |
|---|---|
| Live tool list matches `deploy/garmin-mcp/tools.txt` | 155 of 155, none added or missing |
| Tools the read-only filter lets through | 77, every one a read (`get_*` or `count_activities`) |
| Spawn to ready (Docker on the laptop) | 1.1–1.2 s |
| Profile read with the bundle | 0.8–1.0 s, passed |

Found and fixed: the probe returned an empty name, so the account page said "Connected." instead
of "Connected as …". `get_user_profile` returns settings with no name; the probe now asks
`get_full_name` once the profile read has passed.

Still to do in 8.3: a first Garmin sign-in from Render (ADR 0011), once deployed.
