#!/bin/sh
# Copies the base64 token bundle out of the token volume to the clipboard, for
# the app's Connection page. Prints only its length.
set -e
b=$(docker run --rm -v run-coach-ai-garmin-tokens:/t alpine cat /t/garmin_tokens.b64)
printf '%s' "$b" | pbcopy
echo "bundle copied to the clipboard (${#b} characters)"
