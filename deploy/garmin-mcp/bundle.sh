#!/bin/sh
# Copies the base64 token bundle out of the token volume into
# backend/.garmin_bundle.b64 (gitignored), where the server imports it on
# start, and to the clipboard for a hosted copy's Connection page. Prints only
# its length.
set -e
cd "$(dirname "$0")/../.."
b=$(docker run --rm -v run-coach-ai-garmin-tokens:/t alpine cat /t/garmin_tokens.b64)
umask 077
printf '%s\n' "$b" > backend/.garmin_bundle.b64
printf '%s' "$b" | pbcopy 2>/dev/null || true
echo "bundle written to backend/.garmin_bundle.b64 and copied to the clipboard (${#b} characters)"
echo "restart the backend and it connects on start"
