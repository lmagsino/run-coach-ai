#!/usr/bin/env python3
"""Signs a runner in to Garmin Connect for RunCoach and prints the token bundle.

JSON lines on stdin and stdout:
  in   {"email": ..., "password": ...}
  out  {"status": "needs_mfa"}                then in {"code": ...}
  out  {"status": "ok", "bundle": ...}       or {"status": "error", "kind": ..., "message": ...}

The password arrives on stdin, never in argv or the environment, and is never
written back out: messages from Garmin's library are scrubbed of it. The bundle
is what upstream's garmin-mcp-auth writes, base64 of garmin_tokens.json, so a
signed-in runner and a pasted bundle are stored the same way.
"""
import base64
import json
import os
import sys
import tempfile


def say(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def fail(kind, message):
    say({"status": "error", "kind": kind, "message": message})
    sys.exit(0)


def classify(exc):
    from garminconnect import GarminConnectAuthenticationError, GarminConnectTooManyRequestsError

    text = str(exc).lower()
    # 429 is a rate limit; 403 is how Cloudflare turns away a server it does
    # not trust. Either way, retrying from here will not help.
    if isinstance(exc, GarminConnectTooManyRequestsError) or "429" in text or "403" in text or "cloudflare" in text:
        return "blocked"
    if isinstance(exc, GarminConnectAuthenticationError) or "401" in text:
        return "bad_credentials"
    return "failed"


def main():
    try:
        creds = json.loads(sys.stdin.readline())
        email, password = creds["email"], creds["password"]
    except (ValueError, KeyError, TypeError):
        fail("bad_request", "expected one JSON line with email and password")

    def scrub(exc):
        return str(exc).replace(password, "***")[:300] if password else str(exc)[:300]

    from garminconnect import Garmin

    try:
        garmin = Garmin(email=email, password=password, return_on_mfa=True)
        result, state = garmin.login()
        if result == "needs_mfa":
            say({"status": "needs_mfa"})
            line = sys.stdin.readline()
            if not line:
                fail("cancelled", "no code was given")
            try:
                code = str(json.loads(line).get("code", "")).strip()
                garmin.resume_login(state, code)
            except Exception as exc:  # a wrong or expired code, or a refusal
                kind = classify(exc)
                fail(kind if kind == "blocked" else "bad_code", scrub(exc))

        with tempfile.TemporaryDirectory() as tokens:
            garmin.client.dump(tokens)
            with open(os.path.join(tokens, "garmin_tokens.json")) as f:
                data = f.read()
        say({"status": "ok", "bundle": base64.b64encode(data.encode()).decode()})
    except SystemExit:
        raise
    except Exception as exc:
        fail(classify(exc), scrub(exc))


if __name__ == "__main__":
    main()
