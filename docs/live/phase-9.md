# Phase 9 accounts: acceptance test report (14 Oct 2026)

What was run, against what, and what it found. The unit and API suites cover the same ground
without a network (`go test ./...`, `npm test`); this report is about the real stack.

## Setup

- A real server (the Phase 9 code) with **Postgres 15**, the **real Garmin spawn** (upstream
  `garmin-mcp` at `43b0c5b`), and the **real model** (`claude-sonnet-5-5` for answers,
  `claude-haiku-5-5` for triage).
- Driven by `frontend/e2e/accounts-live.mjs` in three steps around a **server restart**, plus
  direct database checks. Two members (Ana, Ben) and the owner.
- One real question was asked per full run (about five cents each); everything else made no
  model call.

## Results

| # | Check | Result |
|---|---|---|
| 1 | Owner signs in with the password from the environment | pass |
| 2 | Owner renames and changes the password on the account page | pass |
| 3 | The owner is not offered account deletion | pass |
| 4 | A member signs up with the invite code and lands on the account page | pass |
| 5 | A member pastes a real bundle; the server probes it through a real Garmin call and shows "connected" | pass |
| 6 | The member asks a real question and the reply comes from her own Garmin connection (12–13 s) | pass |
| 7 | The usage line counts the question (1 of 10) | pass |
| 8 | A member with no bundle is told to connect Garmin; nothing is read from anyone else's account | pass |
| 9 | A member gets 404 for another member's chat | pass |
| 10 | Sign-up and account pages fit a 390 px phone without sideways scrolling | pass |
| 11 | After a restart, the environment password no longer works and the changed one does (the owner bootstrap never overwrites) | pass |
| 12 | After a restart, the member's chat, reply, Garmin connection and usage are all still there | pass |
| 13 | A member at the daily cap sees it in the composer before asking; no chat is created and no model call is made | pass |
| 14 | Deleting an account leaves no chats, messages, Garmin row, usage or sessions for that runner; the audit trail is kept with the runner blanked | pass |
| 15 | Stored bundles are ciphertext: key-version byte first, no JSON or base64 inside | pass |
| 16 | With no invite code, sign-up is refused (403) and the sign-in page shows no "Create an account" link | pass |
| 17 | The global cap stops the owner too, with no model call | pass |
| 18 | `cmd/set-password` resets a member's password: the old one is refused, the new one works | pass |

## Bugs the runs found, all fixed before this report

- **Tenancy (serious).** A runner with no stored bundle still had the Garmin command spawned.
  With the then-documented option of mounting the owner's token volume, a member who had not
  connected Garmin would have been answered from the owner's account. Now no bundle means "not
  connected" and nothing is spawned; the volume option is gone from the docs.
- The config never read the owner email, invite code or caps from the environment (the unit
  tests built the config by hand).
- The sign-in rate limit counted successful sign-ins, so normal use could lock a runner out.
- Sign-out after deleting an account failed and left the page stuck.
- Opening the Garmin section scrolled the whole page and pushed the header off screen.
- At the cap, a refused question left an empty "New chat" behind with a log-like message.
- New chats showed as "New chat" in the sidebar for the whole reply.
- The coach told a runner "I can't chart this, because the chart tool needs at least 5 points",
  leaking an internal rule into the answer.

## Note on the test machine

On the work laptop, CrowdStrike blocked the test server as malware: it was a compiled binary
started from `/tmp` by a background script. That launch pattern is gone. Locally RunCoach now
runs only from the repository (`go run ./cmd/server`, `npm run dev`), never as a built program
(see CLAUDE.md). The one step of the final rerun that hit the block was not repeated; the run
before it had passed every check above.
