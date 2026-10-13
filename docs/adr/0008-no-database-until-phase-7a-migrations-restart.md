---
status: accepted
date: 2026-10-13
---
# Phases 5 and 6 run without a database; migrations restart at 000001 in Phase 7a

After Strava is removed nothing is stored until chats are persisted in Phase 7a: there are
no OAuth tokens, no activity cache, and chat history still travels with the request as it
has since Phase 4. So `DATABASE_URL` becomes optional in Phase 5, the server starts
without it (`/healthz` reports `db: off`) and the live passes of Phases 5 and 6 need no
Postgres running.

The two existing migrations (chat tables, Strava OAuth tokens, activity cache) have only
ever been applied to Leo's laptop. Rather than carry a `000003` that drops two tables and
adds `runner_id` to the rest, Phase 7a deletes them and writes a fresh `000001` with
`runner_id` on every table from the start (ADR 0002). The down migration for the old
files is `go run ./cmd/migrate down` before the branch is merged, or simply dropping the
local database.
