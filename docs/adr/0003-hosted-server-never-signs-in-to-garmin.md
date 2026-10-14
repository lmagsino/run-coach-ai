---
status: accepted
date: 2026-10-13
---
# The hosted server never signs in to Garmin

> Amended by [ADR 0011](0011-runners-may-sign-in-to-garmin-through-the-server.md): runners may now
> sign in through the server, with the token paste below kept as the fallback.

Garmin's SSO sits behind a Cloudflare edge that rate-limits server-side sign-ins per IP
(roughly 20 failures an hour earns a day-long block), the mobile login flow has returned
429 for everyone since March 2026, and a free host's shared egress IP is the first to be
burned. So the Garmin sign-in (email, password, MFA) happens once on Leo's laptop with
upstream's `garmin-mcp-auth`, and the hosted service only ever holds and uses the
resulting token bundle.

## How it reaches the server

`garmin-mcp-auth` writes a one-line base64 token bundle to `~/.garminconnect_base64`. Leo
pastes it into the app's Connection screen; the server validates it with one probe call,
encrypts it (AES-256-GCM, key from `RUNCOACH_TOKEN_KEY`) and stores it in
`garmin_connections`. Considered and dropped: a laptop-side `cmd/import-tokens` that
wrote straight to the database, because it would put the production database URL and the
encryption key on the laptop for a job the UI does with one textarea.

## Enforced by code, not by prompt

Upstream falls back to an email/password sign-in when tokens fail and those variables are
set. `LocalSpawn` therefore builds the child environment from scratch (`PATH`, `HOME`,
`GARMINTOKENS` and nothing inherited) and a unit test asserts `GARMIN_EMAIL` and
`GARMIN_PASSWORD` are absent. Upstream's own non-interactive guard is the second net.

## Consequences

- Token refresh (OAuth2 from OAuth1) happens in memory inside each spawn and is not a
  sign-in, so it is not throttled.
- Every six months or so the bundle expires. The chip, the reply and `/healthz` all say so
  and name the fix: run `garmin-mcp-auth` on the laptop, paste again.
- If Garmin changes its private endpoints, the coach says "Garmin unavailable" and never
  answers from memory.
