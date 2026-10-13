---
status: accepted
date: 2026-10-13
---
# One free Render web service serves the API and the UI from one origin

Hosting must be free, so the model is the only paid line. The MVP runs as a single Render
free web service (Go server, the built Vue bundle embedded in the binary, and the Python
garmin-mcp installed in the same image) backed by Neon's free Postgres. Fly has no free
tier for new organisations; Render free has no persistent disk, which is why the token
bundle lives encrypted in Postgres and is written to a private temp directory only for
the life of one spawn.

## One origin on purpose

Serving the bundle from the Go binary means the phone talks to one host: the session is a
plain `HttpOnly; Secure; SameSite=Lax` cookie, there is no CORS in production, and Render
needs one service, not two. The Vite dev server keeps CORS for local work only.

## Accepted costs

The service sleeps after 15 idle minutes and takes about a minute to wake, so the login
page shows a "waking up" state rather than an error. Neon scales to zero and wakes in
under a second. The month's free hours cover one always-available service. Fallback if
Render changes its free tier: an Oracle Always Free ARM VM, which also has a disk and is
the roadmap gateway's host.
