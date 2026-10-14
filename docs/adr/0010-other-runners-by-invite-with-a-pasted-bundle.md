---
status: accepted
date: 2026-10-14
---
# Other runners sign up by invite and connect Garmin with a bundle they make themselves

RunCoach opens to other runners with email-and-password accounts. Each runner
connects Garmin the way the owner does: they run upstream's `garmin-mcp-auth`
once on their own computer (a `uvx` one-liner or a published Docker image) and
paste the one-line token bundle it writes. The server never sees a Garmin
password and never signs in to Garmin for anyone (ADR 0003).

## Considered

- **Server-side Garmin sign-in** (email, password and MFA typed into our UI).
  Fails from datacenter IPs, which is what every free host is, and makes us the
  custodian of Garmin passwords. Rejected.
- **A hosted sign-in gateway** (the MIT MissingMCP pattern: dedicated egress
  IPs, per-runner workers). The right answer for runners who cannot run a
  command; about a week of work and an always-on VM. Roadmap, behind
  `garmin.Source`.

## Rules that look like limits but are the design

- **Invite code.** Every reply is billed to the owner's API key, so sign-up is
  closed unless `RUNCOACH_INVITE_CODE` is set and typed correctly.
- **Caps.** Members start at `RUNCOACH_MEMBER_CAP` (10) questions a day, the
  owner at 30, and `RUNCOACH_GLOBAL_DAILY_CAP` (100) bounds the day across
  everyone before any model call.
- **No email verification or reset mail.** There is no mail service on free
  tiers. The invite code stands in for verification; the owner resets a
  password with `cmd/set-password`.
- **Hard delete.** Deleting an account removes the runner, sessions, chats,
  messages, usage and the Garmin bundle in one cascade. Nothing about a runner
  is kept that they cannot see.
- **The owner bootstrap is one-way.** `RUNCOACH_OWNER_EMAIL` and
  `RUNCOACH_OWNER_PASSWORD_HASH` fill the owner row only where it is empty, so
  a password changed on the account page is not reset by the next deploy.
