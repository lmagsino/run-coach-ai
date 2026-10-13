---
status: accepted
date: 2026-10-13
---
# One runner now, every table and interface shaped for many

The MVP serves one person, Leo. Serving other runners means handling their Garmin access,
which today is only possible through unofficial paths with real rate-limit and credential
risk (ADR 0003), so it is roadmap, not scope. But the cost of adding a runner later is set
by what we build now, so three seams are in from the first migration:

- every table carries `runner_id`; the owner is row one; a scoped query helper is the only
  data access and a test fails if a query bypasses it;
- `auth.Provider` with one implementation, `OwnerPassword` (hash in config, cookie
  session), and a route layer that never asks "is this Leo";
- `garmin.Source` with one implementation, `LocalSpawn`; `GatewayHTTP` and `BYOURL` are
  named in the interface's doc and nowhere else;
- `usage` and a daily question cap per runner, which doubles as the owner's cost guard.

## What is deliberately not built

No sign-up, no multi-runner tests, no privacy page, no gateway client. Each is a roadmap
item in `tasks/plan.md`, costed at about seven days together.
