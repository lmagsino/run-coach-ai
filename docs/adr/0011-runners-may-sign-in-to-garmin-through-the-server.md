---
status: accepted
date: 2026-10-14
---
# Runners may sign in to Garmin through the server, with the paste as fallback

Amends [ADR 0003](0003-hosted-server-never-signs-in-to-garmin.md). Making a token bundle in a
terminal and pasting it was too much to ask of a runner who only wants to connect their watch.
The account page now offers a Garmin email and password form, and the MFA code when Garmin asks
for one. The token paste stays, folded under "Other ways to connect".

## How it works

`POST /api/garmin/login` runs `runcoach-garmin-login` (`deploy/garmin-mcp/login.py`, installed in
both images). It signs in with upstream's own `garminconnect` and returns the same bundle
`garmin-mcp-auth` writes, so the bundle is probed, sealed and stored exactly like a pasted one.
When Garmin asks for a code, the helper keeps the sign-in open for up to five minutes and
`POST /api/garmin/login/code` finishes it.

## What protects the password

- It reaches the helper once, on stdin: never argv, the environment, the store, the log or a
  reply. The helper scrubs it from Garmin's error messages. The page clears the field on send.
- The helper gets none of the server's environment beyond what starting a program needs, so
  never the Anthropic key or the token key.
- Five sign-ins per runner per 15 minutes, counting successes, so RunCoach cannot trip Garmin's
  lock on a runner's account.

## Why ADR 0003's reasons still matter

Garmin's Cloudflare edge throttles server-side sign-ins per IP, and a free host's shared egress IP
is the first to be burned. A sign-in from the server can therefore fail for reasons that have
nothing to do with the runner. When it does (429, 403, Cloudflare), the reply says so and the
page opens the token paste, which works from the runner's own computer. Token refresh inside each
spawn is not a sign-in and is not affected. `GARMIN_LOGIN_COMMAND=off` turns the form off.

Whether Render's egress can sign in at all is unknown until it is tried with a real account.
